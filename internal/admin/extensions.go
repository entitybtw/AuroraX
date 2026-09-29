package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/addon"
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
// extension. Extensions declare arbitrary fields so external extensions
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
	// Theme maps CSS custom properties applied as the base/dark palette.
	Theme map[string]string `json:"theme,omitempty"`
	// ThemeLight overrides applied when the dashboard is in light mode
	// (data-theme="light" or prefers-color-scheme: light). Merged on top of Theme.
	ThemeLight map[string]string `json:"theme_light,omitempty"`
	// ThemeDark overrides applied when the dashboard is in dark mode.
	// Merged on top of Theme (usually redundant with Theme alone).
	ThemeDark map[string]string `json:"theme_dark,omitempty"`
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

// ExtensionAuth carries device-flow or authorization-code+PKCE wiring an
// extension supplies so the gateway and dashboard do not hardcode
// provider-specific auth endpoints. The gateway core never interprets these
// fields: auth addons (kind auth) read them from the stored manifest.
type ExtensionAuth struct {
	// Server is the auth server base URL (device authorization).
	Server string `json:"server,omitempty"`
	// ClientID is the public client_id used in the device flow.
	ClientID string `json:"client_id,omitempty"`
	// VerificationBase is the origin used to expand relative verification URIs.
	// Empty = treat verification URIs as absolute.
	VerificationBase string `json:"verification_base,omitempty"`
	// UserAgent is an optional UA for auth HTTP calls.
	UserAgent string `json:"user_agent,omitempty"`
	// Scope is an optional scope string for the device flow.
	Scope string `json:"scope,omitempty"`
	// Grant selects the flow: "device" (default) or "authorization_code".
	Grant string `json:"grant,omitempty"`
	// AuthorizeURL is the browser authorize endpoint (authorization_code).
	AuthorizeURL string `json:"authorize_url,omitempty"`
	// TokenURL is the token endpoint for code exchange / refresh (JSON body
	// unless TokenStyle is "form").
	TokenURL string `json:"token_url,omitempty"`
	// TokenStyle: "json" (default) or "form".
	TokenStyle string `json:"token_style,omitempty"`
	// Scopes is the space-delimited scope list for authorization_code.
	Scopes string `json:"scopes,omitempty"`
	// StateIsVerifier: when true, state equals the PKCE code_verifier.
	StateIsVerifier bool `json:"state_is_verifier,omitempty"`
	// RedirectURI overrides the loopback redirect (default 127.0.0.1 callback).
	RedirectURI string `json:"redirect_uri,omitempty"`
}

// ExtensionProvides declares optional capabilities that are not built into the
// gateway by default and are activated when the extension is installed —
// for example optional provider types or auth addon wiring.
type ExtensionProvides struct {
	// ProviderTypes are factory provider types this extension activates.
	// Types absent from this list stay unregistered.
	ProviderTypes []string `json:"provider_types,omitempty"`
	// Features are optional capability flags surfaced to the operator
	// (e.g. ["external_auth"]).
	Features []string `json:"features,omitempty"`
}

// ExtensionFile is one materialised extension file. The JSON form accepts:
//
//	"auth.go": "// code…"                 (plain string)
//	"auth.go": ["line", "line"]           (array of lines, joined with "\n")
//	"auth.go": {"ref": "ext/auth.go"}     (code lives in a separate file next
//	                                       to the manifest; content is fetched
//	                                       from the extension source)
//	{"ref": …, "content": …}              (ref with already-fetched content)
//
// Refs keep large embedded sources out of the manifest so documents stay
// reviewable; content is resolved at import/apply time and persisted so the
// gateway keeps working offline after a successful import.
type ExtensionFile struct {
	Ref     string
	Content string
}

// ExtensionFiles maps relative materialisation paths to their entries.
type ExtensionFiles map[string]ExtensionFile

