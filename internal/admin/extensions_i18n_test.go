package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

// newI18nTestHandler wires an isolated handler (extension store, sidecar
// store, locale store) seeded with the given extensions.
func newI18nTestHandler(t *testing.T, exts ...Extension) (*Handler, *ExtensionStore) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AURORA_EXTENSIONS_PATH", filepath.Join(dir, "extensions.json"))
	t.Setenv("AURORA_EXTENSIONS_FILES_DIR", filepath.Join(dir, "files"))
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "sidecar.json"))
	t.Setenv("AURORA_EXTENSION_LOCALE_PATH", filepath.Join(dir, "locale.json"))
	store := NewExtensionStore()
	for _, e := range exts {
		store.Upsert(e)
	}
	h := NewHandler(nil, nil,
		WithExtensionStore(store),
		WithSidecarStore(NewSidecarOverrideStore()),
		WithExtensionLocaleStore(NewExtensionLocaleStore()),
	)
	return h, store
}

// demoI18nExtension is a small extension exercising every translated surface.
func demoI18nExtension() Extension {
	return Extension{
		ID:      "demo-ext",
		Name:    "Demo Extension",
		Tagline: "A demo",
		UI: ExtensionUI{
			Help:     "English help",
			LogoText: "Demo",
			Fields: []ExtensionField{
				{Key: "region", Label: "Region", Description: "Where to connect"},
			},
			Nav: []ExtensionNavEntry{
				{ID: "demo", Label: "Demo page", To: "/admin/dashboard/ext/demo"},
			},
			SettingsTabs: []ExtensionSettingsTab{
				{ID: "demo-tab", Label: "Demo settings",
					Blocks: []ExtensionUIBlock{{Kind: "text", Text: "Tab body text"}}},
			},
			Pages: []ExtensionUIPage{
				{ID: "demo", Path: "demo", Title: "Demo page", Summary: "Page summary",
					Blocks: []ExtensionUIBlock{
						{Kind: "text", Text: "Block zero"},
						{Kind: "list", Items: []string{"step one", "step two"}},
					}},
			},
		},
		I18n: &ExtensionI18n{
			Locales: map[string]ExtensionTranslation{
				"ru": {
					Name:    "Демо-расширение",
					Tagline: "Демо",
					UI: &ExtensionUITranslation{
						Help:     "Справка на русском",
						LogoText: "Демо",
						Fields: map[string]ExtensionFieldTranslation{
							"region": {Label: "Регион", Description: "Куда подключаться"},
						},
						Nav: map[string]string{"demo": "Страница демо"},
						Pages: map[string]ExtensionPageTranslation{"demo": {Title: "Страница демо", Summary: "Сводка", Blocks: map[string]ExtensionBlockTranslation{
							"0": {Text: "Блок ноль"},
							"1": {Items: []string{"шаг один", "шаг два"}},
						}}},
						SettingsTabs: map[string]ExtensionSettingsTabTranslation{
							"demo-tab": {Label: "Настройки демо", Blocks: map[string]ExtensionBlockTranslation{"0": {Text: "Текст вкладки"}}},
						},
					},
				},
				"pt-br": {
					Name: "Extensão Demo",
					UI:   &ExtensionUITranslation{Help: "Ajuda em português"},
				},
			},
		},
	}
}

// ruLanguagePack is an applied language pack overriding the embedded Russian
// translation of demo-ext's name.
func ruLanguagePack() Extension {
	return Extension{
		ID:      "locale-ru",
		Name:    "Русский язык",
		Type:    "localization",
		Locale:  "ru",
		Applied: true,
		Catalog: map[string]ExtensionTranslation{
			"demo-ext": {
				Name: "Демо-расширение (пакет)",
				UI: &ExtensionUITranslation{
					Help: "Справка из пакета",
				},
			},
		},
	}
}

