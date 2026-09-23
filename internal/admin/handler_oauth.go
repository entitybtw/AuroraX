package admin

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/providers/oauth"
)

// OAuthHandler manages OAuth device flow for providers.
type OAuthHandler struct {
	registry         *oauth.Registry
	verificationBase string
	// baseFunc, when set, overrides verificationBase (reads live sidecar
	// settings so extension apply updates the origin without restart).
	baseFunc func() string
}

// NewOAuthHandler creates a new OAuth admin handler.
func NewOAuthHandler(registry *oauth.Registry) *OAuthHandler {
	return &OAuthHandler{registry: registry}
}

// RegisterOAuthRoutes mounts the OAuth admin API routes.
func (h *OAuthHandler) RegisterOAuthRoutes(g RouteRegistrar) {
	g.GET("/oauth/providers", h.AllProvidersStatus)
	g.POST("/oauth/:provider/start", h.StartDeviceFlow)
	g.POST("/oauth/:provider/poll", h.PollToken)
	g.GET("/oauth/:provider/status", h.TokenStatus)
	g.POST("/oauth/:provider/refresh", h.RefreshToken)
	g.POST("/oauth/refresh", h.RefreshAllTokens)
	g.DELETE("/oauth/:provider/token", h.ClearToken)
}

// StartDeviceFlowResponse is the response from starting a device flow.
type StartDeviceFlowResponse struct {
	UserCode                string `json:"user_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	VerificationBase        string `json:"verification_base,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// SetVerificationBase injects the extension-driven origin used to expand
// relative verification URIs (no provider-specific hardcode in the UI).
func (h *OAuthHandler) SetVerificationBase(base string) {
	h.verificationBase = strings.TrimRight(strings.TrimSpace(base), "/")
}

// WithVerificationBaseFunc wires a live resolver for the extension-supplied
// verification origin (relative device-flow URIs).
func (h *OAuthHandler) WithVerificationBaseFunc(fn func() string) {
	h.baseFunc = fn
}

func (h *OAuthHandler) currentVerificationBase() string {
	if h.baseFunc != nil {
		if v := strings.TrimRight(strings.TrimSpace(h.baseFunc()), "/"); v != "" {
			return v
		}
	}
	return h.verificationBase
}

// verificationBaseFor expands a relative verification URI. Absolute http(s)
// URIs are returned unchanged.
func (h *OAuthHandler) verificationBaseFor(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri
	}
	base := h.currentVerificationBase()
	if base == "" {
		return uri
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	return base + uri
}

// StartDeviceFlow initiates the OAuth 2.0 device authorization grant.
// POST /admin/api/v1/oauth/:provider/start
func (h *OAuthHandler) StartDeviceFlow(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}

	mgr := h.registry.Get(providerName)
	if mgr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "provider not found or OAuth not configured",
		})
	}

	dc, err := mgr.StartDeviceFlow()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to start device flow: " + err.Error(),
		})
	}

	// Start polling in background
	go h.backgroundPoll(providerName, mgr, dc.DeviceCode, time.Duration(dc.Interval)*time.Second, time.Now().Add(time.Duration(dc.ExpiresIn)*time.Second))

	return c.JSON(http.StatusOK, StartDeviceFlowResponse{
		UserCode:                dc.UserCode,
		VerificationURIComplete: h.verificationBaseFor(dc.VerificationURIComplete),
		VerificationBase:        h.currentVerificationBase(),
		ExpiresIn:               dc.ExpiresIn,
		Interval:                dc.Interval,
	})
}

func (h *OAuthHandler) backgroundPoll(providerName string, mgr *oauth.Manager, deviceCode string, interval time.Duration, expiresAt time.Time) {
	log.Printf("oauth: background polling started provider=%s", providerName)
	token, err := mgr.PollForToken(deviceCode, interval, expiresAt)
	if err != nil {
		log.Printf("oauth: background polling failed provider=%s error=%v", providerName, err)
		return
	}
	if err := mgr.SaveTokenFromResponse(token); err != nil {
		log.Printf("oauth: failed to save token provider=%s error=%v", providerName, err)
		return
	}
	log.Printf("oauth: background polling succeeded, token saved provider=%s", providerName)
}

// PollTokenRequest is the request body for manual polling.
type PollTokenRequest struct {
	DeviceCode string `json:"device_code"`
	Interval   int    `json:"interval"`
	ExpiresIn  int    `json:"expires_in"`
}