func (f *ExtensionFile) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = ExtensionFile{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.Content = s
		return nil
	}
	var lines []string
	if err := json.Unmarshal(data, &lines); err == nil {
		f.Content = strings.Join(lines, "\n")
		return nil
	}
	var obj struct {
		Ref     string          `json:"ref"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("must be a string, an array of lines, or {\"ref\": …}")
	}
	// An object with only "content" is an embedded file written in object
	// form; "" ref simply round-trips back to the plain string/array shape.
	f.Ref = obj.Ref
	if len(obj.Content) == 0 || string(obj.Content) == "null" {
		return nil
	}
	var content string
	if err := json.Unmarshal(obj.Content, &content); err == nil {
		f.Content = content
		return nil
	}
	var contentLines []string
	if err := json.Unmarshal(obj.Content, &contentLines); err != nil {
		return fmt.Errorf("content must be a string or an array of lines")
	}
	f.Content = strings.Join(contentLines, "\n")
	return nil
}

func (f ExtensionFile) MarshalJSON() ([]byte, error) {
	content, err := marshalFileContent(f.Content)
	if err != nil {
		return nil, err
	}
	if f.Ref == "" {
		return content, nil
	}
	obj := map[string]json.RawMessage{"ref": mustJSON(f.Ref)}
	if f.Content != "" {
		obj["content"] = content
	}
	return json.Marshal(obj)
}

// marshalFileContent emits single-line content as a string and multi-line
// content as an array of lines so embedded sources stay reviewable.
func marshalFileContent(content string) (json.RawMessage, error) {
	if strings.Contains(content, "\n") {
		return json.Marshal(strings.Split(content, "\n"))
	}
	return json.Marshal(content)
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}

func (f *ExtensionFiles) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(ExtensionFiles, len(raw))
	for key, val := range raw {
		var entry ExtensionFile
		if err := json.Unmarshal(val, &entry); err != nil {
			return fmt.Errorf("files[%q]: %w", key, err)
		}
		out[key] = entry
	}
	*f = out
	return nil
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
	// Auth supplies auth endpoints for this extension's providers; only auth
	// addons (kind auth) read it.
	Auth *ExtensionAuth `json:"auth,omitempty"`
	// Files is a map of relative path → file content materialised under
	// configs/extensions/<id>/ on apply (tool schemas, helper scripts, …).
	// Paths must stay under the extension directory (no ".." / absolute).
	// The JSON form accepts either a plain string or an array of lines
	// (joined with "\n") so authored documents keep embedded sources
	// readable instead of escaping every newline onto one giant line.
	Files    ExtensionFiles  `json:"files,omitempty"`
	Provides *ExtensionProvides `json:"provides,omitempty"`
	UI       ExtensionUI        `json:"ui,omitempty"`
	// Config holds user-saved overrides: values for ui.fields keys and
	// optional theme var overrides (CSS custom properties starting with "--").
	// It is persisted with the extension and merged into the UI theme on
	// ListExtensionUI.
	Config map[string]string `json:"config,omitempty"`
	// Applied marks that the operator activated this extension (UI + sidecar).
	Applied bool `json:"applied,omitempty"`
	// Order is the operator-controlled sort position in the extensions list.
	Order   int    `json:"order,omitempty"`
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
// optional provider types it provides. When a record with the same id
// already exists, Config is merged so operator overrides survive.
func (s *ExtensionStore) Upsert(ext Extension) []string {
	s.mu.Lock()
	ext.Builtin = false
	replaced := false
	for i, e := range s.imported {
		if e.ID == ext.ID {
			ext.Config = mergeConfig(e.Config, ext.Config)
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

// ResetConfig clears user-saved overrides for one extension, restoring the
// shipped defaults (ui field values and theme variable overrides). Unlike
// Upsert it does not merge, so the Config map is actually removed.
func (s *ExtensionStore) ResetConfig(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.imported {
		if e.ID == id {
			e.Config = nil
			s.imported[i] = e
			s.saveLocked()
			return true
		}
	}
	return false
}

// Import inserts or re-imports an extension, preserving operator state on
// re-import: existing Config is merged, Applied and a custom Name are kept,
// and Order is kept when the incoming record does not set one.
func (s *ExtensionStore) Import(ext Extension) []string {
	s.mu.Lock()
	ext.Builtin = false
	replaced := false
	for i, e := range s.imported {
		if e.ID == ext.ID {
			ext.Name = e.Name
			ext.Applied = e.Applied
			if ext.Order == 0 {
				ext.Order = e.Order
			}
			// Existing operator Config wins over incoming keys; new keys are added.
			ext.Config = mergeConfig(ext.Config, e.Config)
			if ext.Source == "" {
				ext.Source = e.Source
			}
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

// mergeConfig overlays incoming onto existing (incoming keys win).
func mergeConfig(existing, incoming map[string]string) map[string]string {
	if len(existing) == 0 {
		return incoming
	}
	if len(incoming) == 0 {
		return existing
	}
	merged := make(map[string]string, len(existing)+len(incoming))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range incoming {
		merged[k] = v
	}
	return merged
}

// Reorder assigns Order from an explicit id sequence. Ids not listed keep
// their relative order after the listed ones.
func (s *ExtensionStore) Reorder(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		if _, ok := pos[id]; ok {
			return fmt.Errorf("duplicate id %q", id)
		}
		found := false
		for _, e := range s.imported {
			if e.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("extension %q not found", id)
		}
		pos[id] = i
	}
	for i := range s.imported {
		if p, ok := pos[s.imported[i].ID]; ok {
			s.imported[i].Order = p
			continue
		}
		s.imported[i].Order = len(ids) + i
	}
	s.saveLocked()
	return nil
}

// UnapplyThemesExcept deactivates every applied theme extension except id.
// Exactly one theme may stay active at a time.
func (s *ExtensionStore) UnapplyThemesExcept(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.imported {
		e := &s.imported[i]
		if e.Applied && e.Type == "theme" && e.ID != id {
			e.Applied = false
			changed = true
		}
	}
	if changed {
		s.saveLocked()
	}
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

// Validate normalizes and checks an extension. Problems accumulate so an
// import reports every broken field at once instead of one at a time.
func (p *Extension) Validate() error {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if p.ID == "" {
		add("id: required")
	} else if strings.ContainsAny(p.ID, " /\\") {
		add("id: must not contain spaces or slashes")
	}
	if p.Schema != 0 && p.Schema != 1 {
		add("schema: unsupported version %d (supported: 1)", p.Schema)
	}
	if p.BaseURL != "" && !isHTTPURL(p.BaseURL) {
		add("base_url: %q must be an absolute http(s) URL", p.BaseURL)
	}
	if p.Type == "" {
		// Default to sidecar; explicit types such as "theme" are preserved.
		p.Type = "sidecar"
	}
	// An explicit extension id is a normal store id; provider types live in
	// a separate namespace (provides.provider_types).
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

	// Header rules must map onto a mode the session hub actually implements.
	validModes := map[string]bool{
		"generate": true, "map": true, "map_or_generate": true,
		"passthrough": true, "static": true, "random_from_list": true, "remove": true,
	}
	validCharsets := map[string]bool{"": true, "alphanumeric": true, "hex": true, "digits": true}
	for i, h := range p.Headers {
		field := fmt.Sprintf("headers[%d]", i)
		name := strings.TrimSpace(h.Name)
		if name == "" {
			add("%s.name: required", field)
		} else if strings.ContainsAny(name, " \t") {
			add("%s.name: %q is not a valid header name", field, h.Name)
		}
		if !validModes[h.Mode] {
			add("%s.mode: %q is unknown (use generate, map, map_or_generate, passthrough, static, random_from_list or remove)", field, h.Mode)
		}
		if !validCharsets[h.Charset] {
			add("%s.charset: %q is unknown (alphanumeric, hex or digits)", field, h.Charset)
		}
		switch h.Mode {
		case "static":
			if strings.TrimSpace(h.Value) == "" {
				add("%s: static mode requires value", field)
			}
		case "random_from_list":
			if len(h.Values) == 0 {
				add("%s: random_from_list mode requires values", field)
			}
		case "generate", "map_or_generate":
			if h.Length <= 0 {
				add("%s.length: %s mode needs a positive length", field, h.Mode)
			}
		}
	}

	// Auth-flow wiring.
	if p.Auth != nil {
		o := p.Auth
		for field, value := range map[string]string{
			"server":            o.Server,
			"authorize_url":     o.AuthorizeURL,
			"token_url":         o.TokenURL,
			"verification_base": o.VerificationBase,
		} {
			if value != "" && !isHTTPURL(value) {
				add("auth.%s: %q must be an absolute http(s) URL", field, value)
			}
		}
		if strings.TrimSpace(o.ClientID) == "" {
			add("auth.client_id: required")
		}
		switch strings.TrimSpace(o.Grant) {
		case "", "device":
			if strings.TrimSpace(o.Server) == "" {
				add("auth.server: required for the device grant")
			}
		case "authorization_code":
			if strings.TrimSpace(o.AuthorizeURL) == "" {
				add("auth.authorize_url: required for the authorization_code grant")
			}
			if strings.TrimSpace(o.TokenURL) == "" {
				add("auth.token_url: required for the authorization_code grant")
			}
			if o.TokenStyle != "" && o.TokenStyle != "json" && o.TokenStyle != "form" {
				add("auth.token_style: %q is unknown (json or form)", o.TokenStyle)
			}
		default:
			add("auth.grant: %q is unknown (device or authorization_code)", o.Grant)
		}
	}

	if p.Provides != nil {
		for i, t := range p.Provides.ProviderTypes {
			if strings.TrimSpace(t) == "" || strings.ContainsAny(t, " \t") {
				add("provides.provider_types[%d]: %q is not a valid type name", i, t)
			}
		}
		for i, f := range p.Provides.Features {
			if strings.TrimSpace(f) == "" {
				add("provides.features[%d]: empty feature name", i)
			}
		}
	}

	for i, t := range p.Tools {
		if strings.TrimSpace(t.Name) == "" && strings.TrimSpace(t.Source) == "" && len(t.InlineJSON) == 0 {
			add("tool_schemas[%d]: needs a name, source or inline schema", i)
		}
	}

	// UI form fields.
	fieldKeys := map[string]bool{}
	validFieldTypes := map[string]bool{"": true, "text": true, "number": true, "boolean": true, "select": true, "color": true}
	for i, f := range p.UI.Fields {
		field := fmt.Sprintf("ui.fields[%d]", i)
		key := strings.TrimSpace(f.Key)
		if key == "" {
			add("%s.key: required", field)
		} else if fieldKeys[key] {
			add("%s.key: duplicate key %q", field, key)
		} else {
			fieldKeys[key] = true
		}
		if !validFieldTypes[strings.TrimSpace(f.Type)] {
			add("%s.type: %q is unknown (text, number, boolean, select or color)", field, f.Type)
		}
		if f.Type == "select" && len(f.Options) == 0 {
			add("%s: select field needs options", field)
		}
	}

	// Settings keys that must carry JSON string arrays.
	for _, key := range []string{"forward_headers", "inject_tool_types"} {
		if v, ok := p.Settings[key]; ok && strings.TrimSpace(v) != "" {
			var arr []string
			if err := json.Unmarshal([]byte(v), &arr); err != nil {
				add("settings.%s: must be a JSON array of strings", key)
			}
		}
	}

	if p.Files != nil {
		for rel, entry := range p.Files {
			if !safeExtRelPath(rel) {
				add("files: path %q is not allowed (must be relative and stay under the extension dir)", rel)
			}
			if entry.Ref == "" {
				continue
			}
			if isHTTPURL(entry.Ref) {
				continue
			}
			if !safeExtRelPath(entry.Ref) {
				add("files[%q].ref: %q is not allowed (use a relative path next to the manifest or an http(s) URL)", rel, entry.Ref)
			}
		}
	}

	if len(errs) > 0 {
		// Report everything; only a hostile input could produce thousands of
		// problems, so clip at a generous bound.
		if len(errs) > 30 {
			errs = append(errs[:30:30], fmt.Sprintf("… and %d more problem(s)", len(errs)-30))
		}
		return fmt.Errorf("extension validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// isHTTPURL reports whether value is an absolute http:// or https:// URL.
func isHTTPURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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
// tools path, or "" when none was written. Entries that only carry a ref must
// have been resolved first (ResolveFileRefs) — otherwise the write is refused
// rather than silently producing an empty file.
func (p *Extension) MaterializeFiles(dir string) (toolsPath string, err error) {
	if len(p.Files) == 0 {
		return "", nil
	}
	for rel, entry := range p.Files {
		if !safeExtRelPath(rel) {
			return "", fmt.Errorf("unsafe files path %q", rel)
		}
		if entry.Ref != "" && entry.Content == "" {
			return "", fmt.Errorf("file %q references %q which was not loaded — re-import the extension or upload the file", rel, entry.Ref)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, []byte(entry.Content), 0o644); err != nil {
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

// cloneExtensionFiles copies the file map so ref resolution never mutates the
// record held by the extension store (maps are shared by reference).
func cloneExtensionFiles(in ExtensionFiles) ExtensionFiles {
	if in == nil {
		return nil
	}
	out := make(ExtensionFiles, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// resolveFileRef turns a files ref into a fetchable URL: absolute http(s) refs
// pass through, relative refs resolve against the extension source URL.
func resolveFileRef(baseURL, ref string) (string, error) {
	if isHTTPURL(ref) {
		return ref, nil
	}
	base, err := url.Parse(baseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return "", fmt.Errorf("cannot resolve %q: extension has no source url", ref)
	}
	rel, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid ref %q: %w", ref, err)
	}
	return base.ResolveReference(rel).String(), nil
}

// ResolveFileRefs fills Content for entries that only carry a Ref by fetching
// the referenced file (relative refs resolve against baseURL, the extension
// source URL). Returns the "files" keys that could not be loaded, sorted —
// callers report them so the dashboard can ask the operator to upload those
// exact entries. Entries that already carry content are left untouched.
func (p *Extension) ResolveFileRefs(baseURL string) []string {
	if len(p.Files) == 0 {
		return nil
	}
	var missing []string
	for key, entry := range p.Files {
		if entry.Ref == "" || entry.Content != "" {
			continue
		}
		target, err := resolveFileRef(baseURL, entry.Ref)
		if err != nil {
			missing = append(missing, key)
			continue
		}
		body, err := fetchExtensionFromURL(target)
		if err != nil {
			missing = append(missing, key)
			continue
		}
		entry.Content = body
		p.Files[key] = entry
	}
	sort.Strings(missing)
	return missing
}

// resolveFileRefsFor resolves refs against the extension's own source,
// falling back to configured stores when no explicit source is set.
func (h *Handler) resolveFileRefsFor(ext *Extension) []string {
	base := strings.TrimSpace(ext.Source)
	if base == "" {
		if src, err := h.resolveExtensionSource(*ext); err == nil {
			base = src
		}
	}
	return ext.ResolveFileRefs(base)
}

// WithExtensionStore sets the extension store on the handler.
func WithExtensionStore(store *ExtensionStore) Option {
	return func(h *Handler) {
		h.extensions = store
	}
}

// WithAddonStore sets the Yaegi addon store on the handler. Extension-applied
// *.go files (auth addons) are loaded from it on apply and their UI
// contributions merge into ListExtensionUI.
func WithAddonStore(store *addon.Store) Option {
	return func(h *Handler) {
		h.addonStore = store
	}
}

// ProvidesAppliedFeature reports whether any applied extension declares the
// named capability under provides.features (e.g. "external_auth"). These
// capabilities are extension-only: nothing built-in enables them.
func (s *ExtensionStore) ProvidesAppliedFeature(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.imported {
		if e.Applied && extensionProvidesFeature(e.Provides, name) {
			return true
		}
	}
	return false
}

// ListAddons returns the Yaegi addon store status (loaded scripts, kinds,
// load errors) so operators can see extension-shipped runtime addons.
func (h *Handler) ListAddons(c *echo.Context) error {
	if h.addonStore == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"dir": "", "loaded": []string{}, "kinds": map[string]string{},
		})
	}
	return c.JSON(http.StatusOK, h.addonStore.Status())
}

// ListExtensions returns all imported extensions. Theme detection uses
// type/ui.theme in the dashboard.
func (h *Handler) ListExtensions(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusOK, map[string]any{"extensions": []any{}, "presets": []any{}})
	}
	// "presets" is a temporary alias for older dashboard builds.
	return c.JSON(http.StatusOK, map[string]any{"extensions": h.extensions.List(), "presets": h.extensions.List()})
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
	// Resolve companion-file refs on the way out so the dashboard editor
	// always shows real content instead of a bare {"ref": …} marker.
	ext.Files = cloneExtensionFiles(ext.Files)
	h.resolveFileRefsFor(&ext)
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
	// Pull referenced companion files into the export so the downloaded
	// document is self-contained even if the local record only carries refs.
	ext.Files = cloneExtensionFiles(ext.Files)
	h.resolveFileRefsFor(&ext)
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
		// Files carries operator-uploaded companion files (keyed by the
		// "files" entry name) for manifests whose refs cannot be fetched —
		// e.g. a URL import whose companion file is missing at the source.
		Files map[string]string `json:"files"`
	}
	sourceURL := ""
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.URL != "" {
		fetched, ferr := fetchExtensionFromURL(envelope.URL)
		if ferr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": ferr.Error()})
		}
		raw = fetched
		sourceURL = envelope.URL
	}

	var ext Extension
	if err := json.Unmarshal([]byte(raw), &ext); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid extension JSON: " + err.Error()})
	}
	if sourceURL != "" {
		ext.Source = sourceURL
	}
	if err := ext.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	// Operator-supplied companion files win over anything the manifest could
	// fetch: they are the answer to a missing_files challenge.
	for key, content := range envelope.Files {
		entry, ok := ext.Files[key]
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("uploaded file %q is not referenced by this extension", key),
			})
		}
		entry.Content = content
		ext.Files[key] = entry
	}
	// Refs to companion files (e.g. addon .go sources) resolve against the
	// manifest URL when importing from a URL or store. For a pasted document
	// there is no base to resolve against, so the caller is told which files
	// are still needed and can upload them.
	base := sourceURL
	if base == "" {
		base = strings.TrimSpace(ext.Source)
	}
	if missing := ext.ResolveFileRefs(base); len(missing) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":         "extension references files that were not provided: " + strings.Join(missing, ", "),
			"missing_files": missing,
		})
	}
	activated := h.extensions.Import(ext)
	saved, _ := h.extensions.Get(ext.ID)
	return c.JSON(http.StatusOK, map[string]any{
		"status":                   "ok",
		"extension":                saved,
		"preset":                   saved,
		"activated_provider_types": activated,
	})
}

// UpdateExtension renames an extension, sets its list order, and/or replaces
// the sync source URL (empty string clears it so the source resolves from
// configured stores again).
// PUT /sidecar/extensions/:id {"name":"...","order":2,"source":"https://..."}
func (h *Handler) UpdateExtension(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	var req struct {
		Name   *string `json:"name"`
		Order  *int    `json:"order"`
		Source *string `json:"source"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	changed := false
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name must not be empty"})
		}
		ext.Name = name
		changed = true
	}
	if req.Order != nil {
		ext.Order = *req.Order
		changed = true
	}
	if req.Source != nil {
		source := strings.TrimSpace(*req.Source)
		if source != "" {
			if err := validateSourceURL(source); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
		}
		ext.Source = source
		changed = true
	}
	if !changed {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name, order or source is required"})
	}
	h.extensions.Upsert(ext)
	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"extension": ext,
	})
}

