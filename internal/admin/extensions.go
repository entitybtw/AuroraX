package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
)

// ExtensionHeader is one header transformation an extension installs into the
// Session Hub when applied. It mirrors the sessionhub HeaderRule shape.
type ExtensionHeader struct {
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	Prefix  string   `json:"prefix,omitempty"`
	Length  int      `json:"length,omitempty"`
	Charset string   `json:"charset,omitempty"`
	Value   string   `json:"value,omitempty"`
	Values  []string `json:"values,omitempty"`
}

// ExtensionField is a UI hint describing one configurable value of an
// extension. Extensions declare arbitrary fields so third-party extensions
// render coherently without gateway changes.
type ExtensionField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"` // text | number | boolean | select | color
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

// ExtensionNavEntry adds a sidebar link for an extension-provided page
// (or an external URL when To is absolute http/https).
type ExtensionNavEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	To    string `json:"to"` // path under /admin/dashboard, or absolute URL
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order,omitempty"`
}

// ExtensionBanner is a dismissible-looking strip shown above the page outlet.
type ExtensionBanner struct {
	ID        string `json:"id"`
	Level     string `json:"level,omitempty"` // info | warning | error | success
	Message   string `json:"message"`
	LinkLabel string `json:"link_label,omitempty"`
	LinkURL   string `json:"link_url,omitempty"`
	Order     int    `json:"order,omitempty"`
}

// ExtensionWidget is a card injected into a named layout slot.
type ExtensionWidget struct {
	ID    string `json:"id"`
	Slot  string `json:"slot,omitempty"` // overview | settings (default overview)
	Title string `json:"title,omitempty"`
	Kind  string `json:"kind,omitempty"` // text | stats | links (default text)
	Body  string `json:"body,omitempty"`
	// Stats are key/value pairs for kind=stats.
	Stats []map[string]string `json:"stats,omitempty"`
	// Links are {label,href} pairs for kind=links.
	Links []ExtensionNavLink `json:"links,omitempty"`
	Order int                `json:"order,omitempty"`
}

// ExtensionNavLink is a labeled link used by widgets/pages.
type ExtensionNavLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// ExtensionUIBlock is one structured content block on an extension page.
// No raw HTML/JS is executed — only these shapes render.
type ExtensionUIBlock struct {
	Kind     string             `json:"kind"` // heading | text | code | list | links | divider | kv
	Text     string             `json:"text,omitempty"`
	Language string             `json:"language,omitempty"`
	Items    []string           `json:"items,omitempty"`
	Links    []ExtensionNavLink `json:"links,omitempty"`
	// KV is for kind=kv: list of {k,v} or {label,value}.
	KV []map[string]string `json:"kv,omitempty"`
}

// ExtensionUIPage is a full dashboard page contributed by an extension.
type ExtensionUIPage struct {
	ID      string             `json:"id"`
	Path    string             `json:"path"` // slug under /admin/dashboard/ext/
	Title   string             `json:"title"`
	Icon    string             `json:"icon,omitempty"`
	Order   int                `json:"order,omitempty"`
	Summary string             `json:"summary,omitempty"`
	Blocks  []ExtensionUIBlock `json:"blocks,omitempty"`
}

// ExtensionSettingsTab adds a tab under Settings that renders blocks/widgets.
type ExtensionSettingsTab struct {
	ID     string             `json:"id"`
	Label  string             `json:"label"`
	Icon   string             `json:"icon,omitempty"`
	Order  int                `json:"order,omitempty"`
	Blocks []ExtensionUIBlock `json:"blocks,omitempty"`
}

// ExtensionUI carries presentation + interface contributions for an extension.
// The dashboard applies these only after the extension is applied (Applied=true).
type ExtensionUI struct {
	Accent  string           `json:"accent,omitempty"`
	DocsURL string           `json:"docs_url,omitempty"`
	Help    string           `json:"help,omitempty"`
	Fields  []ExtensionField `json:"fields,omitempty"`
	// Theme maps restricted CSS custom properties (e.g. --accent, --radius).
	Theme map[string]string `json:"theme,omitempty"`
	// Nav adds sidebar entries (usually pointing at ui.pages paths).
	Nav []ExtensionNavEntry `json:"nav,omitempty"`
	// HideNav hides built-in sidebar labels or path suffixes (case-insensitive).
	HideNav []string `json:"hide_nav,omitempty"`
	// Banners render above the main content outlet.
	Banners []ExtensionBanner `json:"banners,omitempty"`
	// Widgets inject cards into layout slots (overview/settings).
	Widgets []ExtensionWidget `json:"widgets,omitempty"`
	// Pages add full routes under /admin/dashboard/ext/{path}.
	Pages []ExtensionUIPage `json:"pages,omitempty"`
	// SettingsTabs add tabs under Settings (render blocks + settings widgets).
	SettingsTabs []ExtensionSettingsTab `json:"settings_tabs,omitempty"`
	// HideSettingsTabs hides built-in settings tab ids.
	HideSettingsTabs []string `json:"hide_settings_tabs,omitempty"`
	// LogoText overrides the sidebar / mobile brand text when set.
	LogoText string `json:"logo_text,omitempty"`
	// LogoURL overrides the brand mark (absolute or root-relative image URL).
	LogoURL string `json:"logo_url,omitempty"`
}