func findExt(t *testing.T, list []Extension, id string) Extension {
	t.Helper()
	for _, e := range list {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("extension %q not in list", id)
	return Extension{}
}

func doListExtensions(t *testing.T, h *Handler, query string) []Extension {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensions(c); err != nil {
		t.Fatalf("ListExtensions: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ListExtensions status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Extensions []Extension `json:"extensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Extensions
}

// TestExtensionLocaleStore_PersistsNormalized: the active locale round-trips
// through disk and is normalized ("PT_br" → "pt-br"); "" resets.
func TestExtensionLocaleStore_PersistsNormalized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locale.json")
	t.Setenv("AURORA_EXTENSION_LOCALE_PATH", path)

	s := NewExtensionLocaleStore()
	if got := s.Get(); got != "" {
		t.Fatalf("fresh locale = %q, want empty", got)
	}
	if got := s.Set("PT_br"); got != "pt-br" {
		t.Fatalf("Set(PT_br) = %q, want pt-br", got)
	}
	reloaded := NewExtensionLocaleStore()
	if got := reloaded.Get(); got != "pt-br" {
		t.Fatalf("reloaded locale = %q, want pt-br", got)
	}
	if got := reloaded.Set(""); got != "" {
		t.Fatalf("reset = %q, want empty", got)
	}
	if got := NewExtensionLocaleStore().Get(); got != "" {
		t.Fatalf("locale after reset = %q, want empty", got)
	}
}

// TestPickLocale_Fallbacks locks the locale resolution rules: exact match,
// bare base-language fallback, single region variant, zh → zh-CN, and no
// guessing between competing region variants.
func TestPickLocale_Fallbacks(t *testing.T) {
	cases := []struct {
		want  string
		avail map[string]bool
		out   string
	}{
		{"ru", map[string]bool{"ru": true}, "ru"},
		{"RU", map[string]bool{"ru": true}, "ru"},
		{"pt-br", map[string]bool{"pt": true}, "pt"},
		{"pt-BR", map[string]bool{"pt-BR": true}, "pt-BR"},
		{"zh", map[string]bool{"zh-cn": true, "zh-tw": true}, "zh-cn"},
		{"zh-tw", map[string]bool{"zh-cn": true, "zh-tw": true}, "zh-tw"},
		{"pt-pt", map[string]bool{"pt-br": true}, "pt-br"},
		{"fr-ca", map[string]bool{"fr": true, "fr-be": true}, "fr"},
		{"fr", map[string]bool{"de": true}, ""},
		{"fr", map[string]bool{}, ""},
		{"", map[string]bool{"ru": true}, ""},
		// Two region variants, no bare base: stay with the source text.
		{"sr", map[string]bool{"sr-cyrl": true, "sr-latn": true}, ""},
	}
	for _, tc := range cases {
		if got := pickLocale(tc.want, tc.avail); got != tc.out {
			t.Errorf("pickLocale(%q, %v) = %q, want %q", tc.want, tc.avail, got, tc.out)
		}
	}
}

// TestListExtensions_LocalizesAndKeepsSourceIntact: with locale=ru the list
// carries Russian strings (pack overriding embedded), while the stored record
// and GetExtension (the JSON editor path) stay verbatim.
func TestListExtensions_LocalizesAndKeepsSourceIntact(t *testing.T) {
	h, store := newI18nTestHandler(t, demoI18nExtension(), ruLanguagePack())

	// No locale → source text.
	list := doListExtensions(t, h, "")
	demo := findExt(t, list, "demo-ext")
	if demo.Name != "Demo Extension" {
		t.Fatalf("default name = %q, want source text", demo.Name)
	}

	// Available locales come from embedded i18n (pt-br, ru) + applied packs (ru).
	if got := h.availableExtensionLocales("ru"); len(got) != 2 || got[0] != "pt-br" || got[1] != "ru" {
		t.Fatalf("available = %v, want [pt-br ru]", got)
	}

	// Activate ru via the persisted store.
	h.extensionLocale().Set("ru")
	list = doListExtensions(t, h, "")
	demo = findExt(t, list, "demo-ext")
	if demo.Name != "Демо-расширение (пакет)" {
		t.Fatalf("name = %q, want pack override", demo.Name)
	}
	if demo.Tagline != "Демо" {
		t.Fatalf("tagline = %q, want embedded translation", demo.Tagline)
	}
	if demo.UI.Help != "Справка из пакета" {
		t.Fatalf("help = %q, want pack translation (pack beats embedded)", demo.UI.Help)
	}
	if demo.UI.LogoText != "Демо" {
		t.Fatalf("logo_text = %q, want embedded translation", demo.UI.LogoText)
	}
	if len(demo.UI.Fields) != 1 || demo.UI.Fields[0].Label != "Регион" ||
		demo.UI.Fields[0].Description != "Куда подключаться" {
		t.Fatalf("fields = %+v, want translated label/description", demo.UI.Fields)
	}
	if len(demo.UI.Nav) != 1 || demo.UI.Nav[0].Label != "Страница демо" {
		t.Fatalf("nav = %+v, want translated label", demo.UI.Nav)
	}
	if len(demo.UI.Pages) != 1 || demo.UI.Pages[0].Title != "Страница демо" ||
		demo.UI.Pages[0].Summary != "Сводка" ||
		len(demo.UI.Pages[0].Blocks) != 2 || demo.UI.Pages[0].Blocks[0].Text != "Блок ноль" ||
		len(demo.UI.Pages[0].Blocks[1].Items) != 2 || demo.UI.Pages[0].Blocks[1].Items[0] != "шаг один" {
		t.Fatalf("pages = %+v, want translated page + block text + list items", demo.UI.Pages)
	}
	if len(demo.UI.SettingsTabs) != 1 || demo.UI.SettingsTabs[0].Label != "Настройки демо" ||
		len(demo.UI.SettingsTabs[0].Blocks) != 1 ||
		demo.UI.SettingsTabs[0].Blocks[0].Text != "Текст вкладки" {
		t.Fatalf("settings_tabs = %+v, want translated tab + block text", demo.UI.SettingsTabs)
	}
	// The language pack itself lists in its own language.
	pack := findExt(t, list, "locale-ru")
	if pack.Name != "Русский язык" || pack.Locale != "ru" || pack.Type != "localization" {
		t.Fatalf("pack = %+v", pack)
	}

	// Stored record untouched.
	stored, _ := store.Get("demo-ext")
	if stored.Name != "Demo Extension" || stored.UI.Help != "English help" {
		t.Fatalf("stored record mutated: name=%q help=%q", stored.Name, stored.UI.Help)
	}
	if stored.UI.Fields[0].Label != "Region" {
		t.Fatalf("stored fields mutated: %+v", stored.UI.Fields)
	}
	if len(stored.UI.Pages) != 1 || len(stored.UI.Pages[0].Blocks) != 2 ||
		stored.UI.Pages[0].Blocks[1].Items[0] != "step one" {
		t.Fatalf("stored page blocks mutated: %+v", stored.UI.Pages)
	}

	// GetExtension (JSON editor) stays raw.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/demo-ext", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "demo-ext"}})
	if err := h.GetExtension(c); err != nil {
		t.Fatalf("GetExtension: %v", err)
	}
	var raw Extension
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw.Name != "Demo Extension" || raw.UI.Help != "English help" {
		t.Fatalf("GetExtension localized the editor document: name=%q help=%q", raw.Name, raw.UI.Help)
	}
}

// TestListExtensions_LocaleQueryOverride: ?locale= wins over the persisted
// setting; an unknown locale falls back to the source text.
func TestListExtensions_LocaleQueryOverride(t *testing.T) {
	h, _ := newI18nTestHandler(t, demoI18nExtension(), ruLanguagePack())
	h.extensionLocale().Set("ru")

	list := doListExtensions(t, h, "?locale=pt-br")
	demo := findExt(t, list, "demo-ext")
	if demo.Name != "Extensão Demo" || demo.UI.Help != "Ajuda em português" {
		t.Fatalf("pt-br override: name=%q help=%q", demo.Name, demo.UI.Help)
	}

	list = doListExtensions(t, h, "?locale=de")
	demo = findExt(t, list, "demo-ext")
	if demo.Name != "Demo Extension" {
		t.Fatalf("unknown locale should keep source text, got %q", demo.Name)
	}
}

// TestListExtensionUI_LocalizesContributions: applied extension UI comes back
// translated, and the stored contribution stays English.
func TestListExtensionUI_LocalizesContributions(t *testing.T) {
	ext := demoI18nExtension()
	ext.Applied = true
	h, store := newI18nTestHandler(t, ext, ruLanguagePack())
	h.extensionLocale().Set("ru")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/ui", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ListExtensionUI(c); err != nil {
		t.Fatalf("ListExtensionUI: %v", err)
	}
	var resp struct {
		Contributions []ExtensionUIContribution `json:"contributions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Contributions) != 1 {
		t.Fatalf("contributions = %d, want 1 (%s)", len(resp.Contributions), rec.Body.String())
	}
	got := resp.Contributions[0]
	if got.Name != "Демо-расширение (пакет)" || got.UI.Help != "Справка из пакета" {
		t.Fatalf("contribution not localized: name=%q help=%q", got.Name, got.UI.Help)
	}
	if len(got.UI.SettingsTabs) != 1 || got.UI.SettingsTabs[0].Label != "Настройки демо" {
		t.Fatalf("settings tab not localized: %+v", got.UI.SettingsTabs)
	}
	stored, _ := store.Get("demo-ext")
	if stored.UI.Help != "English help" {
		t.Fatalf("stored UI mutated: %q", stored.UI.Help)
	}
}

// TestApplyLocalizationPack_LeavesSidecarAlone: applying a language pack
// flips Applied without ever touching sidecar settings.
func TestApplyLocalizationPack_LeavesSidecarAlone(t *testing.T) {
	h, store := newI18nTestHandler(t, demoI18nExtension(), ruLanguagePack())

	before := h.sidecarStore.get()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/locale-ru/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "locale-ru"}})
	if err := h.ApplyExtension(c); err != nil {
		t.Fatalf("ApplyExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status       string `json:"status"`
		Localization bool   `json:"localization"`
		Locale       string `json:"locale"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Localization || resp.Locale != "ru" {
		t.Fatalf("apply response = %s", rec.Body.String())
	}
	pack, _ := store.Get("locale-ru")
	if !pack.Applied {
		t.Fatal("language pack not marked applied")
	}
	after := h.sidecarStore.get()
	if after.Enabled != before.Enabled || after.BaseURL != before.BaseURL || after.DefaultAuth != before.DefaultAuth {
		t.Fatalf("sidecar settings mutated by language pack: before=%+v after=%+v", before, after)
	}

	// Unapply is rejected for language packs: they are install/delete only.
	req2 := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/locale-ru/unapply", nil)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetPathValues(echo.PathValues{{Name: "id", Value: "locale-ru"}})
	if err := h.UnapplyExtension(c2); err != nil {
		t.Fatalf("UnapplyExtension: %v", err)
	}
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("unapply status = %d body=%s, want 400", rec2.Code, rec2.Body.String())
	}
	pack, _ = store.Get("locale-ru")
	if !pack.Applied {
		t.Fatal("language pack lost its applied state on rejected unapply")
	}
}

// TestExtensionLocaleHandlers: GET/PUT /sidecar/extensions/locale contract.
func TestExtensionLocaleHandlers(t *testing.T) {
	h, _ := newI18nTestHandler(t, demoI18nExtension(), ruLanguagePack())
	e := echo.New()

	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/sidecar/extensions/locale", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := h.SetExtensionLocale(c); err != nil {
			t.Fatalf("SetExtensionLocale: %v", err)
		}
		return rec
	}

	// Invalid locale codes are rejected.
	rec := put(`{"locale":"nope!!"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid locale status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = put(`{"locale":"DE"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("set status = %d body=%s", rec.Code, rec.Body.String())
	}
	var setResp struct {
		Locale    string   `json:"locale"`
		Available []string `json:"available"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &setResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if setResp.Locale != "de" {
		t.Fatalf("locale = %q, want de", setResp.Locale)
	}
	// de comes from nowhere yet, but active locale is always listed.
	found := false
	for _, l := range setResp.Available {
		if l == "de" {
			found = true
		}
	}
	if !found {
		t.Fatalf("available = %v, want to include active de", setResp.Available)
	}
	// Applying the ru pack keeps ru available alongside embedded pt-br.
	if got := h.availableExtensionLocales("de"); len(got) != 3 || got[0] != "de" || got[1] != "pt-br" || got[2] != "ru" {
		t.Fatalf("available after pack = %v, want [de pt-br ru]", got)
	}

	// GET reflects the persisted value.
	req := httptest.NewRequest(http.MethodGet, "/sidecar/extensions/locale", nil)
	recGet := httptest.NewRecorder()
	if err := h.GetExtensionLocale(e.NewContext(req, recGet)); err != nil {
		t.Fatalf("GetExtensionLocale: %v", err)
	}
	var getResp struct {
		Locale string `json:"locale"`
	}
	if err := json.Unmarshal(recGet.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if getResp.Locale != "de" {
		t.Fatalf("GET locale = %q, want de", getResp.Locale)
	}

	// Reset to source language.
	if rec := put(`{"locale":""}`); rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d", rec.Code)
	}
	if got := h.extensionLocale().Get(); got != "" {
		t.Fatalf("locale after reset = %q, want empty", got)
	}
}

// TestValidate_LocalePackAndI18n: manifests must pin a locale + catalog for
// language packs, and embedded i18n needs valid codes with content.
func TestValidate_LocalePackAndI18n(t *testing.T) {
	good := Extension{
		ID:     "locale-x",
		Name:   "X pack",
		Type:   "localization",
		Locale: "xx",
		Catalog: map[string]ExtensionTranslation{
			"demo-ext": {Name: "Demo"},
		},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid pack rejected: %v", err)
	}

	noLocale := good
	noLocale.Locale = ""
	err := noLocale.Validate()
	if err == nil || !strings.Contains(err.Error(), "locale: required") {
		t.Fatalf("missing locale error = %v", err)
	}

	noCatalog := good
	noCatalog.Catalog = nil
	err = noCatalog.Validate()
	if err == nil || !strings.Contains(err.Error(), "catalog or dashboard: required") {
		t.Fatalf("missing catalog error = %v", err)
	}

	// A dashboard-only pack (no extension catalog) is valid.
	dashOnly := Extension{
		ID:        "locale-dash",
		Name:      "Dash pack",
		Type:      "localization",
		Locale:    "xx",
		Dashboard: map[string]string{"Save": "Save (xx)"},
	}
	if err := dashOnly.Validate(); err != nil {
		t.Fatalf("dashboard-only pack rejected: %v", err)
	}

	badDash := good
	badDash.Dashboard = map[string]string{"Save": "  "}
	err = badDash.Validate()
	if err == nil || !strings.Contains(err.Error(), `dashboard["Save"]: empty translation`) {
		t.Fatalf("empty dashboard entry error = %v", err)
	}

	emptyEntry := good
	emptyEntry.Catalog = map[string]ExtensionTranslation{"demo-ext": {}}
	err = emptyEntry.Validate()
	if err == nil || !strings.Contains(err.Error(), "empty translation") {
		t.Fatalf("empty translation error = %v", err)
	}

	badCode := Extension{
		ID:   "with-i18n",
		Name: "With i18n",
		I18n: &ExtensionI18n{Locales: map[string]ExtensionTranslation{
			"e": {Name: "Too short"},
		}},
	}
	err = badCode.Validate()
	if err == nil || !strings.Contains(err.Error(), "i18n.locales") {
		t.Fatalf("bad locale code error = %v", err)
	}

	emptyI18n := Extension{
		ID:   "with-i18n",
		Name: "With i18n",
		I18n: &ExtensionI18n{Locales: map[string]ExtensionTranslation{"ru": {}}},
	}
	err = emptyI18n.Validate()
	if err == nil || !strings.Contains(err.Error(), "empty translation") {
		t.Fatalf("empty embedded translation error = %v", err)
	}
}

// TestLocalizationPackJSONRoundTrip: the pack document shape used by the
// store survives unmarshal → marshal unchanged.
func TestLocalizationPackJSONRoundTrip(t *testing.T) {
	raw := `{
	  "id": "locale-de",
	  "name": "Deutsche Sprache",
	  "type": "localization",
	  "locale": "de",
	  "catalog": {
	    "timestamped": {
	      "name": "ExampleClient",
	      "tagline": "ExampleClient-Gateway",
	      "ui": {
	        "help": "Hilfe auf Deutsch",
	        "fields": {"token": {"label": "Token"}},
	        "settings_tabs": {"auth": {"label": "Authentifizierung"}},
	        "pages": {"about": {"title": "Über", "blocks": {"0": {"text": "Text", "items": ["eins", "zwei"]}}}}
	      }
	    }
	  }
	}`
	var p Extension
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if p.Locale != "de" || len(p.Catalog) != 1 {
		t.Fatalf("pack parsed: locale=%q catalog=%d", p.Locale, len(p.Catalog))
	}
	tr := p.Catalog["timestamped"]
	if tr.UI == nil || tr.UI.Help != "Hilfe auf Deutsch" ||
		tr.UI.Fields["token"].Label != "Token" ||
		tr.UI.SettingsTabs["auth"].Label != "Authentifizierung" ||
		tr.UI.Pages["about"].Blocks["0"].Text != "Text" ||
		len(tr.UI.Pages["about"].Blocks["0"].Items) != 2 ||
		tr.UI.Pages["about"].Blocks["0"].Items[0] != "eins" {
		t.Fatalf("catalog content = %+v", tr)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again Extension
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if again.Locale != "de" || again.Catalog["timestamped"].UI.Help != tr.UI.Help {
		t.Fatalf("round trip lost content: %s", out)
	}
}

// TestExtensionLocaleCatalog_DashboardPacks: GET /sidecar/extensions/locale/catalog
// merges dashboard catalogs from installed language packs (Order asc, later
// wins), resolves the locale via pickLocale, and ignores packs without a
// dashboard map.
func TestExtensionLocaleCatalog_DashboardPacks(t *testing.T) {
	base := ruLanguagePack()
	base.Dashboard = map[string]string{"Save": "Сохранить", "Delete": "Удалить"}
	over := Extension{
		ID:        "locale-ru-2",
		Name:      "RU override",
		Type:      "localization",
		Locale:    "ru",
		Order:     50,
		Catalog:   map[string]ExtensionTranslation{"demo-ext": {Name: "Из второго пакета"}},
		Dashboard: map[string]string{"Delete": "Удалять"},
	}
	// A pack with no dashboard map must not break anything.
	noDash := Extension{
		ID:      "locale-pt",
		Name:    "PT pack",
		Type:    "localization",
		Locale:  "pt-br",
		Catalog: map[string]ExtensionTranslation{"demo-ext": {Name: "Demo (pt)"}},
	}
	h, _ := newI18nTestHandler(t, base, over, noDash)

	get := func(query string) (string, map[string]string) {
		t.Helper()
		e := echo.New()
		path := "/sidecar/extensions/locale/catalog"
		if query != "" {
			path += "?" + query
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		if err := h.GetExtensionLocaleCatalog(e.NewContext(req, rec)); err != nil {
			t.Fatalf("GetExtensionLocaleCatalog: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("catalog status = %d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Locale  string            `json:"locale"`
			Catalog map[string]string `json:"catalog"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp.Locale, resp.Catalog
	}

	// Explicit locale: merged catalog, later pack (Order 50) wins on "Delete".
	locale, cat := get("locale=ru")
	if locale != "ru" {
		t.Fatalf("locale = %q, want ru", locale)
	}
	if cat["Save"] != "Сохранить" || cat["Delete"] != "Удалять" {
		t.Fatalf("merged catalog = %v", cat)
	}

	// Fallback resolution: "ru-RU" resolves to the installed "ru" packs.
	locale, _ = get("locale=ru-RU")
	if locale != "ru" {
		t.Fatalf("fallback locale = %q, want ru", locale)
	}

	// Unknown locale yields empty catalog and empty resolution.
	locale, cat = get("locale=de")
	if locale != "" || len(cat) != 0 {
		t.Fatalf("unknown locale = %q catalog=%v", locale, cat)
	}
}

