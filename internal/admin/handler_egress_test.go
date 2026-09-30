package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/internal/egress"
	"aurora/internal/providers"
)

func newEgressTestHandler(t *testing.T, reg *egress.Registry, synced *int) *Handler {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AURORA_PROVIDER_OVERRIDES_PATH", filepath.Join(dir, "provider-overrides.json"))

	store := NewProviderOverrideStore()
	opts := []Option{
		WithProviderOverrides(store),
		WithConfiguredProviders([]providers.SanitizedProviderConfig{
			{Name: "oa-east", Type: "openai", BindIPs: []string{"203.0.113.7", "203.0.113.8"}},
		}),
		WithEgress(reg),
	}
	if synced != nil {
		// Stands in for app.syncEgress: re-read the persisted list, exactly
		// what the gateway does after the handler writes the override.
		opts = append(opts, WithEgressSyncer(func() {
			*synced++
			if o, ok := store.get("oa-east"); ok {
				reg.SetDisabled("oa-east", o.EgressDisabled)
				return
			}
			reg.SetDisabled("oa-east", nil)
		}))
	}
	return NewHandler(nil, nil, opts...)
}

// doEgressRequest invokes an egress handler directly, populating the path
// params the router would otherwise supply.
func doEgressRequest(t *testing.T, h *Handler, call func(*echo.Context) error, path, provider string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "provider", Value: provider}})
	if err := call(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func egressRegistryWithOneIP() *egress.Registry {
	reg := egress.NewRegistry()
	reg.SetCore("oa-east", egress.StrategyRoundRobin, []egress.Candidate{
		{Name: "ip:203.0.113.7", Source: egress.SourceConfig},
		{Name: "ip:203.0.113.8", Source: egress.SourceConfig},
	})
	return reg
}

func TestListEgressStatusReportsEveryExit(t *testing.T) {
	h := newEgressTestHandler(t, egressRegistryWithOneIP(), nil)

	rec := doEgressRequest(t, h, h.ListEgressStatus, "/egress", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out egressStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Providers) != 1 || out.Providers[0].Provider != "oa-east" {
		t.Fatalf("providers = %+v, want the oa-east entry", out.Providers)
	}
	if len(out.Providers[0].Exits) != 2 {
		t.Fatalf("exits = %+v, want both configured addresses", out.Providers[0].Exits)
	}
}

func TestDisableEgressExitPersistsAndTakesEffect(t *testing.T) {
	reg := egressRegistryWithOneIP()
	synced := 0
	h := newEgressTestHandler(t, reg, &synced)

	rec := doEgressRequest(t, h, h.DisableEgressExit, "/egress/oa-east/disable", "oa-east",
		egressExitRequest{Exit: "ip:203.0.113.7"})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if synced != 1 {
		t.Fatalf("syncer called %d times, want 1", synced)
	}
	if got := len(reg.Candidates("oa-east")); got != 1 {
		t.Fatalf("candidates after disable = %d, want 1", got)
	}
	st, ok := reg.Status("oa-east")
	if !ok || len(st.Exits) != 2 {
		t.Fatalf("status = %+v ok=%v, want both exits reported", st, ok)
	}
	for _, exit := range st.Exits {
		if exit.Name == "ip:203.0.113.7" && (!exit.Disabled || exit.Available) {
			t.Fatalf("disabled exit status = %+v", exit)
		}
	}

	// The choice must survive a restart: it lives on the provider override.
	stored, ok := h.providerOverrides.get("oa-east")
	if !ok || len(stored.EgressDisabled) != 1 || stored.EgressDisabled[0] != "ip:203.0.113.7" {
		t.Fatalf("stored override = %+v, want the disabled exit recorded", stored)
	}
	// Seeding from the static provider must not drop its other egress fields.
	if len(stored.BindIPs) != 2 || stored.BindIPs[0] != "203.0.113.7" {
		t.Fatalf("stored bind_ips = %v, want the static list preserved", stored.BindIPs)
	}

	rec = doEgressRequest(t, h, h.EnableEgressExit, "/egress/oa-east/enable", "oa-east",
		egressExitRequest{Exit: "ip:203.0.113.7"})
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := len(reg.Candidates("oa-east")); got != 2 {
		t.Fatalf("candidates after enable = %d, want 2", got)
	}
	if stored, ok := h.providerOverrides.get("oa-east"); ok && len(stored.EgressDisabled) != 0 {
		t.Fatalf("disabled list after enable = %v, want it emptied", stored.EgressDisabled)
	}
}

func TestEgressToggleRejectsUnknownProviderAndExit(t *testing.T) {
	h := newEgressTestHandler(t, egressRegistryWithOneIP(), nil)

	rec := doEgressRequest(t, h, h.DisableEgressExit, "/egress/nope/disable", "nope",
		egressExitRequest{Exit: "ip:203.0.113.7"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown provider status = %d, want 404", rec.Code)
	}

	rec = doEgressRequest(t, h, h.DisableEgressExit, "/egress/oa-east/disable", "oa-east",
		egressExitRequest{Exit: ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty exit status = %d, want 400", rec.Code)
	}
}

// The routes are mounted through the same router the API uses, so a path
// conflict or a param clash would show up here rather than in production.
func TestEgressRoutesAreRegisteredWithoutConflict(t *testing.T) {
	reg := egressRegistryWithOneIP()
	synced := 0
	h := newEgressTestHandler(t, reg, &synced)

	e := echo.New()
	h.RegisterRoutes(e.Group("/admin/api/v1"))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/api/v1/egress", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /egress = %d, body = %s", rec.Code, rec.Body.String())
	}

	body, err := json.Marshal(egressExitRequest{Exit: "ip:203.0.113.7"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/egress/oa-east/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /egress/oa-east/disable = %d, body = %s", rec.Code, rec.Body.String())
	}
	if synced != 1 {
		t.Fatalf("syncer called %d times, want 1", synced)
	}
}
