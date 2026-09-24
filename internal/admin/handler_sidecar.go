package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
)

// SidecarSettings holds the runtime configuration for the generic TLS
// sidecar (extension-driven client fingerprint proxy). The settings are
// persisted to a JSON file and read/written via the admin API. Defaults are
// neutral — extensions supply base_url, user-agent, auth, tool scope and
// optional OAuth device-flow endpoints.
type SidecarSettings struct {
	Enabled         bool           `json:"enabled"`
	Port            int            `json:"port"`
	InjectTools     bool           `json:"inject_tools"`
	InjectToolTypes []string       `json:"inject_tool_types"`
	DefaultAuth     string         `json:"default_auth"`
	UserAgent       string         `json:"user_agent"`
	BaseURL         string         `json:"base_url"`
	MaxAttempts     int            `json:"max_attempts"`
	RetryDelayMs    int            `json:"retry_delay_ms"`
	BindIPs         []string       `json:"bind_ips"`
	Proxies         []SidecarProxy `json:"proxies"`
	// ToolsPath is an absolute or sidecar-relative path to a JSON tool schema
	// file supplied by an extension (overrides the bundled default).
	ToolsPath string `json:"tools_path,omitempty"`
	// OAuth defaults installed by the active extension (device flow).
	OAuthServer           string `json:"oauth_server,omitempty"`
	OAuthClientID         string `json:"oauth_client_id,omitempty"`
	OAuthVerificationBase string `json:"oauth_verification_base,omitempty"`
	// OAuthGrant selects the grant: "device" (default) or "authorization_code".
	OAuthGrant string `json:"oauth_grant,omitempty"`
	// Authorization-code + PKCE endpoints (extension-driven).
	OAuthAuthorizeURL    string `json:"oauth_authorize_url,omitempty"`
	OAuthTokenURL        string `json:"oauth_token_url,omitempty"`
	OAuthTokenStyle      string `json:"oauth_token_style,omitempty"`
	OAuthScopes          string `json:"oauth_scopes,omitempty"`
	OAuthStateIsVerifier bool   `json:"oauth_state_is_verifier,omitempty"`
	OAuthRedirectURI     string `json:"oauth_redirect_uri,omitempty"`
	// PathTemplate is the upstream path appended to base_url for chat
	// completions (default "/chat/completions"). Presets may point at
	// alternative OpenAI-compatible routes.
	PathTemplate string `json:"path_template,omitempty"`
	// ModelsPath overrides the upstream path for GET /v1/models (default "/models").
	ModelsPath string `json:"models_path,omitempty"`
	// ExtraHeaders are static headers always sent upstream (merged after
	// envelope headers; Authorization/Content-Type/User-Agent win if set).
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	// ForwardHeaders lists additional inbound header names to forward
	// upstream (in addition to identity headers).
	ForwardHeaders []string `json:"forward_headers,omitempty"`
	// RetryStatuses replaces the default retry status set (403, 429).
	RetryStatuses []int `json:"retry_statuses,omitempty"`
	// ForceStream when true forces stream=true on upstream chat requests.
	// Null/absent: stream forced only when inject_tools is on.
	ForceStream *bool `json:"force_stream,omitempty"`
}

// SidecarProxy represents a single per-IP CONNECT proxy entry.
type SidecarProxy struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

// SidecarStatus is the live status returned by the GET endpoint.
type SidecarStatus struct {
	Running  bool            `json:"running"`
	Settings SidecarSettings `json:"settings"`
	Proxies  []SidecarProxy  `json:"proxies"`
}

// SidecarOverrideStore persists sidecar settings to disk.
type SidecarOverrideStore struct {
	mu       sync.Mutex
	settings SidecarSettings
	path     string
}

// NewSidecarOverrideStore creates or loads sidecar settings from the given config directory.
func NewSidecarOverrideStore() *SidecarOverrideStore {
	s := &SidecarOverrideStore{
		path: os.Getenv("AURORA_SIDECAR_OVERRIDES_PATH"),
		settings: SidecarSettings{
			Enabled:     true,
			Port:        8090,
			InjectTools: true,
			// Empty inject_tool_types allows all provider types. vllm is
			// listed because free-tier pool members report type "vllm".
			InjectToolTypes: []string{"opencode", "vllm", ""},
			DefaultAuth:     "Bearer public",
			// Required UA for free-tier upstreams; extensions may override.
			UserAgent:    "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14",
			MaxAttempts:  4,
			RetryDelayMs: 750,
		},
	}
	if s.path == "" {
		s.path = "configs/sidecar-overrides.json"
	}
	s.load()
	return s
}

