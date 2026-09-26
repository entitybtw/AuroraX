package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/internal/addon"
)

func TestExtensionStore_ImportOnlyNoBuiltin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.json")
	t.Setenv("AURORA_EXTENSIONS_PATH", path)

	store := NewExtensionStore()
	if len(store.List()) != 0 {
		t.Fatalf("expected no built-in extensions, got %d", len(store.List()))
	}
	if _, ok := store.Get("cli-emulation"); ok {
		t.Fatal("cli-emulation must not be built-in")
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
	p := Extension{ID: "cli-emulation"}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if p.ID != "cli-emulation" {
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
	  "homepage": "https://example.test",
	  "provides": {"provider_types": ["cli-emulation"], "features": ["oauth"]},
	  "inject_tool_types": ["cli-emulation"],
	  "tool_schemas": [{"name":"custom","source":"custom"},{"name":"inline","inline":[{"type":"function"}]}],
	  "settings": {"transport": "bun-tls"},
	  "oauth": {"server":"https://auth.example.test","client_id":"cli","verification_base":"https://auth.example.test"},
	  "files": {"tools/cli-emulation.json":"[]","scripts/helper.js":"// helper"},
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
	if len(p.Files) != 2 || p.Files["tools/cli-emulation.json"] != "[]" {
		t.Fatalf("files not parsed: %+v", p.Files)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(p.InjectTypes) != 1 || p.InjectTypes[0] != "cli-emulation" {
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

// TestExtensionValidate_ReportsEveryBrokenField checks that an import failure
// lists all problems at once (with field paths), not just the first one.
func TestExtensionValidate_ReportsEveryBrokenField(t *testing.T) {
	p := Extension{
		ID:     "broken ext",
		Schema: 9,
		BaseURL: "not-a-url",
		Headers: []ExtensionHeader{
			{Name: "x-ok", Mode: "static", Length: 0},
			{Name: "", Mode: "wobble", Charset: "base64"},
			{Name: "x-gen", Mode: "generate", Length: 0, Charset: "hex"},
		},
		OAuth:    &ExtensionOAuth{Grant: "implicit", ClientID: ""},
		Provides: &ExtensionProvides{ProviderTypes: []string{"bad type"}},
		Tools:    []ExtensionTool{{}},
		UI: ExtensionUI{Fields: []ExtensionField{
			{Key: "a", Type: "wysiwyg"},
			{Key: "a", Type: "text"},
			{Key: "", Type: "select"},
		}},
		Settings: map[string]string{"forward_headers": "not-json"},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	msg := err.Error()
	for _, want := range []string{
		"id:",
		"schema:",
		"base_url:",
		"headers[1].name:",
		"headers[1].mode:",
		"headers[2].length:",
		"oauth.grant:",
		"oauth.client_id:",
		"provides.provider_types[0]:",
		"tool_schemas[0]:",
		"ui.fields[0].type:",
		"ui.fields[1].key: duplicate",
		"ui.fields[2].key:",
		"settings.forward_headers:",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestExtension_MaterializeFiles(t *testing.T) {
	dir := t.TempDir()
	ext := Extension{
		ID: "x",
		Files: map[string]string{
			"tools/cli-emulation-schema.json": `[]`,
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
	if !strings.HasSuffix(toolsPath, filepath.Join("tools", "cli-emulation-schema.json")) {
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
		Name:    "public-tier-main",
		Type:    "vllm",
		BaseURL: "https://zen.example.com/v1",
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
		ID:      "cli-emulation",
		Name:    "Public Tier Profile",
		BaseURL: "https://zen.example.com/v1",
		OAuth: &ExtensionOAuth{
			Server:           "https://auth.example.com/device",
			ClientID:         "aurora-cli",
			VerificationBase: "https://example.com",
		},
		Provides: &ExtensionProvides{
			ProviderTypes: []string{"cli-emulation"},
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
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/cli-emulation/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "cli-emulation"}})
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
	if len(body.OAuthProviders) != 1 || body.OAuthProviders[0] != "public-tier-main" {
		t.Fatalf("oauth_providers = %v, want [public-tier-main]", body.OAuthProviders)
	}

	matched, ok := overrides.get("public-tier-main")
	if !ok {
		t.Fatal("public-tier override missing")
	}
	if matched.AuthMethod != "oauth" || matched.OAuthServer != "https://auth.example.com/device" || matched.OAuthClientID != "aurora-cli" {
		t.Fatalf("public-tier oauth not applied: %+v", matched)
	}
	other, _ := overrides.get("openrouter-main")
	if other.AuthMethod == "oauth" {
		t.Fatalf("non-matching provider must not get oauth: %+v", other)
	}

	// Sidecar defaults also carry OAuth endpoints.
	side := h.sidecarStore.get()
	if side.OAuthServer != "https://auth.example.com/device" || side.OAuthClientID != "aurora-cli" {
		t.Fatalf("sidecar oauth defaults not set: %+v", side)
	}
}

func TestApplyExtension_StampsClientProfileOnPoolProviders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_PROVIDER_OVERRIDES_PATH", filepath.Join(dir, "provider-overrides.json"))
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "sidecar-overrides.json"))

	overrides := NewProviderOverrideStore()
	on := true
	for _, name := range []string{"vllm-zen-main", "vllm-zen-backup"} {
		overrides.upsert(ProviderOverride{
			Name:    name,
			Type:    "vllm",
			BaseURL: "https://zen.example.com/v1",
			BindIP:  name,
			Enabled: &on,
		})
	}
	overrides.upsert(ProviderOverride{
		Name:    "openrouter-main",
		Type:    "openrouter",
		BaseURL: "https://openrouter.ai/api/v1",
		Enabled: &on,
	})

	store := NewExtensionStore()
	ext := Extension{
		ID:        "cli-emulation",
		Name:      "Public Tier Profile",
		BaseURL:   "https://zen.example.com/v1",
		UserAgent: "cli/1.0 runtime/bun/1.3.14",
		Settings:  map[string]string{"sidecar_url": "http://127.0.0.1:8090/v1"},
		Provides:  &ExtensionProvides{ProviderTypes: []string{"cli-emulation"}},
	}
	store.Upsert(ext)

	h := NewHandler(nil, nil,
		WithExtensionStore(store),
		WithSidecarStore(NewSidecarOverrideStore()),
		WithProviderOverrides(overrides),
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/cli-emulation/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "cli-emulation"}})
	if err := h.ApplyExtension(c); err != nil {
		t.Fatalf("ApplyExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		ProfileProviders []string `json:"profile_providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.ProfileProviders) != 2 {
		t.Fatalf("profile_providers = %v, want 2 pool members", body.ProfileProviders)
	}
	for _, name := range []string{"vllm-zen-main", "vllm-zen-backup"} {
		got, ok := overrides.get(name)
		if !ok {
			t.Fatalf("%s override missing", name)
		}
		if got.UserAgent != "cli/1.0 runtime/bun/1.3.14" || got.SidecarURL != "http://127.0.0.1:8090/v1" {
			t.Fatalf("%s client profile not applied: %+v", name, got)
		}
	}
	other, _ := overrides.get("openrouter-main")
	if other.UserAgent != "" || other.SidecarURL != "" {
		t.Fatalf("non-matching provider must keep empty client profile: %+v", other)
	}
	// Raw config conversion must carry the sidecar URL through.
	raw := overrides.RawConfigs()
	if raw["vllm-zen-main"].SidecarURL != "http://127.0.0.1:8090/v1" {
		t.Fatalf("raw provider sidecar_url lost: %+v", raw["vllm-zen-main"])
	}
	if raw["vllm-zen-main"].UserAgent != "cli/1.0 runtime/bun/1.3.14" {
		t.Fatalf("raw provider user_agent lost: %+v", raw["vllm-zen-main"])
	}
}

func TestHandler_ExtensionsHaveNoTags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	// Import with tags in the JSON — field must be dropped (not in schema).
	store := NewExtensionStore()
	raw := `{
	  "id": "no-tags",
	  "name": "No Tags",
	  "type": "theme",
	  "tags": ["official", "theme"]
	}`
	var imported Extension
	if err := json.Unmarshal([]byte(raw), &imported); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	exported, err := json.Marshal(imported)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(exported), `"tags"`) {
		t.Fatalf("tags must not round-trip on Extension: %s", exported)
	}
	store.Upsert(imported)

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensions(c); err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	if strings.Contains(rec.Body.String(), `"tags"`) {
		t.Fatalf("list response must not contain tags: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/sidecar/extensions/no-tags", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "no-tags"}})
	if err := h.GetExtension(c); err != nil {
		t.Fatalf("GetExtension: %v", err)
	}
	if strings.Contains(rec.Body.String(), `"tags"`) {
		t.Fatalf("get response must not contain tags: %s", rec.Body.String())
	}
}

func TestExtensionStore_ImportPreservesOperatorState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{
		ID:      "ext",
		Name:    "Custom Name",
		Version: "1",
		Applied: true,
		Order:   3,
		Config:  map[string]string{"accent": "#112233"},
		Source:  "https://store.example.com/extensions/ext.extension.json",
	})

	store.Import(Extension{
		ID:          "ext",
		Name:        "Upstream Name",
		Version:     "2",
		Description: "updated",
		Config:      map[string]string{"accent": "#445566", "extra": "x"},
	})

	got, ok := store.Get("ext")
	if !ok {
		t.Fatal("extension missing after re-import")
	}
	if got.Name != "Custom Name" {
		t.Fatalf("Name = %q, want Custom Name", got.Name)
	}
	if !got.Applied {
		t.Fatal("Applied must be preserved")
	}
	if got.Order != 3 {
		t.Fatalf("Order = %d, want 3", got.Order)
	}
	if got.Version != "2" {
		t.Fatalf("Version = %q, want 2 (metadata should update)", got.Version)
	}
	if got.Source != "https://store.example.com/extensions/ext.extension.json" {
		t.Fatalf("Source = %q, want preserved source URL", got.Source)
	}
	if got.Config["accent"] != "#112233" || got.Config["extra"] != "x" {
		t.Fatalf("Config merge = %v, want accent kept + extra added", got.Config)
	}
}