// ExtensionTool describes a tool schema an extension wants injected upstream.
// Either InlineJSON (a raw JSON array of tools) or Source references one of the
// gateway's bundled schemas. Unknown sources are ignored at request time, which
// lets extensions ship ahead of gateway support.
type ExtensionTool struct {
	Name       string          `json:"name"`
	Source     string          `json:"source,omitempty"`
	InlineJSON json.RawMessage `json:"inline,omitempty"`
}

// ExtensionOAuth carries device-flow wiring an extension supplies so the
// gateway and dashboard do not hardcode provider-specific OAuth endpoints.
type ExtensionOAuth struct {
	// Server is the OAuth authorization server base URL (device authorization).
	Server string `json:"server,omitempty"`
	// ClientID is the public client_id used in the device flow.
	ClientID string `json:"client_id,omitempty"`
	// VerificationBase is the origin used to expand relative verification URIs
	// (e.g. "https://example.com"). Empty = treat verification URIs as absolute.
	VerificationBase string `json:"verification_base,omitempty"`
	// UserAgent is an optional UA for OAuth HTTP calls.
	UserAgent string `json:"user_agent,omitempty"`
	// Scope is an optional OAuth scope string.
	Scope string `json:"scope,omitempty"`
}

// ExtensionProvides declares optional capabilities that are not built into the
// gateway by default and are activated when the extension is installed —
// for example the "opencode" provider type or OAuth device-flow wiring.
type ExtensionProvides struct {
	// ProviderTypes are factory provider types this extension activates
	// (e.g. ["opencode"]). Types absent from this list stay unregistered.
	ProviderTypes []string `json:"provider_types,omitempty"`
	// Features are optional capability flags surfaced to the operator
	// (e.g. ["oauth"]).
	Features []string `json:"features,omitempty"`
}

