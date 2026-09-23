package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestExtensionStore_ImportOnlyNoBuiltin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.json")
	t.Setenv("AURORA_EXTENSIONS_PATH", path)

	store := NewExtensionStore()
	if len(store.List()) != 0 {
		t.Fatalf("expected no built-in extensions, got %d", len(store.List()))
	}
	if _, ok := store.Get("opencode"); ok {
		t.Fatal("opencode must not be built-in")
	}

	custom := Extension{
		ID:          "my-proto",
		Name:        "My Protocol",
		BaseURL:     "https://example.test/v1",
		DefaultAuth: "Bearer secret",
		Headers: []ExtensionHeader{
			{Name: "x-my-session", Mode: "map_or_generate", Prefix: "s_", Length: 16, Charset: "hex"},
		},
		Provides: &ExtensionProvides{ProviderTypes: []string{"demo-type"}, Features: []string{"oauth"}},
	}
	if err := custom.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	var activated []string
	store.SetProviderTypeActivator(func(p *ExtensionProvides) []string {
		activated = append(activated, p.ProviderTypes...)
		return p.ProviderTypes
	})
	gotTypes := store.Upsert(custom)
	if len(gotTypes) != 1 || gotTypes[0] != "demo-type" {
		t.Fatalf("activated = %v, want [demo-type]", gotTypes)
	}
	if len(activated) != 1 {
		t.Fatalf("activator calls = %v", activated)
	}

	reloaded := NewExtensionStore()
	e, ok := reloaded.Get("my-proto")
	if !ok {
		t.Fatal("imported extension did not persist")
	}
	if e.Name != "My Protocol" || e.BaseURL != "https://example.test/v1" {
		t.Fatalf("reloaded mismatch: %+v", e)
	}
	if e.Provides == nil || len(e.Provides.ProviderTypes) != 1 {
		t.Fatalf("provides not persisted: %+v", e.Provides)
	}

	if !store.Delete("my-proto") {
		t.Fatal("imported extension should be deletable")
	}
	if store.Delete("missing") {
		t.Fatal("missing id should not delete")
	}
}

func TestExtension_ValidateRejectsReservedID(t *testing.T) {
	p := Extension{ID: "opencode"}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if p.ID != "opencode" {
		t.Fatalf("store seed id should be preserved, got %q", p.ID)
	}

	bad := Extension{ID: "with space"}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected validation error for id with space")
	}
}

func TestExtension_ExtendedSchemaRoundTrip(t *testing.T) {
	raw := `{
	  "schema": 1,
	  "id": "proto-x",
	  "name": "Proto X",
	  "author": "community",
	  "tags": ["tls", "custom"],
	  "homepage": "https://example.test",
	  "provides": {"provider_types": ["opencode"], "features": ["oauth"]},
	  "inject_tool_types": ["opencode"],
	  "tool_schemas": [{"name":"custom","source":"custom"},{"name":"inline","inline":[{"type":"function"}]}],
	  "settings": {"transport": "bun-tls"},
	  "oauth": {"server":"https://auth.example.test","client_id":"cli","verification_base":"https://auth.example.test"},
	  "files": {"tools/opencode.json":"[]","scripts/helper.js":"// helper"},
	  "headers": [{"name":"x-session","mode":"generate","prefix":"s_","length":24,"charset":"hex"}],
	  "ui": {"accent":"#123456","fields":[{"key":"region","label":"Region","type":"select","options":["eu","us"]}]}
	}`
	var p Extension
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Tools) != 2 || p.Tools[0].Source != "custom" {
		t.Fatalf("tool_schemas not parsed: %+v", p.Tools)
	}
	if p.Settings["transport"] != "bun-tls" {
		t.Fatalf("settings not parsed: %+v", p.Settings)
	}
	if p.Provides == nil || len(p.Provides.ProviderTypes) != 1 || p.Provides.Features[0] != "oauth" {
		t.Fatalf("provides not parsed: %+v", p.Provides)
	}
	if p.OAuth == nil || p.OAuth.Server != "https://auth.example.test" || p.OAuth.VerificationBase != "https://auth.example.test" {
		t.Fatalf("oauth not parsed: %+v", p.OAuth)
	}
	if len(p.Files) != 2 || p.Files["tools/opencode.json"] != "[]" {
		t.Fatalf("files not parsed: %+v", p.Files)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(p.InjectTypes) != 1 || p.InjectTypes[0] != "opencode" {
		t.Fatalf("inject_tool_types not parsed: %+v", p.InjectTypes)
	}
	if len(p.UI.Fields) != 1 || p.UI.Fields[0].Key != "region" {
		t.Fatalf("ui.fields not parsed: %+v", p.UI.Fields)
	}
	out, err := json.Marshal(p)
	if err != nil || !json.Valid(out) {
		t.Fatalf("round-trip invalid: %v", err)
	}
}

