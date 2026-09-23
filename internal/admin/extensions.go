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
	Type        string   `json:"type,omitempty"` // text | number | boolean | select
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
	ID       string `json:"id"`
	Level    string `json:"level,omitempty"` // info | warning | error | success
	Message  string `json:"message"`
	LinkLabel string `json:"link_label,omitempty"`
	LinkURL  string `json:"link_url,omitempty"`
	Order    int    `json:"order,omitempty"`
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
	Kind     string   `json:"kind"` // heading | text | code | list | links | divider | kv
	Text     string   `json:"text,omitempty"`
	Language string   `json:"language,omitempty"`
	Items    []string `json:"items,omitempty"`
	Links    []ExtensionNavLink `json:"links,omitempty"`
	// KV is for kind=kv: list of {k,v} or {label,value}.
	KV []map[string]string `json:"kv,omitempty"`
}

// ExtensionUIPage is a full dashboard page contributed by an extension.
type ExtensionUIPage struct {
	ID     string             `json:"id"`
	Path   string             `json:"path"` // slug under /admin/dashboard/ext/
	Title  string             `json:"title"`
	Icon   string             `json:"icon,omitempty"`
	Order  int                `json:"order,omitempty"`
	Summary string            `json:"summary,omitempty"`
	Blocks []ExtensionUIBlock `json:"blocks,omitempty"`
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
	Files    map[string]string   `json:"files,omitempty"`
	Provides *ExtensionProvides  `json:"provides,omitempty"`
	UI       ExtensionUI         `json:"ui,omitempty"`
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

// ListExtensions returns all imported extensions.
func (h *Handler) ListExtensions(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusOK, map[string]any{"extensions": []any{}, "presets": []any{}})
	}
	list := h.extensions.List()
	// "presets" is a temporary alias for older dashboard builds.
	return c.JSON(http.StatusOK, map[string]any{"extensions": list, "presets": list})
}

// GetExtension returns a single extension by id.
func (h *Handler) GetExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
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
		"status":   "ok",
		"extension": saved,
		"preset":   saved,
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

// ApplyExtension applies an extension's sidecar settings and returns the header
// rules the caller should install into the Session Hub via
// /sessionhub/providers/:name/ensure-headers. Optional provider types declared
// under provides.provider_types are activated on apply.
func (h *Handler) ApplyExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	if h.sidecarStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "sidecar store unavailable"})
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
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "materialize files: " + ferr.Error()})
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
		"status":                    "ok",
		"extension":                 ext.ID,
		"preset":                    ext.ID,
		"headers":                   ext.Headers,
		"tools":                     tools,
		"activated_provider_types":  activated,
		"provides":                  ext.Provides,
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
	return c.JSON(http.StatusOK, resp)
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
		if e.UI.Accent == "" && len(e.UI.Nav) == 0 && len(e.UI.Pages) == 0 &&
			len(e.UI.Banners) == 0 && len(e.UI.Widgets) == 0 &&
			len(e.UI.SettingsTabs) == 0 && len(e.UI.HideNav) == 0 &&
			len(e.UI.HideSettingsTabs) == 0 && e.UI.Help == "" &&
			len(e.UI.Theme) == 0 {
			continue
		}
		out = append(out, ExtensionUIContribution{ID: e.ID, Name: e.Name, UI: e.UI})
	}
	return c.JSON(http.StatusOK, map[string]any{"contributions": out})
}

// ListExtensionStores returns configured store base URLs. Operators type a
// store URL in the dashboard; extensions do not ship catalog URLs.
func (h *Handler) ListExtensionStores(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"stores": []string{}})
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
	browse := *u
	browse.Path = strings.TrimRight(browse.Path, "/") + "/api/v1/extensions"
	q := browse.Query()
	for _, key := range []string{"q", "tag", "source", "type"} {
		if v := strings.TrimSpace(c.QueryParam(key)); v != "" {
			q.Set(key, v)
		}
	}
	browse.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Get(browse.String())
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "store fetch: " + err.Error()})
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "store read: " + err.Error()})
	}
	if resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("store status %d", resp.StatusCode),
		})
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "invalid store JSON"})
	}
	return c.JSONBlob(http.StatusOK, data)
}

// InstallExtensionFromStore installs an extension by raw URL (typically a
// store's /api/v1/extensions/{id}/raw endpoint) or by store base + id.
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
		target = strings.TrimRight(strings.TrimSpace(req.Store), "/") +
			"/api/v1/extensions/" + url.PathEscape(req.ID) + "/raw"
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