// Extension is a portable, data-driven bundle that configures the sidecar and
// Session Hub headers (and optional provider types) together. The gateway ships
// with no built-in extensions: they are imported from JSON, a URL, or a store.
type Extension struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Tagline      string            `json:"tagline,omitempty"`
	Description  string            `json:"description,omitempty"`
	Version      string            `json:"version,omitempty"`
	Author       string            `json:"author,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Type         string            `json:"type,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Schema       int               `json:"schema,omitempty"`
	BaseURL      string            `json:"base_url,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	DefaultAuth  string            `json:"default_auth,omitempty"`
	InjectTools  *bool             `json:"inject_tools,omitempty"`
	InjectTypes  []string          `json:"inject_tool_types,omitempty"`
	MaxAttempts  int               `json:"max_attempts,omitempty"`
	RetryDelayMs int               `json:"retry_delay_ms,omitempty"`
	Headers      []ExtensionHeader `json:"headers,omitempty"`
	// Tools lets an extension request specific upstream tool schemas.
	Tools []ExtensionTool `json:"tool_schemas,omitempty"`
	// Requirements and Notes are surfaced verbatim in the UI.
	Requirements []string `json:"requirements,omitempty"`
	Notes        []string `json:"notes,omitempty"`
	// Settings is a free-form map for extension-specific tunables that map
	// onto sidecar settings by key.
	Settings map[string]string `json:"settings,omitempty"`
	// OAuth supplies device-flow endpoints for this extension's providers.
	OAuth *ExtensionOAuth `json:"oauth,omitempty"`
	// Files is a map of relative path → file content materialised under
	// configs/extensions/<id>/ on apply (tool schemas, helper scripts, …).
	// Paths must stay under the extension directory (no ".." / absolute).
	Files    map[string]string  `json:"files,omitempty"`
	Provides *ExtensionProvides `json:"provides,omitempty"`
	UI       ExtensionUI        `json:"ui,omitempty"`
	// Config holds user-saved overrides: values for ui.fields keys and
	// optional theme var overrides (CSS custom properties starting with "--").
	// It is persisted with the extension and merged into the UI theme on
	// ListExtensionUI.
	Config map[string]string `json:"config,omitempty"`
	// Applied marks that the operator activated this extension (UI + sidecar).
	Applied bool   `json:"applied,omitempty"`
	Builtin bool   `json:"builtin"`
	Source  string `json:"source,omitempty"`
}

// ExtensionStore holds imported extensions, persisting them to disk so they
// survive restarts. The gateway never ships built-in extensions.
type ExtensionStore struct {
	mu        sync.RWMutex
	path      string
	imported  []Extension
	activator func(provides *ExtensionProvides) []string
}

// NewExtensionStore loads imported extensions from the config directory.
// Legacy AURORA_SIDECAR_PRESETS_PATH / configs/sidecar-presets.json files are
// migrated automatically so existing installs keep their imports.
func NewExtensionStore() *ExtensionStore {
	s := &ExtensionStore{path: os.Getenv("AURORA_EXTENSIONS_PATH")}
	if s.path == "" {
		s.path = "configs/extensions.json"
	}
	s.load()
	s.migrateLegacy()
	return s
}

// SetProviderTypeActivator installs a callback invoked on import/apply so
// optional provider types (declared under provides.provider_types) can be
// registered with the factory. It returns which types were activated.
func (s *ExtensionStore) SetProviderTypeActivator(fn func(provides *ExtensionProvides) []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activator = fn
}

func (s *ExtensionStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var loaded []Extension
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	for i := range loaded {
		loaded[i].Builtin = false
	}
	s.imported = loaded
}

// migrateLegacy imports the pre-extensions preset file when the new path is empty.
func (s *ExtensionStore) migrateLegacy() {
	s.mu.Lock()
	have := len(s.imported)
	s.mu.Unlock()
	if have > 0 {
		return
	}
	for _, env := range []string{"AURORA_SIDECAR_PRESETS_PATH", "AURORA_SIDECAR_EXTENSIONS_PATH"} {
		if p := os.Getenv(env); p != "" {
			if s.tryImportLegacyFile(p) {
				return
			}
		}
	}
	if s.tryImportLegacyFile("configs/sidecar-presets.json") {
		return
	}
	s.tryImportLegacyFile("configs/sidecar-extensions.json")
}

func (s *ExtensionStore) tryImportLegacyFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var loaded []Extension
	if err := json.Unmarshal(data, &loaded); err != nil || len(loaded) == 0 {
		return false
	}
	s.mu.Lock()
	s.imported = append(s.imported, loaded...)
	s.saveLocked()
	s.mu.Unlock()
	return true
}

func (s *ExtensionStore) saveLocked() {
	data, err := json.MarshalIndent(s.imported, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = os.WriteFile(s.path, data, 0o644)
}

// List returns imported extensions (built-in list is always empty).
func (s *ExtensionStore) List() []Extension {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Extension, len(s.imported))
	copy(out, s.imported)
	return out
}

// Get returns an extension by id.
func (s *ExtensionStore) Get(id string) (Extension, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.imported {
		if e.ID == id {
			return e, true
		}
	}
	return Extension{}, false
}

// Upsert stores (or replaces) an imported extension and activates any
// optional provider types it provides.
func (s *ExtensionStore) Upsert(ext Extension) []string {
	s.mu.Lock()
	ext.Builtin = false
	replaced := false
	for i, e := range s.imported {
		if e.ID == ext.ID {
			s.imported[i] = ext
			replaced = true
			break
		}
	}
	if !replaced {
		s.imported = append(s.imported, ext)
	}
	s.saveLocked()
	activator := s.activator
	s.mu.Unlock()

	if activator != nil && ext.Provides != nil {
		return activator(ext.Provides)
	}
	return nil
}

// Delete removes an imported extension.
func (s *ExtensionStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.imported {
		if e.ID == id {
			s.imported = append(s.imported[:i], s.imported[i+1:]...)
			s.saveLocked()
			return true
		}
	}
	return false
}

// ActivateOptionalTypes runs the activator for every stored extension that
// declares provides.provider_types (startup path). Returns activated type names.
func (s *ExtensionStore) ActivateOptionalTypes() []string {
	s.mu.RLock()
	activator := s.activator
	var list []Extension
	for _, e := range s.imported {
		if e.Provides != nil && (len(e.Provides.ProviderTypes) > 0 || len(e.Provides.Features) > 0) {
			list = append(list, e)
		}
	}
	s.mu.RUnlock()
	if activator == nil {
		return nil
	}
	var activated []string
	seen := map[string]bool{}
	for _, e := range list {
		for _, t := range activator(e.Provides) {
			if !seen[t] {
				seen[t] = true
				activated = append(activated, t)
			}
		}
	}
	return activated
}

// Validate normalizes and checks an extension.
func (p *Extension) Validate() error {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" {
		return fmt.Errorf("extension id is required")
	}
	if strings.ContainsAny(p.ID, " /\\") {
		return fmt.Errorf("extension id must not contain spaces or slashes")
	}
	if p.Type == "" {
		// Default to sidecar; explicit types such as "theme" are preserved.
		p.Type = "sidecar"
	}
	// "opencode" is a normal extension id (the store seed uses it). Provider
	// types live in a separate namespace (provides.provider_types).
	if p.Name == "" {
		p.Name = p.ID
	}
	if p.DefaultAuth == "" {
		p.DefaultAuth = "Bearer public"
	}
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 4
	}
	if p.RetryDelayMs <= 0 {
		p.RetryDelayMs = 750
	}
	if p.Files != nil {
		for rel := range p.Files {
			if !safeExtRelPath(rel) {
				return fmt.Errorf("files path %q is not allowed (must be relative and stay under the extension dir)", rel)
			}
		}
	}
	return nil
}

// safeExtRelPath reports whether rel is a safe relative path for extension files.
func safeExtRelPath(rel string) bool {
	rel = strings.TrimSpace(rel)
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") {
		return false
	}
	if strings.Contains(rel, "..") {
		return false
	}
	return !strings.ContainsAny(rel, ":")
}

// EffectiveTags returns the stored tags unioned with auto-detected capability
// tags derived from the extension's shape. List/Get responses surface this
// derived list so the dashboard's tag filters see capabilities (theme,
// sessionhub, sidecar, ui, oauth, providers) without the tags being persisted.
func (e Extension) EffectiveTags() []string {
	seen := make(map[string]bool, len(e.Tags)+6)
	out := make([]string, 0, len(e.Tags)+6)
	add := func(t string) {
		if t == "" {
			return
		}
		key := strings.ToLower(t)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, t)
	}
	for _, t := range e.Tags {
		add(t)
	}

	hasThemeTag := false
	for _, t := range e.Tags {
		if strings.EqualFold(strings.TrimSpace(t), "theme") {
			hasThemeTag = true
			break
		}
	}
	hasColorField := false
	for _, f := range e.UI.Fields {
		if strings.EqualFold(f.Type, "color") {
			hasColorField = true
			break
		}
	}
	if e.Type == "theme" || hasThemeTag || len(e.UI.Theme) > 0 || hasColorField {
		add("theme")
	}
	if len(e.Headers) > 0 {
		add("sessionhub")
	}
	if e.BaseURL != "" || e.InjectTools != nil || e.Type == "sidecar" {
		add("sidecar")
	}
	if e.UI.Accent != "" || len(e.UI.Nav) > 0 || len(e.UI.Pages) > 0 ||
		len(e.UI.Banners) > 0 || len(e.UI.Widgets) > 0 || len(e.UI.SettingsTabs) > 0 {
		add("ui")
	}
	if e.OAuth != nil || extensionProvidesFeature(e.Provides, "oauth") {
		add("oauth")
	}
	if e.Provides != nil && len(e.Provides.ProviderTypes) > 0 {
		add("providers")
	}
	return out
}

// MaterializeFiles writes ext.Files under dir (creating subdirs). Returns the
// absolute path of the first *.json tool schema file suitable as a sidecar
// tools path, or "" when none was written.
func (p *Extension) MaterializeFiles(dir string) (toolsPath string, err error) {
	if len(p.Files) == 0 {
		return "", nil
	}
	for rel, content := range p.Files {
		if !safeExtRelPath(rel) {
			return "", fmt.Errorf("unsafe files path %q", rel)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			return "", err
		}
		if toolsPath == "" && strings.HasSuffix(strings.ToLower(rel), ".json") &&
			(strings.Contains(strings.ToLower(rel), "tool") || strings.Contains(strings.ToLower(rel), "schema")) {
			toolsPath = dst
		}
	}
	// Prefer an explicit tools file name when several JSON files exist.
	if toolsPath == "" {
		for rel := range p.Files {
			if strings.HasSuffix(strings.ToLower(rel), ".json") {
				toolsPath = filepath.Join(dir, filepath.FromSlash(rel))
				break
			}
		}
	}
	return toolsPath, nil
}

// ExtensionFilesDir returns the on-disk directory for an extension's files.
func ExtensionFilesDir(id string) string {
	base := strings.TrimSpace(os.Getenv("AURORA_EXTENSIONS_FILES_DIR"))
	if base == "" {
		base = "configs/extensions"
	}
	return filepath.Join(base, id)
}

// WithExtensionStore sets the extension store on the handler.
func WithExtensionStore(store *ExtensionStore) Option {
	return func(h *Handler) {
		h.extensions = store
	}
}

// ListExtensions returns all imported extensions. Tags on each response copy
// are replaced with EffectiveTags() so the dashboard sees auto-detected
// capability tags without them being persisted.
func (h *Handler) ListExtensions(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusOK, map[string]any{"extensions": []any{}, "presets": []any{}})
	}
	list := h.extensions.List()
	for i := range list {
		list[i].Tags = list[i].EffectiveTags()
	}
	// "presets" is a temporary alias for older dashboard builds.
	return c.JSON(http.StatusOK, map[string]any{"extensions": list, "presets": list})
}

// GetExtension returns a single extension by id. Tags on the response copy
// are replaced with EffectiveTags() (see ListExtensions).
func (h *Handler) GetExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	ext.Tags = ext.EffectiveTags()
	return c.JSON(http.StatusOK, ext)
}

// ExportExtension returns an extension as a downloadable JSON body.
func (h *Handler) ExportExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	ext.Builtin = false
	data, err := json.MarshalIndent(ext, "", "  ")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", ext.ID+".extension.json"))
	return c.JSONBlob(http.StatusOK, data)
}

// ImportExtension imports an extension from a JSON body, or from a URL when
// the body contains {"url": "..."}.
func (h *Handler) ImportExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
	}

	raw := strings.TrimSpace(string(body))
	var envelope struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.URL != "" {
		fetched, ferr := fetchExtensionFromURL(envelope.URL)
		if ferr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": ferr.Error()})
		}
		raw = fetched
	}

	var ext Extension
	if err := json.Unmarshal([]byte(raw), &ext); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid extension JSON: " + err.Error()})
	}
	if err := ext.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	activated := h.extensions.Upsert(ext)
	saved, _ := h.extensions.Get(ext.ID)
	return c.JSON(http.StatusOK, map[string]any{
		"status":                   "ok",
		"extension":                saved,
		"preset":                   saved,
		"activated_provider_types": activated,
	})
}

// DeleteExtension removes an imported extension.
func (h *Handler) DeleteExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	if !h.extensions.Delete(c.Param("id")) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// UnapplyExtension deactivates an extension (Applied=false). Sidecar settings
// and session hub headers installed by a previous apply are left untouched;
// the operator re-applies another extension or edits settings to replace them.
func (h *Handler) UnapplyExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	ext.Applied = false
	h.extensions.Upsert(ext)
	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"extension": ext.ID,
		"applied":   false,
	})
}

// UpdateExtensionConfig merges user-saved overrides into an extension's
// Config map. Keys must match a ui.fields key or be a CSS custom property
// starting with "--".
func (h *Handler) UpdateExtensionConfig(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	var req struct {
		Config map[string]string `json:"config"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	fieldKeys := make(map[string]bool, len(ext.UI.Fields))
	for _, f := range ext.UI.Fields {
		fieldKeys[f.Key] = true
	}
	for key := range req.Config {
		if strings.HasPrefix(key, "--") {
			continue
		}
		if !fieldKeys[key] {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid config key %q: must match a ui.fields key or start with --", key),
			})
		}
	}
	if ext.Config == nil {
		ext.Config = make(map[string]string, len(req.Config))
	}
	for key, value := range req.Config {
		ext.Config[key] = value
	}
	h.extensions.Upsert(ext)
	saved, _ := h.extensions.Get(ext.ID)
	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"extension": saved,
	})
}

