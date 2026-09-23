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
		ID:      "opencode",
		Name:    "upstream",
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

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

func TestExtension_EffectiveTags(t *testing.T) {
	themeExt := Extension{
		ID:   "theme-ext",
		Type: "theme",
		Tags: []string{"custom", "theme"},
	}
	tags := themeExt.EffectiveTags()
	for _, want := range []string{"custom", "theme"} {
		if !containsTag(tags, want) {
			t.Fatalf("themeExt tags = %v, missing %q", tags, want)
		}
	}
	if count := countFold(tags, "theme"); count != 1 {
		t.Fatalf("theme tag should appear once, got %d in %v", count, tags)
	}
	if len(themeExt.Tags) != 2 {
		t.Fatalf("EffectiveTags must not mutate stored tags: %v", themeExt.Tags)
	}

	inject := true
	full := Extension{
		ID:          "full-ext",
		BaseURL:     "https://example.test/v1",
		InjectTools: &inject,
		Headers:     []ExtensionHeader{{Name: "x-session", Mode: "generate"}},
		Tags:        []string{"tls"},
		UI: ExtensionUI{
			Accent: "#112233",
			Nav:    []ExtensionNavEntry{{ID: "n", Label: "N", To: "n"}},
		},
		Provides: &ExtensionProvides{
			ProviderTypes: []string{"demo"},
			Features:      []string{"oauth"},
		},
		OAuth: &ExtensionOAuth{Server: "https://auth.example.test"},
	}
	tags = full.EffectiveTags()
	for _, want := range []string{"tls", "sessionhub", "sidecar", "ui", "oauth", "providers"} {
		if !containsTag(tags, want) {
			t.Fatalf("full tags = %v, missing %q", tags, want)
		}
	}
	if len(full.Tags) != 1 || full.Tags[0] != "tls" {
		t.Fatalf("stored tags mutated: %v", full.Tags)
	}

	colorField := Extension{
		ID: "color-ext",
		UI: ExtensionUI{Fields: []ExtensionField{{Key: "brand", Type: "color"}}},
	}
	tags = colorField.EffectiveTags()
	if !containsTag(tags, "theme") {
		t.Fatalf("color ui.fields should imply theme tag, got %v", tags)
	}
	if containsTag(tags, "ui") {
		t.Fatalf("color field alone must not imply ui tag: %v", tags)
	}
}

func countFold(tags []string, want string) int {
	n := 0
	for _, t := range tags {
		if strings.EqualFold(t, want) {
			n++
		}
	}
	return n
}

func TestHandler_ListAndGetExtensions_EffectiveTags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{
		ID:      "theme-ext",
		Name:    "Theme Ext",
		Type:    "theme",
		Headers: []ExtensionHeader{{Name: "x-session", Mode: "generate"}},
	})

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensions(c); err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	var listBody struct {
		Extensions []Extension `json:"extensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(listBody.Extensions) != 1 {
		t.Fatalf("extensions = %d, want 1", len(listBody.Extensions))
	}
	listed := listBody.Extensions[0]
	if !containsTag(listed.Tags, "theme") || !containsTag(listed.Tags, "sessionhub") {
		t.Fatalf("list response tags = %v, want theme+sessionhub", listed.Tags)
	}

	req = httptest.NewRequest(http.MethodGet, "/sidecar/extensions/theme-ext", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "theme-ext"}})
	if err := h.GetExtension(c); err != nil {
		t.Fatalf("GetExtension: %v", err)
	}
	var got Extension
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !containsTag(got.Tags, "theme") || !containsTag(got.Tags, "sessionhub") {
		t.Fatalf("get response tags = %v, want theme+sessionhub", got.Tags)
	}

	stored, ok := store.Get("theme-ext")
	if !ok {
		t.Fatal("extension missing from store")
	}
	if len(stored.Tags) != 0 {
		t.Fatalf("stored tags must stay empty, got %v", stored.Tags)
	}
}

func TestHandler_UnapplyExtension(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{ID: "toggle-ext", Name: "Toggle", Applied: true})

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/toggle-ext/unapply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "toggle-ext"}})
	if err := h.UnapplyExtension(c); err != nil {
		t.Fatalf("UnapplyExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status  string `json:"status"`
		Applied bool   `json:"applied"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" || body.Applied {
		t.Fatalf("body = %+v, want status ok applied false", body)
	}
	got, ok := store.Get("toggle-ext")
	if !ok {
		t.Fatal("extension missing")
	}
	if got.Applied {
		t.Fatal("Applied should be false after unapply")
	}

	// Unknown id → 404.
	req = httptest.NewRequest(http.MethodPost, "/sidecar/extensions/missing/unapply", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "missing"}})
	if err := h.UnapplyExtension(c); err != nil {
		t.Fatalf("UnapplyExtension missing: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing id status = %d, want 404", rec.Code)
	}
}

