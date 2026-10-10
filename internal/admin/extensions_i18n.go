package admin

// Extension localization (i18n).
//
// Translatable surface of an extension manifest:
//
//	name, tagline, description, ui.help, ui.logo_text,
//	ui.fields[].label / ui.fields[].description,
//	ui.nav[].label, ui.banners[].message / link_label,
//	ui.widgets[].title / body, ui.pages[].title / summary,
//	ui.pages[].blocks[].text, ui.settings_tabs[].label / blocks[].text.
//
// Translations arrive from two places:
//
//  1. Embedded: the manifest itself carries i18n.locales.<code> — an
//     extension ships its own translations.
//  2. Packs: a type="localization" extension (a language pack) carries a
//     catalog keyed by the target extension id. Applied packs override the
//     embedded translation for their locale; several packs may be applied at
//     once (unlike themes, which are one-at-a-time).
//
// The active locale is a persisted store setting (GET/PUT
// /sidecar/extensions/locale, file configs/extension-locale.json by default).
// List endpoints accept ?locale=<code> as a per-request override.
// GetExtension / ExportExtension intentionally stay unlocalized: the JSON
// editor must always show the stored document verbatim.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
)

// ExtensionFieldTranslation localizes one ui.fields entry (keyed by field key).
type ExtensionFieldTranslation struct {
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// ExtensionBannerTranslation localizes one ui.banners entry (keyed by banner id).
type ExtensionBannerTranslation struct {
	Message   string `json:"message,omitempty"`
	LinkLabel string `json:"link_label,omitempty"`
}

// ExtensionWidgetTranslation localizes one ui.widgets entry (keyed by widget id).
type ExtensionWidgetTranslation struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// ExtensionBlockTranslation localizes one page or settings-tab block (map key
// = the block's decimal index in the source manifest). Text replaces
// heading/text/code block text; Items replaces a list block's items wholesale,
// so translators ship the whole list for that index.
type ExtensionBlockTranslation struct {
	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
}

// ExtensionPageTranslation localizes one ui.pages entry (keyed by page id).
// Blocks are keyed by their decimal index inside the page's blocks array, so
// inserting a block above them in the source manifest shifts the indexes and
// the stale translation simply stops matching instead of landing on the
// wrong paragraph.
type ExtensionPageTranslation struct {
	Title   string                               `json:"title,omitempty"`
	Summary string                               `json:"summary,omitempty"`
	Blocks  map[string]ExtensionBlockTranslation `json:"blocks,omitempty"`
}

// ExtensionSettingsTabTranslation localizes one ui.settings_tabs entry
// (keyed by tab id).
type ExtensionSettingsTabTranslation struct {
	Label  string                               `json:"label,omitempty"`
	Blocks map[string]ExtensionBlockTranslation `json:"blocks,omitempty"`
}

// ExtensionUITranslation carries the translatable strings of ExtensionUI.
// Map keys mirror the ids of the contributed entries (nav/banner/widget/page/
// settings-tab ids, field keys); missing keys fall back to the source text.
type ExtensionUITranslation struct {
	Help         string                                     `json:"help,omitempty"`
	LogoText     string                                     `json:"logo_text,omitempty"`
	Fields       map[string]ExtensionFieldTranslation       `json:"fields,omitempty"`
	Nav          map[string]string                          `json:"nav,omitempty"`
	Banners      map[string]ExtensionBannerTranslation      `json:"banners,omitempty"`
	Widgets      map[string]ExtensionWidgetTranslation      `json:"widgets,omitempty"`
	Pages        map[string]ExtensionPageTranslation        `json:"pages,omitempty"`
	SettingsTabs map[string]ExtensionSettingsTabTranslation `json:"settings_tabs,omitempty"`
}

// ExtensionTranslation is a full translation of one extension's translatable
// surface for a single locale. Empty strings mean "not translated here" —
// the source text (or a lower-precedence translation) stays.
type ExtensionTranslation struct {
	Name        string                  `json:"name,omitempty"`
	Tagline     string                  `json:"tagline,omitempty"`
	Description string                  `json:"description,omitempty"`
	UI          *ExtensionUITranslation `json:"ui,omitempty"`
}

// ExtensionI18n carries embedded per-locale translations inside a manifest:
// i18n.locales.<code> = ExtensionTranslation.
type ExtensionI18n struct {
	Locales map[string]ExtensionTranslation `json:"locales,omitempty"`
}

// normalizeLocale lowercases a locale tag and unifies its separator so
// "pt_BR" and "pt-br" resolve to the same code.
func normalizeLocale(code string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "_", "-"))
}