// ReorderExtensions assigns list order from an explicit id sequence.
// POST /sidecar/extensions/reorder {"ids":["a","b"]}
func (h *Handler) ReorderExtensions(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ids is required"})
	}
	if err := h.extensions.Reorder(req.IDs); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":     "ok",
		"extensions": h.extensions.List(),
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
	// Remove the extension's Go addons so its settings tabs and hooks go away.
	DetachExtensionAddons(h.addonStore, c.Param("id"))
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// UnapplyExtension deactivates an extension (Applied=false). Sidecar settings
// and session hub headers installed by a previous apply are left untouched;
// the operator re-applies another extension or edits settings to replace them.
// Extension-shipped Go addons are unloaded so their settings tabs disappear
// immediately.
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
	DetachExtensionAddons(h.addonStore, ext.ID)
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

// ResetExtensionConfig clears user-saved overrides (ui field values and theme
// variable overrides) so the extension falls back to its shipped defaults.
// The reset is only offered while the extension has a resolvable sync source.
// POST /sidecar/extensions/:id/config/reset
func (h *Handler) ResetExtensionConfig(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	if _, err := h.resolveExtensionSource(ext); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cannot reset without a sync source: " + err.Error(),
		})
	}
	if !h.extensions.ResetConfig(ext.ID) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
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
	// Optional flexible sidecar routing / header policy from settings JSON.
	if raw, ok := ext.Settings["extra_headers"]; ok && strings.TrimSpace(raw) != "" {
		var m map[string]string
		if json.Unmarshal([]byte(raw), &m) == nil && len(m) > 0 {
			next.ExtraHeaders = m
		}
	}
	if raw, ok := ext.Settings["forward_headers"]; ok && strings.TrimSpace(raw) != "" {
		var list []string
		if json.Unmarshal([]byte(raw), &list) == nil && len(list) > 0 {
			next.ForwardHeaders = list
		}
	}
	if raw, ok := ext.Settings["retry_statuses"]; ok && strings.TrimSpace(raw) != "" {
		var list []int
		if json.Unmarshal([]byte(raw), &list) == nil && len(list) > 0 {
			next.RetryStatuses = list
		}
	}
	if raw, ok := ext.Settings["force_stream"]; ok {
		v := strings.EqualFold(strings.TrimSpace(raw), "true")
		next.ForceStream = &v
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
		case "path_template":
			next.PathTemplate = v
		case "models_path":
			next.ModelsPath = v
		}
	}
	// Materialise extension payload files (tool schemas, scripts, …).
	if len(ext.Files) > 0 {
		// Refs may still be unresolved (e.g. imported before a companion file
		// existed) — resolve them against the source without mutating the
		// stored record.
		ext.Files = cloneExtensionFiles(ext.Files)
		if missing := h.resolveFileRefsFor(&ext); len(missing) > 0 {
			return nil, Extension{}, &applyFailure{http.StatusBadRequest,
				"extension file(s) not loaded: " + strings.Join(missing, ", ") + " — re-import the extension or upload the file(s)"}
		}
		toolsPath, ferr := ext.MaterializeFiles(ExtensionFilesDir(ext.ID))
		if ferr != nil {
			return nil, Extension{}, &applyFailure{http.StatusBadRequest, "materialize files: " + ferr.Error()}
		}
		if toolsPath != "" {
			next.ToolsPath = toolsPath
		}
		// Extension-shipped Go addons (files ending in .go) load into the
		// addon store so their runtime hooks / UI contributions activate
		// with the extension.
		h.LoadExtensionAddons(ext.ID)
	}
	h.sidecarStore.update(next)

	var activated []string
	if h.extensions != nil {
		ext.Applied = true
		activated = h.extensions.Upsert(ext)
		if ext.Type == "theme" {
			h.extensions.UnapplyThemesExcept(ext.ID)
		}
	}

	// provides.features "external_auth" wires extension auth addons onto
	// matching providers.
	externalAuthProviders := h.applyExtensionExternalAuth(c, ext)

	// Client emulation profile (User-Agent + TLS-fingerprint sidecar URL) is
	// stamped onto the providers this extension targets so a pool of
	// pool instances all egress with the emulated client fingerprint.
	profileProviders := h.applyExtensionClientProfile(c, ext)

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
	if ext.Auth != nil {
		resp["auth"] = ext.Auth
	}
	if len(externalAuthProviders) > 0 {
		resp["auth_providers"] = externalAuthProviders
	}
	if len(profileProviders) > 0 {
		resp["profile_providers"] = profileProviders
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

// applyExtensionClientProfile stamps an extension's client emulation profile
// (User-Agent and TLS-fingerprint sidecar URL) onto the providers it targets:
// type ∈ provides.provider_types, or base_url equals the extension base_url
// (e.g. the pool instances behind this extension's base_url). Values come from
// the extension itself or its settings map, so the gateway hardcodes no client
// fingerprint. Returns updated provider names.
func (h *Handler) applyExtensionClientProfile(c *echo.Context, ext Extension) []string {
	if h.providerOverrides == nil {
		return nil
	}
	userAgent := strings.TrimSpace(ext.UserAgent)
	sidecarURL := ""
	if v, ok := ext.Settings["sidecar_url"]; ok {
		sidecarURL = strings.TrimSpace(v)
	}
	if userAgent == "" && sidecarURL == "" {
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
		changed := false
		if userAgent != "" && o.UserAgent != userAgent {
			o.UserAgent = userAgent
			changed = true
		}
		if sidecarURL != "" && o.SidecarURL != sidecarURL {
			o.SidecarURL = sidecarURL
			changed = true
		}
		if !changed {
			continue
		}
		h.providerOverrides.upsert(o)
		updated = append(updated, o.Name)
	}
	if len(updated) > 0 && h.runtimeRefresher != nil {
		if _, err := h.runtimeRefresher.RefreshRuntime(c.Request().Context()); err != nil {
			return updated
		}
	}
	return updated
}

// applyExtensionExternalAuth enables auth_method=external on providers that
// this extension targets: type ∈ provides.provider_types, or base_url equals
// the extension base_url (e.g. pool instances). Auth wiring itself lives in
// the extension's auth addon (kind auth), never in core. Returns updated
// provider names. No-op when the extension does not declare
// provides.features "external_auth".
func (h *Handler) applyExtensionExternalAuth(c *echo.Context, ext Extension) []string {
	if h.providerOverrides == nil || ext.Auth == nil ||
		!extensionProvidesFeature(ext.Provides, "external_auth") {
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
		if strings.EqualFold(strings.TrimSpace(o.AuthMethod), "external") {
			continue
		}
		o.AuthMethod = "external"
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
	Type string      `json:"type,omitempty"`
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
	// The accent color field also drives the extension-list swatch and the
	// provider key tint, so mirror it onto UI.Accent alongside the theme maps.
	if accent, ok := overrides["--accent"]; ok && e.UI.Accent != "" {
		e.UI.Accent = accent
	}
	theme := make(map[string]string, len(e.UI.Theme)+len(overrides))
	for key, value := range e.UI.Theme {
		theme[key] = value
	}
	for key, value := range overrides {
		theme[key] = value
	}
	e.UI.Theme = theme
	// Config overrides also apply to the light/dark variants when present.
	if len(e.UI.ThemeLight) > 0 {
		light := make(map[string]string, len(e.UI.ThemeLight)+len(overrides))
		for key, value := range e.UI.ThemeLight {
			light[key] = value
		}
		for key, value := range overrides {
			light[key] = value
		}
		e.UI.ThemeLight = light
	}
	if len(e.UI.ThemeDark) > 0 {
		dark := make(map[string]string, len(e.UI.ThemeDark)+len(overrides))
		for key, value := range e.UI.ThemeDark {
			dark[key] = value
		}
		for key, value := range overrides {
			dark[key] = value
		}
		e.UI.ThemeDark = dark
	}
	return e.UI
}

// AttachExtensionAddons registers an extension's files directory with the
// addon store when it contains *.go addon scripts. Used on apply and at
// startup for already-applied extensions.
func AttachExtensionAddons(addonStore *addon.Store, extID string) {
	if addonStore == nil {
		return
	}
	dir := ExtensionFilesDir(extID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			addonStore.AddDir(dir)
			return
		}
	}
}

// DetachExtensionAddons unloads an extension's *.go addons so its settings
// tabs, hooks and UI contributions disappear as soon as it is disabled or
// deleted (the mirror of AttachExtensionAddons).
func DetachExtensionAddons(addonStore *addon.Store, extID string) {
	if addonStore == nil {
		return
	}
	addonStore.RemoveDir(ExtensionFilesDir(extID))
}

// LoadExtensionAddons scans an applied extension's files directory for *.go
// addon scripts and registers the directory with the addon store (exported
// so startup can re-attach addons for already-applied extensions).
func (h *Handler) LoadExtensionAddons(extID string) {
	AttachExtensionAddons(h.addonStore, extID)
}

// ListExtensionUI returns merged UI contributions from applied extensions.
// The dashboard uses this to modify navigation, pages, banners, widgets, theme.
// GET /admin/api/v1/sidecar/extensions/ui
func (h *Handler) ListExtensionUI(c *echo.Context) error {
	if h.extensions == nil && h.addonStore == nil {
		return c.JSON(http.StatusOK, map[string]any{"contributions": []any{}})
	}
	out := make([]ExtensionUIContribution, 0)
	for _, e := range h.extensionUIList() {
		out = append(out, e)
	}
	// Extension-shipped Go addons (kind auth) contribute UI by exporting
	// UI() as an ExtensionUI JSON document. Contributions are gated on the
	// owning extension still being applied so a stale addon (crash between
	// unapply and unload, or an edited files dir) can never keep a disabled
	// extension's settings tabs on screen.
	if h.addonStore != nil {
		for _, a := range h.addonStore.ByKind(addon.KindAuth) {
			if extID := extensionIDForAddonPath(a.Path); extID != "" && !h.extensionApplied(extID) {
				continue
			}
			raw, err := a.CallString("UI")
			if err != nil || strings.TrimSpace(raw) == "" {
				continue
			}
			var ui ExtensionUI
			if json.Unmarshal([]byte(raw), &ui) != nil {
				continue
			}
			out = append(out, ExtensionUIContribution{
				ID:   "addon:" + a.Name,
				Name: a.Name,
				Type: "auth",
				UI:   ui,
			})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"contributions": out})
}

// extensionApplied reports whether an extension exists and is currently
// applied. Missing extensions count as not applied.
func (h *Handler) extensionApplied(id string) bool {
	if h.extensions == nil || id == "" {
		return false
	}
	ext, ok := h.extensions.Get(id)
	return ok && ext.Applied
}

// extensionIDForAddonPath maps an addon source path to the id of the
// extension that shipped it: configs/extensions/<id>/auth-x.go → <id>.
// Addons outside the extension files tree (operator drop-ins under
// configs/addons) return "" and stay ungated.
func extensionIDForAddonPath(path string) string {
	dir := filepath.Dir(path)
	id := filepath.Base(dir)
	if id == "" || id == "." || id == string(filepath.Separator) {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	want, err := filepath.Abs(ExtensionFilesDir(id))
	if err != nil {
		return ""
	}
	if abs != want {
		return ""
	}
	return id
}

// extensionUIList returns the applied extensions' UI contributions.
func (h *Handler) extensionUIList() []ExtensionUIContribution {
	if h.extensions == nil {
		return nil
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
			len(ui.Theme) == 0 && len(ui.ThemeLight) == 0 && len(ui.ThemeDark) == 0 &&
			ui.LogoText == "" && ui.LogoURL == "" {
			continue
		}
		out = append(out, ExtensionUIContribution{ID: e.ID, Name: e.Name, Type: e.Type, UI: ui})
	}
	return out
}

// ListExtensionStores returns configured store base URLs persisted under
// AURORA_EXTENSION_STORES_PATH (default configs/extension-stores.json).
func (h *Handler) ListExtensionStores(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"stores": h.extensionStores().List()})
}