// PollTokenResponse is the response from polling.
type PollTokenResponse struct {
	Status      string `json:"status"`
	AccessToken string `json:"access_token,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PollToken polls the token endpoint once.
// POST /admin/api/v1/oauth/:provider/poll
func (h *OAuthHandler) PollToken(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}

	mgr := h.registry.Get(providerName)
	if mgr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "provider not found or OAuth not configured",
		})
	}

	var req PollTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.DeviceCode == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "device_code is required"})
	}

	interval := time.Duration(req.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
	if req.ExpiresIn == 0 {
		expiresAt = time.Now().Add(10 * time.Minute)
	}

	token, pending, err := mgr.ExchangeTokenPublic(req.DeviceCode, interval, expiresAt)
	if err != nil {
		return c.JSON(http.StatusOK, PollTokenResponse{
			Status: "error",
			Error:  err.Error(),
		})
	}
	if pending {
		return c.JSON(http.StatusOK, PollTokenResponse{
			Status: "pending",
		})
	}

	if err := mgr.SaveTokenFromResponse(token); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to save token: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, PollTokenResponse{
		Status:      "authorized",
		AccessToken: token.AccessToken,
	})
}

// TokenStatusResponse shows the current token state.
type TokenStatusResponse struct {
	HasToken    bool      `json:"has_token"`
	Expired     bool      `json:"expired"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Email       string    `json:"email,omitempty"`
	AccountID   string    `json:"account_id,omitempty"`
	Server      string    `json:"server,omitempty"`
	ProviderName string  `json:"provider_name,omitempty"`
}

// OAuthProviderStatus is a single provider's OAuth status for bulk listing.
type OAuthProviderStatus struct {
	Name      string `json:"name"`
	HasToken  bool   `json:"has_token"`
	Expired   bool   `json:"expired"`
	Email     string `json:"email,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

// TokenStatus returns the current OAuth token status for a provider.
// GET /admin/api/v1/oauth/:provider/status
func (h *OAuthHandler) TokenStatus(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}

	mgr := h.registry.Get(providerName)
	if mgr == nil {
		return c.JSON(http.StatusOK, TokenStatusResponse{
			HasToken: false,
		})
	}

	info := mgr.TokenInfo()
	if info == nil {
		return c.JSON(http.StatusOK, TokenStatusResponse{
			HasToken: false,
		})
	}

	return c.JSON(http.StatusOK, TokenStatusResponse{
		HasToken:    true,
		Expired:     time.Now().After(info.ExpiresAt),
		ExpiresAt:   info.ExpiresAt,
		Email:       info.Email,
		AccountID:   info.AccountID,
		Server:      info.Server,
		ProviderName: providerName,
	})
}

// RefreshToken forces a refresh of the stored OAuth token for a provider.
// POST /admin/api/v1/oauth/:provider/refresh
func (h *OAuthHandler) RefreshToken(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}

	mgr := h.registry.Get(providerName)
	if mgr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "provider not found or OAuth not configured",
		})
	}
	if !mgr.HasToken() {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "no token stored for provider",
		})
	}

	if err := mgr.RefreshToken(); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "failed to refresh token: " + err.Error(),
		})
	}

	info := mgr.TokenInfo()
	resp := TokenStatusResponse{HasToken: true, ProviderName: providerName}
	if info != nil {
		resp.Expired = time.Now().After(info.ExpiresAt)
		resp.ExpiresAt = info.ExpiresAt
		resp.Email = info.Email
		resp.AccountID = info.AccountID
		resp.Server = info.Server
	}
	return c.JSON(http.StatusOK, resp)
}

// RefreshAllResult reports the outcome of refreshing one provider.
type RefreshAllResult struct {
	Name      string `json:"name"`
	Refreshed bool   `json:"refreshed"`
	Error     string `json:"error,omitempty"`
}

// RefreshAllTokens forces a refresh of every stored OAuth token.
// POST /admin/api/v1/oauth/refresh
func (h *OAuthHandler) RefreshAllTokens(c *echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []RefreshAllResult{})
	}
	results := make([]RefreshAllResult, 0)
	for name, mgr := range h.registry.AllManagers() {
		if !mgr.HasToken() {
			continue
		}
		if err := mgr.RefreshToken(); err != nil {
			results = append(results, RefreshAllResult{Name: name, Error: err.Error()})
			continue
		}
		results = append(results, RefreshAllResult{Name: name, Refreshed: true})
	}
	return c.JSON(http.StatusOK, results)
}

// ClearToken deletes the stored OAuth token for a provider.
// DELETE /admin/api/v1/oauth/:provider/token
func (h *OAuthHandler) ClearToken(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}

	mgr := h.registry.Get(providerName)
	if mgr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "provider not found or OAuth not configured",
		})
	}

	if err := mgr.ClearToken(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to clear token: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
}

// AllProvidersStatus returns OAuth status for all registered providers.
// GET /admin/api/v1/oauth/providers
func (h *OAuthHandler) AllProvidersStatus(c *echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []OAuthProviderStatus{})
	}
	all := h.registry.AllManagers()
	result := make([]OAuthProviderStatus, 0, len(all))
	for name, mgr := range all {
		info := mgr.TokenInfo()
		if info == nil {
			result = append(result, OAuthProviderStatus{Name: name, HasToken: false})
			continue
		}
		result = append(result, OAuthProviderStatus{
			Name:      name,
			HasToken:  true,
			Expired:   time.Now().After(info.ExpiresAt),
			Email:     info.Email,
			AccountID: info.AccountID,
		})
	}
	return c.JSON(http.StatusOK, result)
}