func TestHandler_UpdateExtensionConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{
		ID:   "cfg-ext",
		Name: "Cfg",
		UI: ExtensionUI{
			Fields: []ExtensionField{
				{Key: "accent", Label: "Accent", Type: "color"},
				{Key: "region", Label: "Region", Type: "select", Options: []string{"eu", "us"}},
			},
		},
		Config: map[string]string{"region": "eu"},
	})

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	doPut := func(payload string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/sidecar/extensions/cfg-ext/config",
			strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "cfg-ext"}})
		if err := h.UpdateExtensionConfig(c); err != nil {
			t.Fatalf("UpdateExtensionConfig: %v", err)
		}
		return rec
	}

	rec := doPut(`{"config":{"accent":"#ff0000","--radius":"8px"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get("cfg-ext")
	if got.Config["accent"] != "#ff0000" || got.Config["--radius"] != "8px" {
		t.Fatalf("config not merged: %+v", got.Config)
	}
	if got.Config["region"] != "eu" {
		t.Fatalf("existing config keys must be preserved: %+v", got.Config)
	}

	// Validated: unknown key that is neither a ui.fields key nor a CSS var.
	rec = doPut(`{"config":{"bogus":"1"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bogus key status = %d, want 400", rec.Code)
	}
	got, _ = store.Get("cfg-ext")
	if _, exists := got.Config["bogus"]; exists {
		t.Fatalf("bogus key must not persist: %+v", got.Config)
	}
}

func TestHandler_ListExtensionUI_MergesConfigTheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	ext := Extension{
		ID:      "ui-cfg",
		Name:    "UI Cfg",
		Applied: true,
		UI: ExtensionUI{
			Theme: map[string]string{"--accent": "#112233"},
			Fields: []ExtensionField{
				{Key: "accent", Label: "Accent", Type: "color"},
				{Key: "region", Label: "Region", Type: "select"},
			},
		},
		Config: map[string]string{
			"accent":   "#aabbcc",
			"--radius": "12px",
			"region":   "eu",
		},
	}
	store.Upsert(ext)

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/ui", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensionUI(c); err != nil {
		t.Fatalf("ListExtensionUI: %v", err)
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
	theme := body.Contributions[0].UI.Theme
	if theme["--accent"] != "#aabbcc" {
		t.Fatalf("color accent field must override --accent, theme=%v", theme)
	}
	if theme["--radius"] != "12px" {
		t.Fatalf("-- config key must land in theme, theme=%v", theme)
	}

	stored, _ := store.Get("ui-cfg")
	if stored.UI.Theme["--accent"] != "#112233" {
		t.Fatalf("stored theme must not be mutated: %v", stored.UI.Theme)
	}
}

func TestExtension_ValidateAllowsThemeType(t *testing.T) {
	theme := Extension{ID: "aurora-theme", Type: "theme"}
	if err := theme.Validate(); err != nil {
		t.Fatalf("theme validate: %v", err)
	}
	if theme.Type != "theme" {
		t.Fatalf("type = %q, want theme preserved", theme.Type)
	}

	empty := Extension{ID: "plain"}
	if err := empty.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if empty.Type != "sidecar" {
		t.Fatalf("empty type should default to sidecar, got %q", empty.Type)
	}
}