// AddExtensionStore persists a store base URL (POST {"url":"https://…"}).
func (h *Handler) AddExtensionStore(c *echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	base := strings.TrimSpace(req.URL)
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "url must be http or https"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status": "ok",
		"stores": h.extensionStores().Add(strings.TrimRight(base, "/")),
	})
}

// DeleteExtensionStore removes a persisted store base URL (DELETE ?url=…).
func (h *Handler) DeleteExtensionStore(c *echo.Context) error {
	base := strings.TrimSpace(c.QueryParam("url"))
	if base == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "url is required"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status": "ok",
		"stores": h.extensionStores().Remove(base),
	})
}

// ExtensionStoreURLStore persists the operator-configured store base URLs.
type ExtensionStoreURLStore struct {
	mu     sync.RWMutex
	path   string
	stores []string
}

// NewExtensionStoreURLStore loads persisted store URLs from disk.
func NewExtensionStoreURLStore() *ExtensionStoreURLStore {
	s := &ExtensionStoreURLStore{path: os.Getenv("AURORA_EXTENSION_STORES_PATH")}
	if s.path == "" {
		s.path = "configs/extension-stores.json"
	}
	s.load()
	return s
}

func (s *ExtensionStoreURLStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var loaded []string
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	s.stores = loaded
}