// applyFailure carries an HTTP status + message out of buildApplyResponse.
type applyFailure struct {
	status int
	msg    string
}

// ApplyExtension applies an extension's sidecar settings and returns the header
// rules the caller should install into the Session Hub via
// /sessionhub/providers/:name/ensure-headers. Optional provider types declared
// under provides.provider_types are activated on apply.
func (h *Handler) ApplyExtension(c *echo.Context) error {
	resp, _, fail := h.buildApplyResponse(c)
	if fail != nil {
		return c.JSON(fail.status, map[string]string{"error": fail.msg})
	}
	return c.JSON(http.StatusOK, resp)
}

// FullApplyExtension runs the full apply pipeline (identical to
// ApplyExtension) and, when a session_hub_target is supplied and the
// extension declares header rules, also installs those rules into the
// Session Hub target (provider or pool) server-side. The response is the
// apply payload plus a "session_hub" result when headers were considered.
// If the Session Hub is not wired into the handler, session_hub reports
// installed=false and the headers stay in the response so the frontend can
// chain POST /sessionhub/providers/:name/ensure-headers itself.
func (h *Handler) FullApplyExtension(c *echo.Context) error {
	var req struct {
		SessionHubTarget string `json:"session_hub_target"`
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
	}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
	}
	resp, ext, fail := h.buildApplyResponse(c)
	if fail != nil {
		return c.JSON(fail.status, map[string]string{"error": fail.msg})
	}
	target := strings.TrimSpace(req.SessionHubTarget)
	if target != "" && len(ext.Headers) > 0 {
		resp["session_hub"] = h.ensureSessionHubHeaders(target, ext.Headers)
	}
	return c.JSON(http.StatusOK, resp)
}