func TestExtension_ValidateRejectsUnsafeFilesPath(t *testing.T) {
	p := Extension{ID: "x", Name: "x", Files: map[string]string{"../escape.json": "[]"}}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for .. path")
	}
	p.Files = map[string]string{"/abs.json": "[]"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestExtension_MaterializeFiles(t *testing.T) {
	dir := t.TempDir()
	ext := Extension{
		ID: "x",
		Files: map[string]string{
			"tools/opencode-schema.json": `[]`,
			"scripts/README.md":          "docs",
		},
	}
	toolsPath, err := ext.MaterializeFiles(dir)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if toolsPath == "" {
		t.Fatal("expected tools path for schema json")
	}
	data, err := os.ReadFile(toolsPath)
	if err != nil || string(data) != "[]" {
		t.Fatalf("materialized content: %v %q", err, data)
	}
	if !strings.HasSuffix(toolsPath, filepath.Join("tools", "opencode-schema.json")) {
		t.Fatalf("tools path = %q", toolsPath)
	}
}

func TestExtensionStore_MigratesLegacyPresetsFile(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "sidecar-presets.json")
	data := `[{"id":"legacy-ext","name":"Legacy","base_url":"https://example.test/v1"}]`
	if err := os.WriteFile(legacy, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURORA_SIDECAR_PRESETS_PATH", legacy)
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	if _, ok := store.Get("legacy-ext"); !ok {
		t.Fatal("legacy preset should migrate into the extension store")
	}
}

func TestHandler_ListExtensionUI_OnlyApplied(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	withUI := Extension{
		ID:   "ui-ext",
		Name: "UI Ext",
		UI: ExtensionUI{
			Accent: "#112233",
			Nav: []ExtensionNavEntry{
				{ID: "docs", Label: "Docs", To: "docs", Icon: "book"},
			},
			Pages: []ExtensionUIPage{
				{ID: "docs", Path: "docs", Title: "Docs"},
			},
			Banners: []ExtensionBanner{
				{ID: "b1", Message: "Hello", Level: "info"},
			},
		},
	}
	noUI := Extension{ID: "plain", Name: "Plain"}
	other := Extension{
		ID:   "not-applied",
		Name: "Not Applied",
		UI:   ExtensionUI{Accent: "#445566"},
	}
	store.Upsert(withUI)
	store.Upsert(noUI)
	store.Upsert(other)

	// Simulate apply for ui-ext only.
	applied, ok := store.Get("ui-ext")
	if !ok {
		t.Fatal("ui-ext missing")
	}
	applied.Applied = true
	store.Upsert(applied)

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/ui", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensionUI(c); err != nil {
		t.Fatalf("ListExtensionUI: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Contributions []ExtensionUIContribution `json:"contributions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Contributions) != 1 {
		t.Fatalf("contributions = %d, want 1 (%s)", len(body.Contributions), rec.Body.String())
	}
	got := body.Contributions[0]
	if got.ID != "ui-ext" || got.UI.Accent != "#112233" || len(got.UI.Nav) != 1 {
		t.Fatalf("unexpected contribution: %+v", got)
	}
}

func TestApplyExtension_ConfiguresOAuthOnMatchingProviders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_PROVIDER_OVERRIDES_PATH", filepath.Join(dir, "provider-overrides.json"))
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "sidecar-overrides.json"))

	overrides := NewProviderOverrideStore()
	zend := true
	overrides.upsert(ProviderOverride{
		Name:    "vllm-zen-main",
		Type:    "vllm",
		BaseURL: "https://opencode.ai/zen/v1",
		Enabled: &zend,
	})
	overrides.upsert(ProviderOverride{
		Name:    "openrouter-main",
		Type:    "openrouter",
		BaseURL: "https://openrouter.ai/api/v1",
		Enabled: &zend,
	})

	store := NewExtensionStore()
	ext := Extension{
		ID:     "opencode",
		Name:   "upstream",
		BaseURL: "https://opencode.ai/zen/v1",
		OAuth: &ExtensionOAuth{
			Server:           "https://example.com/console",
			ClientID:         "opencode-cli",
			VerificationBase: "https://example.com",
		},
		Provides: &ExtensionProvides{
			ProviderTypes: []string{"opencode"},
			Features:      []string{"oauth"},
		},
	}
	store.Upsert(ext)

	h := NewHandler(nil, nil,
		WithExtensionStore(store),
		WithSidecarStore(NewSidecarOverrideStore()),
		WithProviderOverrides(overrides),
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/opencode/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "opencode"}})
	if err := h.ApplyExtension(c); err != nil {
		t.Fatalf("ApplyExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		OAuthProviders []string `json:"oauth_providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.OAuthProviders) != 1 || body.OAuthProviders[0] != "vllm-zen-main" {
		t.Fatalf("oauth_providers = %v, want [vllm-zen-main]", body.OAuthProviders)
	}

	zen, ok := overrides.get("vllm-zen-main")
	if !ok {
		t.Fatal("zen override missing")
	}
	if zen.AuthMethod != "oauth" || zen.OAuthServer != "https://example.com/console" || zen.OAuthClientID != "opencode-cli" {
		t.Fatalf("zen oauth not applied: %+v", zen)
	}
	other, _ := overrides.get("openrouter-main")
	if other.AuthMethod == "oauth" {
		t.Fatalf("non-matching provider must not get oauth: %+v", other)
	}

	// Sidecar defaults also carry OAuth endpoints.
	side := h.sidecarStore.get()
	if side.OAuthServer != "https://example.com/console" || side.OAuthClientID != "opencode-cli" {
		t.Fatalf("sidecar oauth defaults not set: %+v", side)
	}
}