func (s *ExtensionStoreURLStore) saveLocked() {
	data, err := json.MarshalIndent(s.stores, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = os.WriteFile(s.path, data, 0o644)
}

// List returns a copy of the configured store base URLs.
func (s *ExtensionStoreURLStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.stores))
	copy(out, s.stores)
	return out
}

// Add appends a store base URL (idempotent) and persists the list.
func (s *ExtensionStoreURLStore) Add(base string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.stores {
		if existing == base {
			out := make([]string, len(s.stores))
			copy(out, s.stores)
			return out
		}
	}
	s.stores = append(s.stores, base)
	s.saveLocked()
	out := make([]string, len(s.stores))
	copy(out, s.stores)
	return out
}

// Remove drops a store base URL and persists the list.
func (s *ExtensionStoreURLStore) Remove(base string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	next := s.stores[:0]
	for _, existing := range s.stores {
		if existing == base || existing == trimmed {
			continue
		}
		next = append(next, existing)
	}
	s.stores = next
	s.saveLocked()
	out := make([]string, len(s.stores))
	copy(out, s.stores)
	return out
}

// extensionStores returns the handler's store-URL registry, creating it on
// first use so handlers work without an explicit option in tests.
func (h *Handler) extensionStores() *ExtensionStoreURLStore {
	h.extensionStoresOnce.Do(func() {
		if h.extensionStoreURLs == nil {
			h.extensionStoreURLs = NewExtensionStoreURLStore()
		}
	})
	return h.extensionStoreURLs
}