// ensureSessionHubHeaders installs header rules into the session hub target
// through the wired SessionHeaderEnsurer. It never fails the overall apply:
// on error or when no ensurer is wired it reports installed=false so the
// caller can fall back to chaining ensure-headers from the frontend.
func (h *Handler) ensureSessionHubHeaders(target string, headers []ExtensionHeader) map[string]any {
	if h.sessionHeaderEnsurer == nil {
		return map[string]any{
			"installed": false,
			"target":    target,
			"reason":    "session hub unavailable",
		}
	}
	result, err := h.sessionHeaderEnsurer.EnsureHeaders(target, headers)
	if err != nil {
		return map[string]any{
			"installed": false,
			"target":    target,
			"error":     err.Error(),
		}
	}
	out := map[string]any{}
	for key, value := range result {
		out[key] = value
	}
	out["installed"] = true
	out["target"] = target
	return out
}

// buildApplyResponse runs the shared apply pipeline and returns the response
// payload. On failure it returns a non-nil *applyFailure describing the HTTP
// error the caller should render.
func (h *Handler) buildApplyResponse(c *echo.Context) (map[string]any, Extension, *applyFailure) {
	if h.extensions == nil {
		return nil, Extension{}, &applyFailure{http.StatusNotFound, "extensions unavailable"}
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return nil, Extension{}, &applyFailure{http.StatusNotFound, "extension not found"}
	}
	if h.sidecarStore == nil {
		return nil, Extension{}, &applyFailure{http.StatusServiceUnavailable, "sidecar store unavailable"}
	}

	// Merge into current sidecar settings; values the extension does not
	// mention are left untouched.
	current := h.sidecarStore.get()
	next := current
	next.Enabled = true
	if ext.BaseURL != "" {
		next.BaseURL = ext.BaseURL
	}
	if ext.UserAgent != "" {
		next.UserAgent = ext.UserAgent
	}
	if ext.DefaultAuth != "" {
		next.DefaultAuth = ext.DefaultAuth
	}
	if ext.InjectTools != nil {
		next.InjectTools = *ext.InjectTools
	}
	if ext.InjectTypes != nil {
		next.InjectToolTypes = ext.InjectTypes
	}
	if ext.MaxAttempts > 0 {
		next.MaxAttempts = ext.MaxAttempts
	}
	if ext.RetryDelayMs > 0 {
		next.RetryDelayMs = ext.RetryDelayMs
	}
	// Free-form extension settings keyed onto known sidecar knobs.
	for k, v := range ext.Settings {
		switch strings.ToLower(k) {
		case "base_url":
			next.BaseURL = v
		case "user_agent":
			next.UserAgent = v
		case "default_auth":
			next.DefaultAuth = v
		case "tools_path", "tool_schemas_path":
			next.ToolsPath = v
		case "oauth_server":
			next.OAuthServer = v
		case "oauth_client_id":
			next.OAuthClientID = v
		case "oauth_verification_base":
			next.OAuthVerificationBase = v
		}
	}
	// Materialise extension payload files (tool schemas, scripts, …).
	if len(ext.Files) > 0 {
		toolsPath, ferr := ext.MaterializeFiles(ExtensionFilesDir(ext.ID))
		if ferr != nil {
			return nil, Extension{}, &applyFailure{http.StatusBadRequest, "materialize files: " + ferr.Error()}
		}
		if toolsPath != "" {
			next.ToolsPath = toolsPath
		}
	}
	// Extension OAuth becomes the sidecar/global defaults for device flow.
	if ext.OAuth != nil {
		if ext.OAuth.Server != "" {
			next.OAuthServer = ext.OAuth.Server
		}
		if ext.OAuth.ClientID != "" {
			next.OAuthClientID = ext.OAuth.ClientID
		}
		if ext.OAuth.VerificationBase != "" {
			next.OAuthVerificationBase = ext.OAuth.VerificationBase
		}
	}
	h.sidecarStore.update(next)

	var activated []string
	if h.extensions != nil {
		ext.Applied = true
		activated = h.extensions.Upsert(ext)
	}

	// provides.features "oauth" wires device-flow OAuth onto matching providers.
	oauthProviders := h.applyExtensionOAuth(c, ext)

	tools := make([]string, 0, len(ext.Tools))
	for _, t := range ext.Tools {
		if t.Source != "" {
			tools = append(tools, t.Source)
		} else if t.Name != "" {
			tools = append(tools, t.Name)
		}
	}

	resp := map[string]any{
		"status":                   "ok",
		"extension":                ext.ID,
		"preset":                   ext.ID,
		"headers":                  ext.Headers,
		"tools":                    tools,
		"activated_provider_types": activated,
		"provides":                 ext.Provides,
	}
	if ext.OAuth != nil {
		resp["oauth"] = ext.OAuth
	}
	if len(oauthProviders) > 0 {
		resp["oauth_providers"] = oauthProviders
	}
	if next.ToolsPath != "" {
		resp["tools_path"] = next.ToolsPath
	}
	return resp, ext, nil
}