// TestImportLocalizationPack_AppliesImmediately: importing a language pack
// activates it right away — packs are install/delete only, no apply step.
func TestImportLocalizationPack_AppliesImmediately(t *testing.T) {
	h, store := newI18nTestHandler(t)
	pack := ruLanguagePack()
	pack.Applied = false
	body, _ := json.Marshal(pack)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/import", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ImportExtension(c); err != nil {
		t.Fatalf("ImportExtension: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("import status = %d body=%s", rec.Code, rec.Body.String())
	}
	saved, ok := store.Get("locale-ru")
	if !ok {
		t.Fatal("pack not stored after import")
	}
	if !saved.Applied {
		t.Fatal("imported language pack must be applied immediately")
	}

	// A non-localization extension must not be auto-applied on import.
	plain := Extension{ID: "plain-ext", Name: "Plain", BaseURL: "http://localhost"}
	body2, _ := json.Marshal(plain)
	req2 := httptest.NewRequest(http.MethodPost, "/sidecar/extensions/import", strings.NewReader(string(body2)))
	rec2 := httptest.NewRecorder()
	if err := h.ImportExtension(e.NewContext(req2, rec2)); err != nil {
		t.Fatalf("import plain: %v", err)
	}
	saved, _ = store.Get("plain-ext")
	if saved.Applied {
		t.Fatal("non-localization extension must not be auto-applied")
	}
}