// WithExtensionStoreURLs sets the store-URL registry on the handler.
func WithExtensionStoreURLs(store *ExtensionStoreURLStore) Option {
	return func(h *Handler) {
		h.extensionStoreURLs = store
	}
}

// validateSourceURL accepts only absolute http(s) URLs so a bad source can
// never be persisted (file:// or arbitrary schemes would otherwise leak into
// fetchers).
func validateSourceURL(source string) error {
	u, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("invalid source url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("source url must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("source url must include a host")
	}
	return nil
}

// resolveExtensionSource returns the URL to sync an extension from. An explicit
// Source wins; otherwise every configured store is probed for the same id so an
// extension imported as raw JSON can still be synced with the store it came
// from (or any other configured store that carries it).
func (h *Handler) resolveExtensionSource(ext Extension) (string, error) {
	if source := strings.TrimSpace(ext.Source); source != "" {
		return source, nil
	}
	for _, base := range h.extensionStores().List() {
		found, _, err := fetchFirstOK(storeRawURLs(base, ext.ID))
		if err == nil && found != "" {
			return found, nil
		}
	}
	return "", fmt.Errorf("extension has no source url and was not found in any configured store")
}

// fetchResolvedRemoteExtension resolves the sync source and downloads it,
// verifying the remote id matches. ok=false means "no source configured".
func (h *Handler) fetchResolvedRemoteExtension(ext Extension) (source string, remote Extension, ok bool, errMsg string) {
	resolved, err := h.resolveExtensionSource(ext)
	if err != nil {
		return "", Extension{}, false, err.Error()
	}
	remote, ferr := h.fetchRemoteExtension(resolved)
	if ferr != nil {
		return resolved, Extension{}, false, ferr.Error()
	}
	if remote.ID != ext.ID {
		return resolved, Extension{}, false, fmt.Sprintf("source id %q does not match extension %q", remote.ID, ext.ID)
	}
	return resolved, remote, true, ""
}