// extensionProvidesFeature reports whether provides.features lists name
// (case-insensitive).
func extensionProvidesFeature(p *ExtensionProvides, name string) bool {
	if p == nil {
		return false
	}
	for _, f := range p.Features {
		if strings.EqualFold(strings.TrimSpace(f), name) {
			return true
		}
	}
	return false
}

// applyExtensionOAuth enables auth_method=oauth on providers that this
// extension targets: type ∈ provides.provider_types, or base_url equals the
// extension base_url (e.g. free tier free-tier vllm instances). OAuth server /
// client_id come from extension.oauth. Returns updated provider names.
// No-op when the extension does not declare provides.features "oauth".
func (h *Handler) applyExtensionOAuth(c *echo.Context, ext Extension) []string {
	if h.providerOverrides == nil || ext.OAuth == nil ||
		!extensionProvidesFeature(ext.Provides, "oauth") {
		return nil
	}
	server := strings.TrimSpace(ext.OAuth.Server)
	clientID := strings.TrimSpace(ext.OAuth.ClientID)
	if server == "" || clientID == "" {
		return nil
	}

	types := map[string]bool{}
	if ext.Provides != nil {
		for _, t := range ext.Provides.ProviderTypes {
			t = strings.ToLower(strings.TrimSpace(t))
			if t != "" {
				types[t] = true
			}
		}
	}
	extBase := strings.TrimRight(strings.TrimSpace(ext.BaseURL), "/")

	var updated []string
	for _, o := range h.providerOverrides.list() {
		if !o.IsEnabled() {
			continue
		}
		match := types[strings.ToLower(strings.TrimSpace(o.Type))]
		if !match && extBase != "" {
			base := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
			match = base == extBase
		}
		if !match {
			continue
		}
		if o.AuthMethod == "oauth" && o.OAuthServer == server && o.OAuthClientID == clientID {
			continue
		}
		o.AuthMethod = "oauth"
		o.OAuthServer = server
		o.OAuthClientID = clientID
		h.providerOverrides.upsert(o)
		updated = append(updated, o.Name)
	}
	if len(updated) > 0 && h.runtimeRefresher != nil {
		if _, err := h.runtimeRefresher.RefreshRuntime(c.Request().Context()); err != nil {
			// Surface refresh failure without failing apply — overrides persist.
			return updated
		}
	}
	return updated
}