func TestHandler_ExtensionStoreURLsCRUD(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSION_STORES_PATH", filepath.Join(dir, "stores.json"))
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	h := NewHandler(nil, nil)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/stores", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensionStores(c); err != nil {
		t.Fatalf("ListExtensionStores: %v", err)
	}
	var listBody struct {
		Stores []string `json:"stores"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listBody.Stores) != 0 {
		t.Fatalf("stores = %v, want empty", listBody.Stores)
	}

	req = httptest.NewRequest(http.MethodPost, "/sidecar/extensions/stores", strings.NewReader(
		`{"url":"https://store.example.com"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.AddExtensionStore(c); err != nil {
		t.Fatalf("AddExtensionStore: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("add status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/sidecar/extensions/stores", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.ListExtensionStores(c); err != nil {
		t.Fatalf("ListExtensionStores after add: %v", err)
	}
	listBody.Stores = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list after add: %v", err)
	}
	if len(listBody.Stores) != 1 || listBody.Stores[0] != "https://store.example.com" {
		t.Fatalf("stores = %v, want [https://store.example.com]", listBody.Stores)
	}

	req = httptest.NewRequest(http.MethodDelete, "/sidecar/extensions/stores?url="+
		url.QueryEscape("https://store.example.com"), nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.DeleteExtensionStore(c); err != nil {
		t.Fatalf("DeleteExtensionStore: %v", err)
	}
	listBody.Stores = []string{"sentinel"}
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	if len(listBody.Stores) != 0 {
		t.Fatalf("stores after delete = %v, want empty", listBody.Stores)
	}
}

func TestHandler_CheckAndUpdateExtensionFromSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	remote := `{"id":"src-ext","name":"Src","version":"2","type":"sidecar","base_url":"https://api.example.com/v1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remote))
	}))
	defer srv.Close()

	store := NewExtensionStore()
	store.Upsert(Extension{
		ID:      "src-ext",
		Name:    "My Rename",
		Version: "1",
		Applied: true,
		Order:   5,
		Config:  map[string]string{"k": "v"},
		Source:  srv.URL + "/ext.json",
	})
	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/src-ext/check-update", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "src-ext"}})
	if err := h.CheckExtensionUpdate(c); err != nil {
		t.Fatalf("CheckExtensionUpdate: %v", err)
	}
	var check struct {
		UpdateAvailable bool   `json:"update_available"`
		RemoteVersion   string `json:"remote_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode check: %v", err)
	}
	if !check.UpdateAvailable || check.RemoteVersion != "2" {
		t.Fatalf("check = %+v, want update available v2", check)
	}

	req = httptest.NewRequest(http.MethodPost, "/sidecar/extensions/src-ext/update", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "src-ext"}})
	if err := h.UpdateExtensionFromSource(c); err != nil {
		t.Fatalf("UpdateExtensionFromSource: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}

	got, ok := store.Get("src-ext")
	if !ok {
		t.Fatal("extension missing after update")
	}
	if got.Version != "2" {
		t.Fatalf("Version = %q, want 2", got.Version)
	}
	if got.Name != "My Rename" {
		t.Fatalf("Name = %q, want preserved custom name", got.Name)
	}
	if !got.Applied || got.Order != 5 {
		t.Fatalf("state not preserved: applied=%v order=%d", got.Applied, got.Order)
	}
	if got.Config["k"] != "v" {
		t.Fatalf("Config = %v, want preserved", got.Config)
	}
	if got.Source != srv.URL+"/ext.json" {
		t.Fatalf("Source = %q, want preserved", got.Source)
	}
}

func TestHandler_UpdateExtensionSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{ID: "src-edit", Name: "Src", Version: "1", Type: "sidecar"})
	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/sidecar/extensions/src-edit", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: "src-edit"}})
		if err := h.UpdateExtension(c); err != nil {
			t.Fatalf("UpdateExtension(%s): %v", body, err)
		}
		return rec
	}

	rec := put(`{"source":"https://store.example.com/extensions/src-edit.extension.json"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("set source status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := store.Get("src-edit"); got.Source != "https://store.example.com/extensions/src-edit.extension.json" {
		t.Fatalf("Source = %q, want stored", got.Source)
	}

	rec = put(`{"source":"ftp://bad.example/ext.json"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad scheme status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = put(`{"source":"javascript:alert(1)"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad scheme status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = put(`{"source":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear source status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := store.Get("src-edit"); got.Source != "" {
		t.Fatalf("Source = %q, want cleared", got.Source)
	}
}

func TestHandler_CheckUpdateResolvesFromStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_EXTENSION_STORES_PATH", filepath.Join(dir, "stores.json"))

	remote := `{"id":"store-ext","name":"Store Ext","version":"3","type":"sidecar","base_url":"https://api.example.com/v1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/extensions/store-ext.extension.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remote))
	}))
	defer srv.Close()

	store := NewExtensionStore()
	// Imported as raw JSON: no Source recorded.
	store.Upsert(Extension{ID: "store-ext", Name: "Store Ext", Version: "1", Type: "sidecar"})
	h := NewHandler(nil, nil, WithExtensionStore(store))
	h.extensionStores().Add(srv.URL)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/store-ext/check-update", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "store-ext"}})
	if err := h.CheckExtensionUpdate(c); err != nil {
		t.Fatalf("CheckExtensionUpdate: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", rec.Code, rec.Body.String())
	}
	var check struct {
		Source          string `json:"source"`
		RemoteVersion   string `json:"remote_version"`
		UpdateAvailable bool   `json:"update_available"`
		Resolved        bool   `json:"resolved"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode check: %v", err)
	}
	if !check.UpdateAvailable || check.RemoteVersion != "3" {
		t.Fatalf("check = %+v, want update available v3", check)
	}
	if check.Source != srv.URL+"/extensions/store-ext.extension.json" {
		t.Fatalf("Source = %q, want resolved store URL", check.Source)
	}
	if !check.Resolved {
		t.Fatal("resolved = false, want true (source came from a store)")
	}

	// Update from the store-resolved source; metadata refreshes in place.
	req = httptest.NewRequest(http.MethodPost, "/sidecar/extensions/store-ext/update", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "store-ext"}})
	if err := h.UpdateExtensionFromSource(c); err != nil {
		t.Fatalf("UpdateExtensionFromSource: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, ok := store.Get("store-ext")
	if !ok || got.Version != "3" {
		t.Fatalf("Version = %q, want 3", got.Version)
	}
	if got.Source != srv.URL+"/extensions/store-ext.extension.json" {
		t.Fatalf("Source = %q, want resolved store URL persisted for future syncs", got.Source)
	}
}

func TestHandler_CheckUpdateNoSourceAnywhere(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_EXTENSION_STORES_PATH", filepath.Join(dir, "stores.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{ID: "orphan", Name: "Orphan", Version: "1", Type: "sidecar"})
	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/orphan/check-update", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "orphan"}})
	if err := h.CheckExtensionUpdate(c); err != nil {
		t.Fatalf("CheckExtensionUpdate: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rec.Code, rec.Body.String())
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

func TestHandler_ResetExtensionConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_EXTENSION_STORES_PATH", filepath.Join(dir, "stores.json"))

	store := NewExtensionStore()
	store.Upsert(Extension{
		ID:     "reset-ext",
		Name:   "Reset",
		Source: "https://store.example.com/extensions/reset-ext.extension.json",
		UI: ExtensionUI{
			Fields: []ExtensionField{
				{Key: "accent", Label: "Accent", Type: "color", Default: "#5b8def"},
			},
		},
		Config: map[string]string{"accent": "#ff0000"},
	})
	store.Upsert(Extension{
		ID:     "no-source-ext",
		Name:   "NoSource",
		Config: map[string]string{"accent": "#00ff00"},
	})

	h := NewHandler(nil, nil, WithExtensionStore(store))
	e := echo.New()

	doReset := func(id string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/"+id+"/config/reset", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: id}})
		if err := h.ResetExtensionConfig(c); err != nil {
			t.Fatalf("ResetExtensionConfig(%s): %v", id, err)
		}
		return rec
	}

	// With a sync source the overrides are cleared and defaults win again.
	rec := doReset("reset-ext")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get("reset-ext")
	if got.Config != nil {
		t.Fatalf("config must be cleared, got %+v", got.Config)
	}

	// Without any resolvable source the reset is refused and config is kept.
	rec = doReset("no-source-ext")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("no-source status = %d, want 400", rec.Code)
	}
	got, _ = store.Get("no-source-ext")
	if got.Config["accent"] != "#00ff00" {
		t.Fatalf("config must be preserved on refusal: %+v", got.Config)
	}

	// Unknown extension → 404.
	rec = doReset("missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", rec.Code)
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
			Theme:      map[string]string{"--accent": "#112233"},
			ThemeLight: map[string]string{"--bg": "#fafafa", "--accent": "#aa00cc"},
			ThemeDark:  map[string]string{"--bg": "#0a0a0a", "--accent": "#aa00cc"},
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
	ui := body.Contributions[0].UI
	if ui.Theme["--accent"] != "#aabbcc" {
		t.Fatalf("color accent field must override --accent, theme=%v", ui.Theme)
	}
	if ui.Theme["--radius"] != "12px" {
		t.Fatalf("-- config key must land in theme, theme=%v", ui.Theme)
	}
	if ui.ThemeLight == nil || ui.ThemeLight["--bg"] != "#fafafa" {
		t.Fatalf("theme_light base keys must survive: %v", ui.ThemeLight)
	}
	if ui.ThemeLight["--accent"] != "#aabbcc" {
		t.Fatalf("config accent must override theme_light accent: %v", ui.ThemeLight)
	}
	if ui.ThemeDark == nil || ui.ThemeDark["--bg"] != "#0a0a0a" {
		t.Fatalf("theme_dark base keys must survive: %v", ui.ThemeDark)
	}
	if ui.ThemeDark["--radius"] != "12px" {
		t.Fatalf("config --radius must overlay theme_dark: %v", ui.ThemeDark)
	}
	if ui.ThemeLight["--radius"] != "12px" {
		t.Fatalf("config --radius must overlay theme_light: %v", ui.ThemeLight)
	}

	stored, _ := store.Get("ui-cfg")
	if stored.UI.Theme["--accent"] != "#112233" {
		t.Fatalf("stored theme must not be mutated: %v", stored.UI.Theme)
	}
	if stored.UI.ThemeLight["--accent"] != "#aa00cc" {
		t.Fatalf("stored theme_light must not be mutated: %v", stored.UI.ThemeLight)
	}
}

func TestHandler_ExtensionUI_ThemeVariantsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))

	raw := `{
	  "id": "variants",
	  "name": "Variants",
	  "type": "theme",
	  "ui": {
	    "accent": "#112233",
	    "theme": {"--bg": "#000000"},
	    "theme_light": {"--bg": "#ffffff", "--text": "#111111"},
	    "theme_dark": {"--bg": "#000000", "--text": "#eeeeee"}
	  }
	}`
	var ext Extension
	if err := json.Unmarshal([]byte(raw), &ext); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ext.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	out, err := json.Marshal(ext)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"theme_light"`) || !strings.Contains(string(out), `"theme_dark"`) {
		t.Fatalf("theme variants must round-trip: %s", out)
	}

	store := NewExtensionStore()
	store.Upsert(ext)
	applied, _ := store.Get("variants")
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
	var body struct {
		Contributions []ExtensionUIContribution `json:"contributions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Contributions) != 1 {
		t.Fatalf("contributions = %d, want 1", len(body.Contributions))
	}
	ui := body.Contributions[0].UI
	if ui.ThemeLight["--bg"] != "#ffffff" || ui.ThemeDark["--bg"] != "#000000" {
		t.Fatalf("list UI must expose variants: light=%v dark=%v", ui.ThemeLight, ui.ThemeDark)
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

// TestUnapplyExtension_DetachesAddonTabs verifies that disabling an extension
// unloads its shipped Go addons, so its settings tabs (e.g. the auth login
// tabs) disappear from the dashboard right away instead of surviving until a
// gateway restart.
func TestUnapplyExtension_DetachesAddonTabs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	filesBase := filepath.Join(dir, "files")
	t.Setenv("AURORA_EXTENSIONS_FILES_DIR", filesBase)
	filesDir := filepath.Join(filesBase, "auth-ext")

	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package main\n\n// addon-kind: auth\nfunc UI() string { return `{\"settings_tabs\":[{\"id\":\"tab1\",\"label\":\"Tab\",\"blocks\":[]}]}` }\n"
	if err := os.WriteFile(filepath.Join(filesDir, "auth-test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	extStore := NewExtensionStore()
	extStore.Upsert(Extension{ID: "auth-ext", Name: "AuthExt", Applied: true})
	addonStore := addon.NewStore(filepath.Join(dir, "addons"))

	h := NewHandler(nil, nil, WithExtensionStore(extStore), WithAddonStore(addonStore))
	AttachExtensionAddons(addonStore, "auth-ext")
	if got := addonStore.ByKind(addon.KindAuth); len(got) != 1 {
		t.Fatalf("addons loaded = %d, want 1; filesDir=%s env=%s status=%+v", len(got), filesDir, os.Getenv("AURORA_EXTENSIONS_FILES_DIR"), addonStore.Status())
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/auth-ext/unapply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "auth-ext"}})
	if err := h.UnapplyExtension(c); err != nil {
		t.Fatalf("UnapplyExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := addonStore.ByKind(addon.KindAuth); len(got) != 0 {
		t.Fatalf("addons after unapply = %d, want 0 (tabs must disappear)", len(got))
	}
	for _, d := range addonStore.Dirs() {
		if d == filesDir {
			t.Fatalf("extension dir must be dropped from scan set: %v", addonStore.Dirs())
		}
	}
}
