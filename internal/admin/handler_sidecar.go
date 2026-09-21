package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
)

// SidecarSettings holds the runtime configuration for the free tier
// sidecar. The settings are persisted to a JSON file in the configs directory
// and can be read/written via the admin API.
type SidecarSettings struct {
	Enabled       bool              `json:"enabled"`
	Port          int               `json:"port"`
	InjectTools   bool              `json:"inject_tools"`
	DefaultAuth   string            `json:"default_auth"`
	UserAgent     string            `json:"user_agent"`
	BaseURL       string            `json:"base_url"`
	MaxAttempts   int               `json:"max_attempts"`
	RetryDelayMs  int               `json:"retry_delay_ms"`
	BindIPs       []string          `json:"bind_ips"`
	Proxies       []SidecarProxy    `json:"proxies"`
}

// SidecarProxy represents a single per-IP CONNECT proxy entry.
type SidecarProxy struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

// SidecarStatus is the live status returned by the GET endpoint.
type SidecarStatus struct {
	Running  bool             `json:"running"`
	Settings SidecarSettings  `json:"settings"`
	Proxies  []SidecarProxy   `json:"proxies"`
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
			Enabled:      true,
			Port:         8090,
			InjectTools:  true,
			DefaultAuth:  "Bearer public",
			BaseURL:      "https://opencode.ai/zen/v1",
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
	// Bind IPs are normally supplied via OPENCODE_ZEN_BIND_IPS and consumed by
	// the container entrypoint. Fall back to that env var so operators see the
	// live configuration even before saving anything from the dashboard.
	if len(settings.BindIPs) == 0 {
		if raw := strings.TrimSpace(os.Getenv("OPENCODE_ZEN_BIND_IPS")); raw != "" {
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