// ExtensionUIContribution is one applied extension's UI payload for the dashboard.
type ExtensionUIContribution struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	UI   ExtensionUI `json:"ui"`
}

// extensionUIWithConfig returns e.UI with user Config overrides merged into
// the theme map (the UI copy's Theme is replaced, never the stored map).
// Config keys starting with "--" are CSS custom properties and are set
// directly. Config keys matching a ui.fields key of type color map onto the
// theme only when the key is "accent" (→ "--accent"); other color field keys
// without a "--" prefix are skipped.
func extensionUIWithConfig(e Extension) ExtensionUI {
	if len(e.Config) == 0 {
		return e.UI
	}
	colorKeys := make(map[string]bool)
	for _, f := range e.UI.Fields {
		if strings.EqualFold(f.Type, "color") {
			colorKeys[f.Key] = true
		}
	}
	overrides := make(map[string]string)
	for key, value := range e.Config {
		if strings.HasPrefix(key, "--") {
			overrides[key] = value
			continue
		}
		if key == "accent" && colorKeys[key] {
			overrides["--accent"] = value
		}
	}
	if len(overrides) == 0 {
		return e.UI
	}
	theme := make(map[string]string, len(e.UI.Theme)+len(overrides))
	for key, value := range e.UI.Theme {
		theme[key] = value
	}
	for key, value := range overrides {
		theme[key] = value
	}
	e.UI.Theme = theme
	return e.UI
}

// ListExtensionUI returns merged UI contributions from applied extensions.
// The dashboard uses this to modify navigation, pages, banners, widgets, theme.
// GET /admin/api/v1/sidecar/extensions/ui
func (h *Handler) ListExtensionUI(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusOK, map[string]any{"contributions": []any{}})
	}
	out := make([]ExtensionUIContribution, 0)
	for _, e := range h.extensions.List() {
		if !e.Applied {
			continue
		}
		ui := extensionUIWithConfig(e)
		if ui.Accent == "" && len(ui.Nav) == 0 && len(ui.Pages) == 0 &&
			len(ui.Banners) == 0 && len(ui.Widgets) == 0 &&
			len(ui.SettingsTabs) == 0 && len(ui.HideNav) == 0 &&
			len(ui.HideSettingsTabs) == 0 && ui.Help == "" &&
			len(ui.Theme) == 0 && ui.LogoText == "" && ui.LogoURL == "" {
			continue
		}
		out = append(out, ExtensionUIContribution{ID: e.ID, Name: e.Name, UI: ui})
	}
	return c.JSON(http.StatusOK, map[string]any{"contributions": out})
}