// fakeHeaderEnsurer records EnsureHeaders calls for full-apply tests.
type fakeHeaderEnsurer struct {
	target  string
	headers []ExtensionHeader
	err     error
}

func (f *fakeHeaderEnsurer) EnsureHeaders(target string, headers []ExtensionHeader) (map[string]any, error) {
	f.target = target
	f.headers = headers
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{
		"provider": target,
		"added":    []string{headers[0].Name},
		"already":  []string{},
	}, nil
}

func TestHandler_FullApplyExtension(t *testing.T) {
	newSetup := func(t *testing.T) (*Handler, *ExtensionStore) {
		t.Helper()
		dir := t.TempDir()
		t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
		t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "sidecar-overrides.json"))
		store := NewExtensionStore()
		store.Upsert(Extension{
			ID:      "hdr-ext",
			Name:    "Hdr",
			BaseURL: "https://example.test/v1",
			Headers: []ExtensionHeader{
				{Name: "x-session", Mode: "generate", Prefix: "s_", Length: 16, Charset: "hex"},
			},
		})
		h := NewHandler(nil, nil,
			WithExtensionStore(store),
			WithSidecarStore(NewSidecarOverrideStore()),
		)
		return h, store
	}

	doFullApply := func(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
		t.Helper()
		e := echo.New()
		var r *strings.Reader
		if body != "" {
			r = strings.NewReader(body)
		} else {
			r = strings.NewReader("")
		}
		req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/hdr-ext/full-apply", r)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "hdr-ext"}})
		if err := h.FullApplyExtension(c); err != nil {
			t.Fatalf("FullApplyExtension: %v", err)
		}
		return rec
	}

	t.Run("installs headers when ensurer wired", func(t *testing.T) {
		h, store := newSetup(t)
		ensurer := &fakeHeaderEnsurer{}
		h.SetSessionHeaderEnsurer(ensurer)

		rec := doFullApply(t, h, `{"session_hub_target":"main-pool"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Status     string            `json:"status"`
			Headers    []ExtensionHeader `json:"headers"`
			SessionHub map[string]any    `json:"session_hub"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Status != "ok" || len(body.Headers) != 1 {
			t.Fatalf("apply shape = %+v", body)
		}
		installed, _ := body.SessionHub["installed"].(bool)
		if !installed {
			t.Fatalf("session_hub = %+v, want installed", body.SessionHub)
		}
		if body.SessionHub["target"] != "main-pool" {
			t.Fatalf("session_hub target = %v", body.SessionHub["target"])
		}
		if ensurer.target != "main-pool" || len(ensurer.headers) != 1 {
			t.Fatalf("ensurer call = %q %+v", ensurer.target, ensurer.headers)
		}
		got, _ := store.Get("hdr-ext")
		if !got.Applied {
			t.Fatal("full-apply must set Applied=true")
		}
	})

	t.Run("reports unavailable without ensurer", func(t *testing.T) {
		h, _ := newSetup(t)
		rec := doFullApply(t, h, `{"session_hub_target":"main-pool"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Headers    []ExtensionHeader `json:"headers"`
			SessionHub map[string]any    `json:"session_hub"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if installed, _ := body.SessionHub["installed"].(bool); installed {
			t.Fatalf("session_hub = %+v, want installed=false", body.SessionHub)
		}
		if len(body.Headers) == 0 {
			t.Fatal("headers must remain in the response for frontend ensure-headers chaining")
		}
	})

	t.Run("omits session_hub without target", func(t *testing.T) {
		h, _ := newSetup(t)
		ensurer := &fakeHeaderEnsurer{}
		h.SetSessionHeaderEnsurer(ensurer)
		rec := doFullApply(t, h, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, exists := body["session_hub"]; exists {
			t.Fatalf("session_hub must be absent without target: %v", body)
		}
		if ensurer.target != "" {
			t.Fatalf("ensurer must not be called, got %q", ensurer.target)
		}
	})
}