// CheckExtensionUpdate re-fetches the sync source (explicit Source, else a
// configured store) and reports whether the remote version differs.
// GET /sidecar/extensions/:id/check-update
func (h *Handler) CheckExtensionUpdate(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	source, remote, ok, errMsg := h.fetchResolvedRemoteExtension(ext)
	if !ok {
		if source == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errMsg})
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": errMsg})
	}
	current := strings.TrimSpace(ext.Version)
	latest := strings.TrimSpace(remote.Version)
	return c.JSON(http.StatusOK, map[string]any{
		"id":               ext.ID,
		"source":           source,
		"current_version":  current,
		"remote_version":   latest,
		"update_available": latest != current,
		"resolved":         strings.TrimSpace(ext.Source) == "",
	})
}

// UpdateExtensionFromSource re-fetches the sync source and replaces metadata
// while preserving operator state (Config, Applied, Order, custom Name, Source).
// POST /sidecar/extensions/:id/update
func (h *Handler) UpdateExtensionFromSource(c *echo.Context) error {
	if h.extensions == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extensions unavailable"})
	}
	ext, ok := h.extensions.Get(c.Param("id"))
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "extension not found"})
	}
	source, remote, ok, errMsg := h.fetchResolvedRemoteExtension(ext)
	if !ok {
		if source == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errMsg})
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": errMsg})
	}
	// Keep an explicit source as-is; persist the resolved store URL only when
	// the extension had none, so later syncs go straight to the same place.
	if strings.TrimSpace(ext.Source) == "" {
		remote.Source = source
	} else {
		remote.Source = ext.Source
	}
	// Companion files resolve against the manifest URL.
	if missing := remote.ResolveFileRefs(source); len(missing) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":         "source references files that could not be fetched: " + strings.Join(missing, ", "),
			"missing_files": missing,
		})
	}
	if err := remote.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	updated := strings.TrimSpace(remote.Version) != strings.TrimSpace(ext.Version)
	h.extensions.Import(remote)
	saved, _ := h.extensions.Get(ext.ID)
	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"updated":   updated,
		"extension": saved,
	})
}

// fetchRemoteExtension downloads and parses an extension JSON document.
func (h *Handler) fetchRemoteExtension(source string) (Extension, error) {
	var remote Extension
	raw, err := fetchExtensionFromURL(source)
	if err != nil {
		return remote, err
	}
	if err := json.Unmarshal([]byte(raw), &remote); err != nil {
		return remote, fmt.Errorf("invalid extension JSON: %w", err)
	}
	return remote, nil
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