// ListExtensionStores returns configured store base URLs. Operators type a
// store URL in the dashboard; extensions do not ship catalog URLs.
func (h *Handler) ListExtensionStores(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"stores": []string{}})
}

// storeCatalogURLs returns candidate catalog URLs for a store base. The first
// entry is the plain-static layout, which every static host serves (GitHub
// Pages included); the second is the nginx "pretty route" alias.
func storeCatalogURLs(base string) []string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	return []string{
		trimmed + "/api/v1/extensions.json",
		trimmed + "/api/v1/extensions",
	}
}

// storeRawURLs returns candidate raw extension URLs, static layout first.
func storeRawURLs(base, id string) []string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	escaped := url.PathEscape(id)
	return []string{
		trimmed + "/extensions/" + escaped + ".extension.json",
		trimmed + "/api/v1/extensions/" + escaped + "/raw",
	}
}

// fetchFirstOK GETs each candidate in order and returns the first 200 body
// together with the URL that produced it.
func fetchFirstOK(candidates []string) (string, []byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	var lastErr error
	for _, candidate := range candidates {
		resp, err := client.Get(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return candidate, data, nil
		}
		lastErr = fmt.Errorf("store status %d", resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no store url candidates")
	}
	return "", nil, lastErr
}

// attachRawURLs fills in a raw_url for every catalog entry so the dashboard
// installs from a URL that actually exists on the store host.
func attachRawURLs(catalog any, base string) {
	root, ok := catalog.(map[string]any)
	if !ok {
		return
	}
	list, ok := root["extensions"].([]any)
	if !ok {
		return
	}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := entry["id"].(string)
		if id == "" {
			continue
		}
		if existing, ok := entry["raw_url"].(string); ok && existing != "" {
			continue
		}
		entry["raw_url"] = storeRawURLs(base, id)[0]
	}
}

// BrowseExtensionStore proxies a search to a configured (or ad-hoc) store.
// Query: url=<store base> [q=] [tag=] [type=].
func (h *Handler) BrowseExtensionStore(c *echo.Context) error {
	base := strings.TrimSpace(c.QueryParam("url"))
	if base == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "url is required"})
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "url must be http or https"})
	}
	browseQuery := u.Query()
	for _, key := range []string{"q", "tag", "source", "type"} {
		if v := strings.TrimSpace(c.QueryParam(key)); v != "" {
			browseQuery.Set(key, v)
		}
	}

	candidates := make([]string, 0, 2)
	for _, candidate := range storeCatalogURLs(base) {
		parsed, parseErr := url.Parse(candidate)
		if parseErr != nil {
			continue
		}
		merged := parsed.Query()
		for key, values := range browseQuery {
			for _, v := range values {
				merged.Add(key, v)
			}
		}
		parsed.RawQuery = merged.Encode()
		candidates = append(candidates, parsed.String())
	}

	_, data, fetchErr := fetchFirstOK(candidates)
	if fetchErr != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fetchErr.Error()})
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "invalid store JSON"})
	}
	attachRawURLs(parsed, base)
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return c.JSONBlob(http.StatusOK, data)
	}
	return c.JSONBlob(http.StatusOK, encoded)
}

// InstallExtensionFromStore installs an extension by raw URL (typically a
// store's /extensions/{id}.extension.json file) or by store base + id.
func (h *Handler) InstallExtensionFromStore(c *echo.Context) error {
	var req struct {
		URL   string `json:"url"`
		Store string `json:"store"`
		ID    string `json:"id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	target := strings.TrimSpace(req.URL)
	if target == "" {
		if req.Store == "" || req.ID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "url or store+id is required"})
		}
		resolved, _, fetchErr := fetchFirstOK(storeRawURLs(req.Store, req.ID))
		if fetchErr != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": fetchErr.Error()})
		}
		target = resolved
	}
	body, _ := json.Marshal(map[string]string{"url": target})
	c.Request().Body = io.NopCloser(strings.NewReader(string(body)))
	return h.ImportExtension(c)
}

func fetchExtensionFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("url must be http or https")
	}
	client := &http.Client{}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("fetch extension: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch extension: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read extension: %w", err)
	}
	return string(data), nil
}
