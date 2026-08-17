package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/internal/providers"
)

func newPoolTestHandler(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	poolPath := filepath.Join(dir, "pool-overrides.json")
	providerPath := filepath.Join(dir, "provider-overrides.json")
	t.Setenv("AURORA_POOL_OVERRIDES_PATH", poolPath)
	t.Setenv("AURORA_PROVIDER_OVERRIDES_PATH", providerPath)

	h := NewHandler(nil, nil,
		WithPoolWeights(NewPoolOverrideStore()),
		WithProviderOverrides(NewProviderOverrideStore()),
		WithConfiguredProviders([]providers.SanitizedProviderConfig{
			{Name: "oa-east", Type: "openai"},
			{Name: "oa-west", Type: "openai"},
			{Name: "gr-a", Type: "groq"},
		}),
		WithRuntimeRefresher(&mockRuntimeRefresher{}),
	)
	return h
}

func doPoolRequest(t *testing.T, h *Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// When the handler reads c.Param("name"), populate it from the URL since we
	// invoke the handler directly without going through the router's param matching.
	if method == http.MethodPut || method == http.MethodDelete {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		name := parts[len(parts)-1]
		c.SetPathValues(echo.PathValues{{Name: "name", Value: name}})
	}
	var err error
	switch method {
	case http.MethodPost:
		err = h.CreatePool(c)
	case http.MethodPut:
		err = h.UpdatePool(c)
	case http.MethodDelete:
		err = h.DeletePool(c)
	case http.MethodGet:
		err = h.ListPools(c)
	}
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return rec
}

func TestPoolCRUD_CreateUpdateDelete(t *testing.T) {
	h := newPoolTestHandler(t)

	// Missing member → 400
	rec := doPoolRequest(t, h, http.MethodPost, "/admin/api/v1/pools", map[string]any{
		"name": "p1", "members": []string{}, "strategy": "round_robin",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty members status = %d, want 400", rec.Code)
	}

	// Unknown member → 400
	rec = doPoolRequest(t, h, http.MethodPost, "/admin/api/v1/pools", map[string]any{
		"name": "p1", "members": []string{"does-not-exist"}, "strategy": "round_robin",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown member status = %d, want 400", rec.Code)
	}

	// Mixed provider types → 400
	rec = doPoolRequest(t, h, http.MethodPost, "/admin/api/v1/pools", map[string]any{
		"name": "p1", "members": []string{"oa-east", "gr-a"}, "strategy": "round_robin",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mixed types status = %d, want 400", rec.Code)
	}

	// Name collides with a provider instance → 400
	rec = doPoolRequest(t, h, http.MethodPost, "/admin/api/v1/pools", map[string]any{
		"name": "oa-east", "members": []string{"oa-west"}, "strategy": "round_robin",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("name collision status = %d, want 400", rec.Code)
	}

	// Valid create → 201 and persisted file exists
	rec = doPoolRequest(t, h, http.MethodPost, "/admin/api/v1/pools", map[string]any{
		"name": "multi", "members": []string{"oa-east", "oa-west"}, "strategy": "weighted",
		"weights": map[string]int{"oa-east": 3, "oa-west": 1}, "health_aware": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(os.Getenv("AURORA_POOL_OVERRIDES_PATH"))
	if err != nil {
		t.Fatalf("pool overrides not persisted: %v", err)
	}
	var file poolOverrideFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("decode persisted pools: %v", err)
	}
	if _, ok := file.Pools["multi"]; !ok {
		t.Fatalf("pool 'multi' not found in persisted overrides: %s", data)
	}

	// Update → 200
	rec = doPoolRequest(t, h, http.MethodPut, "/admin/api/v1/pools/multi", map[string]any{
		"members": []string{"oa-east"}, "strategy": "round_robin",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	// Delete → 200 and removed from persisted file
	rec = doPoolRequest(t, h, http.MethodDelete, "/admin/api/v1/pools/multi", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", rec.Code)
	}
	data, _ = os.ReadFile(os.Getenv("AURORA_POOL_OVERRIDES_PATH"))
	var file2 poolOverrideFile
	_ = json.Unmarshal(data, &file2)
	if _, ok := file2.Pools["multi"]; ok {
		t.Fatalf("pool 'multi' still present after delete: %s", data)
	}
	if len(file2.Deleted) == 0 || file2.Deleted[0] != "multi" {
		t.Fatalf("pool 'multi' not recorded as deleted: %s", data)
	}
}

func TestPoolOptions_ExcludesNothing(t *testing.T) {
	h := newPoolTestHandler(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/pools/options", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.PoolOptions(c); err != nil {
		t.Fatalf("PoolOptions failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Providers []ProviderPoolOption `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Providers) != 3 {
		t.Fatalf("providers = %d, want 3", len(body.Providers))
	}
}