// Get returns a copy of the current sidecar settings.
func (s *SidecarOverrideStore) Get() SidecarSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *SidecarOverrideStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var loaded SidecarSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	// Merge with defaults: only override fields that were explicitly set.
	if loaded.Port > 0 {
		s.settings.Port = loaded.Port
	}
	if loaded.DefaultAuth != "" {
		s.settings.DefaultAuth = loaded.DefaultAuth
	}
	if loaded.BaseURL != "" {
		s.settings.BaseURL = loaded.BaseURL
	}
	if loaded.MaxAttempts > 0 {
		s.settings.MaxAttempts = loaded.MaxAttempts
	}
	if loaded.RetryDelayMs > 0 {
		s.settings.RetryDelayMs = loaded.RetryDelayMs
	}
	s.settings.Enabled = loaded.Enabled
	s.settings.InjectTools = loaded.InjectTools
	if loaded.UserAgent != "" {
		s.settings.UserAgent = loaded.UserAgent
	}
	s.settings.BindIPs = loaded.BindIPs
	s.settings.Proxies = loaded.Proxies
	if len(loaded.InjectToolTypes) > 0 {
		s.settings.InjectToolTypes = loaded.InjectToolTypes
	}
	s.settings.ToolsPath = loaded.ToolsPath
	s.settings.OAuthServer = loaded.OAuthServer
	s.settings.OAuthClientID = loaded.OAuthClientID
	s.settings.OAuthVerificationBase = loaded.OAuthVerificationBase
	s.settings.OAuthGrant = loaded.OAuthGrant
	s.settings.OAuthAuthorizeURL = loaded.OAuthAuthorizeURL
	s.settings.OAuthTokenURL = loaded.OAuthTokenURL
	s.settings.OAuthTokenStyle = loaded.OAuthTokenStyle
	s.settings.OAuthScopes = loaded.OAuthScopes
	s.settings.OAuthStateIsVerifier = loaded.OAuthStateIsVerifier
	s.settings.OAuthRedirectURI = loaded.OAuthRedirectURI
	s.settings.PathTemplate = loaded.PathTemplate
	s.settings.ModelsPath = loaded.ModelsPath
	s.settings.ExtraHeaders = loaded.ExtraHeaders
	s.settings.ForwardHeaders = loaded.ForwardHeaders
	s.settings.RetryStatuses = loaded.RetryStatuses
	s.settings.ForceStream = loaded.ForceStream
}

func (s *SidecarOverrideStore) save() {
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}

func (s *SidecarOverrideStore) get() SidecarSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *SidecarOverrideStore) update(update SidecarSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = update
	s.save()
}

// WithSidecarStore sets the sidecar override store on the handler.
func WithSidecarStore(store *SidecarOverrideStore) Option {
	return func(h *Handler) {
		h.sidecarStore = store
	}
}

// GetSidecarStatus returns the current sidecar settings and live status.
func (h *Handler) GetSidecarStatus(c *echo.Context) error {
	settings := h.sidecarStore.get()
	// Bind IPs are normally supplied via AURORA_SIDECAR_BIND_IPS and consumed
	// by the container entrypoint. Fall back to that env var so operators see
	// the live configuration even before saving anything from the dashboard.
	if len(settings.BindIPs) == 0 {
		raw := strings.TrimSpace(os.Getenv("AURORA_SIDECAR_BIND_IPS"))
		if raw != "" {
			for _, ip := range strings.Split(raw, ",") {
				if v := strings.TrimSpace(ip); v != "" {
					settings.BindIPs = append(settings.BindIPs, v)
				}
			}
		}
	}
	// Build proxy list from bind_ips.
	proxies := make([]SidecarProxy, 0, len(settings.BindIPs))
	basePort := 8981
	for i, ip := range settings.BindIPs {
		port := basePort + i
		proxies = append(proxies, SidecarProxy{
			IP:      ip,
			Port:    port,
			Enabled: settings.Enabled,
		})
	}
	return c.JSON(http.StatusOK, SidecarStatus{
		Running:  settings.Enabled,
		Settings: settings,
		Proxies:  proxies,
	})
}

// UpdateSidecarSettings applies new sidecar configuration.
func (h *Handler) UpdateSidecarSettings(c *echo.Context) error {
	h.mutationMu.Lock()
	defer h.mutationMu.Unlock()

	var req SidecarSettings
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	// Validate port range.
	if req.Port < 1 || req.Port > 65535 {
		req.Port = 8090
	}
	if req.MaxAttempts < 1 || req.MaxAttempts > 10 {
		req.MaxAttempts = 4
	}
	if req.RetryDelayMs < 100 || req.RetryDelayMs > 5000 {
		req.RetryDelayMs = 750
	}

	h.sidecarStore.update(req)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