// validLocaleCode reports whether code looks like a BCP-47-ish locale tag:
// a 2–8 letter primary subtag plus optional "-subtag"s of 1–8 alphanumerics.
func validLocaleCode(code string) bool {
	if code == "" || len(code) > 35 {
		return false
	}
	parts := strings.Split(code, "-")
	primary := parts[0]
	if len(primary) < 2 || len(primary) > 8 {
		return false
	}
	for _, r := range primary {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	for _, sub := range parts[1:] {
		if len(sub) < 1 || len(sub) > 8 {
			return false
		}
		for _, r := range sub {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// localeBase returns the primary language subtag ("pt-BR" → "pt").
func localeBase(code string) string {
	return strings.SplitN(code, "-", 2)[0]
}

// pickLocale resolves want against the locales actually available for one
// extension: exact code first, then a base-language fallback. Returns "" when
// nothing usable is available.
func pickLocale(want string, available map[string]bool) string {
	want = normalizeLocale(want)
	if want == "" {
		return ""
	}
	if available[want] {
		return want
	}
	base := localeBase(want)
	var candidates []string
	for code := range available {
		if localeBase(code) == base {
			candidates = append(candidates, code)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	// A bare base-language pack ("pt") covers any region variant ("pt-BR").
	for _, c := range candidates {
		if c == base {
			return c
		}
	}
	// Simplified vs. traditional Chinese: a bare "zh" request prefers the
	// Simplified pack when several zh-* packs are installed.
	if base == "zh" {
		for _, c := range candidates {
			if c == "zh-cn" {
				return c
			}
		}
	}
	// Several region variants of the same language and no clear default —
	// stay with the source text rather than guessing.
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

// applyTranslation returns a copy of ext with tr's non-empty strings layered
// on top. The stored record's slices/maps are never mutated: translated
// slices are rebuilt so the extension store keeps the source text intact.
func applyTranslation(ext Extension, tr ExtensionTranslation) Extension {
	if tr.Name != "" {
		ext.Name = tr.Name
	}
	if tr.Tagline != "" {
		ext.Tagline = tr.Tagline
	}
	if tr.Description != "" {
		ext.Description = tr.Description
	}
	if tr.UI != nil {
		ext.UI = applyUITranslation(ext.UI, *tr.UI)
	}
	return ext
}

// applyUITranslation layers a UI translation over ui (copy-on-write).
func applyUITranslation(ui ExtensionUI, t ExtensionUITranslation) ExtensionUI {
	if t.Help != "" {
		ui.Help = t.Help
	}
	if t.LogoText != "" {
		ui.LogoText = t.LogoText
	}
	if len(t.Fields) > 0 {
		fields := make([]ExtensionField, len(ui.Fields))
		copy(fields, ui.Fields)
		for i := range fields {
			if tr, ok := t.Fields[fields[i].Key]; ok {
				if tr.Label != "" {
					fields[i].Label = tr.Label
				}
				if tr.Description != "" {
					fields[i].Description = tr.Description
				}
			}
		}
		ui.Fields = fields
	}
	if len(t.Nav) > 0 {
		entries := make([]ExtensionNavEntry, len(ui.Nav))
		copy(entries, ui.Nav)
		for i := range entries {
			if label := t.Nav[entries[i].ID]; label != "" {
				entries[i].Label = label
			}
		}
		ui.Nav = entries
	}
	if len(t.Banners) > 0 {
		banners := make([]ExtensionBanner, len(ui.Banners))
		copy(banners, ui.Banners)
		for i := range banners {
			if tr, ok := t.Banners[banners[i].ID]; ok {
				if tr.Message != "" {
					banners[i].Message = tr.Message
				}
				if tr.LinkLabel != "" {
					banners[i].LinkLabel = tr.LinkLabel
				}
			}
		}
		ui.Banners = banners
	}
	if len(t.Widgets) > 0 {
		widgets := make([]ExtensionWidget, len(ui.Widgets))
		copy(widgets, ui.Widgets)
		for i := range widgets {
			if tr, ok := t.Widgets[widgets[i].ID]; ok {
				if tr.Title != "" {
					widgets[i].Title = tr.Title
				}
				if tr.Body != "" {
					widgets[i].Body = tr.Body
				}
			}
		}
		ui.Widgets = widgets
	}
	if len(t.Pages) > 0 {
		pages := make([]ExtensionUIPage, len(ui.Pages))
		copy(pages, ui.Pages)
		for i := range pages {
			tr, ok := t.Pages[pages[i].ID]
			if !ok {
				continue
			}
			if tr.Title != "" {
				pages[i].Title = tr.Title
			}
			if tr.Summary != "" {
				pages[i].Summary = tr.Summary
			}
			blocks := make([]ExtensionUIBlock, len(pages[i].Blocks))
			copy(blocks, pages[i].Blocks)
			applyBlockTexts(blocks, tr.Blocks)
			pages[i].Blocks = blocks
		}
		ui.Pages = pages
	}
	if len(t.SettingsTabs) > 0 {
		tabs := make([]ExtensionSettingsTab, len(ui.SettingsTabs))
		copy(tabs, ui.SettingsTabs)
		for i := range tabs {
			tr, ok := t.SettingsTabs[tabs[i].ID]
			if !ok {
				continue
			}
			if tr.Label != "" {
				tabs[i].Label = tr.Label
			}
			blocks := make([]ExtensionUIBlock, len(tabs[i].Blocks))
			copy(blocks, tabs[i].Blocks)
			applyBlockTexts(blocks, tr.Blocks)
			tabs[i].Blocks = blocks
		}
		ui.SettingsTabs = tabs
	}
	return ui
}

// applyBlockTexts overlays translated block texts/items keyed by decimal
// index. The caller passes a freshly copied slice, so in-place replacement is
// safe; items are copied so the stored pack document is never shared.
func applyBlockTexts(blocks []ExtensionUIBlock, texts map[string]ExtensionBlockTranslation) {
	for i := range blocks {
		tr, ok := texts[fmt.Sprintf("%d", i)]
		if !ok {
			continue
		}
		if tr.Text != "" {
			blocks[i].Text = tr.Text
		}
		if len(tr.Items) > 0 {
			items := make([]string, len(tr.Items))
			copy(items, tr.Items)
			blocks[i].Items = items
		}
	}
}

// localizationPack is one applied type="localization" extension ready to
// overlay its catalog onto other extensions.
type localizationPack struct {
	id      string
	order   int
	locale  string
	catalog map[string]ExtensionTranslation
}

// extensionLocalizer resolves translations for one request locale. It
// snapshots the extension list once so a list endpoint localizes every row
// against a consistent view.
type extensionLocalizer struct {
	locale string
	packs  []localizationPack
	byID   map[string]Extension
}

// newLocalizer builds a localizer for the request locale, or nil when no
// locale applies (empty → source text stays).
func (h *Handler) newLocalizer(locale string) *extensionLocalizer {
	locale = normalizeLocale(locale)
	if locale == "" || h.extensions == nil {
		return nil
	}
	list := h.extensions.List()
	l := &extensionLocalizer{
		locale: locale,
		byID:   make(map[string]Extension, len(list)),
	}
	for _, e := range list {
		l.byID[e.ID] = e
		if e.Applied && e.Type == "localization" && len(e.Catalog) > 0 {
			l.packs = append(l.packs, localizationPack{
				id:      e.ID,
				order:   e.Order,
				locale:  normalizeLocale(e.Locale),
				catalog: e.Catalog,
			})
		}
	}
	// Deterministic overlay order: Order asc, then id asc — the last pack
	// wins when several packs translate the same string.
	sort.SliceStable(l.packs, func(i, j int) bool {
		if l.packs[i].order != l.packs[j].order {
			return l.packs[i].order < l.packs[j].order
		}
		return l.packs[i].id < l.packs[j].id
	})
	return l
}

// availableCodes gathers the locale codes that carry a translation for the
// given extension ids (embedded i18n plus applied pack locales).
func (l *extensionLocalizer) availableCodes(ids []string) map[string]bool {
	avail := make(map[string]bool)
	for _, id := range ids {
		if e, ok := l.byID[id]; ok && e.I18n != nil {
			for code := range e.I18n.Locales {
				avail[normalizeLocale(code)] = true
			}
		}
		for _, p := range l.packs {
			if p.locale == "" {
				continue
			}
			if _, ok := p.catalog[id]; ok {
				avail[p.locale] = true
			}
		}
	}
	return avail
}

// applyName layers embedded-then-pack name/tagline/description translations
// for one extension id over the current strings.
func (l *extensionLocalizer) applyName(id, code, name, tagline, description string) (string, string, string) {
	if e, ok := l.byID[id]; ok && e.I18n != nil {
		if tr, ok := e.I18n.Locales[code]; ok {
			if tr.Name != "" {
				name = tr.Name
			}
			if tr.Tagline != "" {
				tagline = tr.Tagline
			}
			if tr.Description != "" {
				description = tr.Description
			}
		}
	}
	for _, p := range l.packs {
		if p.locale != code {
			continue
		}
		if tr, ok := p.catalog[id]; ok {
			if tr.Name != "" {
				name = tr.Name
			}
			if tr.Tagline != "" {
				tagline = tr.Tagline
			}
			if tr.Description != "" {
				description = tr.Description
			}
		}
	}
	return name, tagline, description
}

// applyUI layers embedded-then-pack UI translations for one extension id
// over ui.
func (l *extensionLocalizer) applyUI(id, code string, ui ExtensionUI) ExtensionUI {
	if e, ok := l.byID[id]; ok && e.I18n != nil {
		if tr, ok := e.I18n.Locales[code]; ok && tr.UI != nil {
			ui = applyUITranslation(ui, *tr.UI)
		}
	}
	for _, p := range l.packs {
		if p.locale != code {
			continue
		}
		if tr, ok := p.catalog[id]; ok && tr.UI != nil {
			ui = applyUITranslation(ui, *tr.UI)
		}
	}
	return ui
}

// extension returns a localized copy of e.
func (l *extensionLocalizer) extension(e Extension) Extension {
	code := pickLocale(l.locale, l.availableCodes([]string{e.ID}))
	if code == "" {
		return e
	}
	// One resolve, then embedded and packs layered in precedence order
	// (packs override embedded).
	e.Name, e.Tagline, e.Description = l.applyName(e.ID, code, e.Name, e.Tagline, e.Description)
	e.UI = l.applyUI(e.ID, code, e.UI)
	return e
}

// contribution localizes a UI contribution. owner names the extension that
// shipped it (same as in.ID for manifest contributions; the owning extension
// id for addon-contributed UI), so a pack keyed by the extension id also
// translates its addons' tab and widget strings. Ids are applied owner-first
// so a more specific "addon:<name>" key wins over the owner-level translation.
// The contribution name only translates when the id is a known extension —
// an addon keeps its file-derived name.
func (l *extensionLocalizer) contribution(in ExtensionUIContribution, owner string) ExtensionUIContribution {
	ids := make([]string, 0, 2)
	if owner != "" && owner != in.ID {
		ids = append(ids, owner)
	}
	ids = append(ids, in.ID)
	code := pickLocale(l.locale, l.availableCodes(ids))
	if code == "" {
		return in
	}
	for _, id := range ids {
		if id == in.ID {
			if _, known := l.byID[id]; known {
				in.Name, _, _ = l.applyName(id, code, in.Name, "", "")
			}
		}
		in.UI = l.applyUI(id, code, in.UI)
	}
	return in
}

// ExtensionLocaleStore persists the operator-selected extension locale to
// configs/extension-locale.json (override path: AURORA_EXTENSION_LOCALE_PATH).
type ExtensionLocaleStore struct {
	mu     sync.RWMutex
	path   string
	locale string
}

// NewExtensionLocaleStore loads the persisted locale (empty = source text).
func NewExtensionLocaleStore() *ExtensionLocaleStore {
	s := &ExtensionLocaleStore{path: os.Getenv("AURORA_EXTENSION_LOCALE_PATH")}
	if s.path == "" {
		s.path = "configs/extension-locale.json"
	}
	s.load()
	return s
}

func (s *ExtensionLocaleStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var doc struct {
		Locale string `json:"locale"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return
	}
	locale := normalizeLocale(doc.Locale)
	if locale == "" || validLocaleCode(locale) {
		s.locale = locale
	}
}

func (s *ExtensionLocaleStore) saveLocked() {
	data, err := json.MarshalIndent(map[string]string{"locale": s.locale}, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = os.WriteFile(s.path, data, 0o644)
}

// Get returns the persisted locale ("" = source language).
func (s *ExtensionLocaleStore) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.locale
}

// Set persists the locale ("" resets to the source language) and returns it
// normalized.
func (s *ExtensionLocaleStore) Set(locale string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locale = normalizeLocale(locale)
	s.saveLocked()
	return s.locale
}

// extensionLocale returns the handler's locale store, creating it on first
// use so handlers work without an explicit option (same pattern as
// extensionStores).
func (h *Handler) extensionLocale() *ExtensionLocaleStore {
	h.extensionLocaleOnce.Do(func() {
		if h.extensionLocaleStore == nil {
			h.extensionLocaleStore = NewExtensionLocaleStore()
		}
	})
	return h.extensionLocaleStore
}

// WithExtensionLocaleStore sets the extension locale store on the handler.
func WithExtensionLocaleStore(store *ExtensionLocaleStore) Option {
	return func(h *Handler) {
		h.extensionLocaleStore = store
	}
}

// requestedLocale resolves the locale for one request: a valid ?locale= query
// override wins, otherwise the persisted operator selection.
func (h *Handler) requestedLocale(c *echo.Context) string {
	if q := strings.TrimSpace(c.QueryParam("locale")); q != "" {
		if norm := normalizeLocale(q); validLocaleCode(norm) {
			return norm
		}
	}
	return h.extensionLocale().Get()
}

// availableExtensionLocales lists every locale code any installed extension
// or applied language pack can provide (plus the active locale so the
// selector never renders blank).
func (h *Handler) availableExtensionLocales(active string) []string {
	seen := make(map[string]bool)
	if active != "" {
		seen[active] = true
	}
	if h.extensions != nil {
		for _, e := range h.extensions.List() {
			if e.I18n != nil {
				for code := range e.I18n.Locales {
					if norm := normalizeLocale(code); validLocaleCode(norm) {
						seen[norm] = true
					}
				}
			}
			if e.Applied && e.Type == "localization" && e.Locale != "" {
				if norm := normalizeLocale(e.Locale); validLocaleCode(norm) {
					seen[norm] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for code := range seen {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// GetExtensionLocale returns the active extension locale and every locale
// available from installed extensions / applied language packs.
// GET /sidecar/extensions/locale
func (h *Handler) GetExtensionLocale(c *echo.Context) error {
	locale := h.extensionLocale().Get()
	return c.JSON(http.StatusOK, map[string]any{
		"locale":    locale,
		"available": h.availableExtensionLocales(locale),
	})
}

// SetExtensionLocale persists the active extension locale ("" resets to the
// source language). PUT /sidecar/extensions/locale {"locale":"ru"}
func (h *Handler) SetExtensionLocale(c *echo.Context) error {
	var req struct {
		Locale string `json:"locale"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	locale := normalizeLocale(req.Locale)
	if locale != "" && !validLocaleCode(locale) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("locale: %q is not a valid locale code (e.g. \"ru\", \"pt-BR\")", req.Locale),
		})
	}
	saved := h.extensionLocale().Set(locale)
	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"locale":    saved,
		"available": h.availableExtensionLocales(saved),
	})
}

// validateLocalization checks a type="localization" manifest: it must pin a
// locale and carry a non-empty catalog of target-extension translations.
func (p *Extension) validateLocalization(add func(format string, args ...any)) {
	if !validLocaleCode(normalizeLocale(p.Locale)) {
		add("locale: required for type %q (the code its catalog is written in, e.g. \"ru\" or \"pt-BR\")", p.Type)
	}
	if len(p.Catalog) == 0 {
		add("catalog: required for type %q (map of target extension id → translation)", p.Type)
	}
	for id, tr := range p.Catalog {
		if strings.TrimSpace(id) == "" {
			add("catalog: empty extension id")
			continue
		}
		if tr.Name == "" && tr.Tagline == "" && tr.Description == "" && tr.UI == nil {
			add("catalog[%q]: empty translation (set name/tagline/description or ui)", id)
		}
	}
}

// validateI18n checks embedded i18n locale codes and shapes.
func (p *Extension) validateI18n(add func(format string, args ...any)) {
	if p.I18n == nil {
		return
	}
	for code, tr := range p.I18n.Locales {
		norm := normalizeLocale(code)
		if !validLocaleCode(norm) {
			add("i18n.locales[%q]: invalid locale code (use e.g. \"en\", \"ru\", \"pt-BR\")", code)
		}
		if tr.Name == "" && tr.Tagline == "" && tr.Description == "" && tr.UI == nil {
			add("i18n.locales[%q]: empty translation", code)
		}
	}
}
