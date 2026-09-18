package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/providers/oauth"
)

// OAuthHandler manages OAuth device flow for providers.
type OAuthHandler struct {
	registry *oauth.Registry
}

// NewOAuthHandler creates a new OAuth admin handler.
func NewOAuthHandler(registry *oauth.Registry) *OAuthHandler {
	return &OAuthHandler{registry: registry}
}

// RegisterOAuthRoutes mounts the OAuth admin API routes.
func (h *OAuthHandler) RegisterOAuthRoutes(g RouteRegistrar) {
	g.POST("/oauth/:provider/start", h.StartDeviceFlow)
	g.POST("/oauth/:provider/poll", h.PollToken)
	g.GET("/oauth/:provider/status", h.TokenStatus)
	g.DELETE("/oauth/:provider/token", h.ClearToken)
}

// StartDeviceFlowRequest is the request body for starting a device flow.
type StartDeviceFlowRequest struct {
	CallbackURL string `json:"callback_url,omitempty"`
}

// StartDeviceFlowResponse is the response from starting a device flow.
type StartDeviceFlowResponse struct {
	UserCode                string `json:"user_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// StartDeviceFlow initiates the OAuth 2.0 device authorization grant.
// POST /admin/api/v1/oauth/:provider/start
func (h *OAuthHandler) StartDeviceFlow(c echo.Context) error {
	providerName := c.PathParam("provider")
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
		VerificationURIComplete: dc.VerificationURIComplete,
		ExpiresIn:               dc.ExpiresIn,
		Interval:                dc.Interval,
	})
}

// backgroundPoll polls the token endpoint until the user authorizes or the code expires.
func (h *OAuthHandler) backgroundPoll(providerName string, mgr *oauth.Manager, deviceCode string, interval time.Duration, expiresAt time.Time) {
	token, err := mgr.PollForToken(deviceCode, interval, expiresAt)
	if err != nil {
		return
	}
	if err := mgr.SaveTokenFromResponse(token); err != nil {
		return
	}
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

// PollToken manually polls the token endpoint once.
// POST /admin/api/v1/oauth/:provider/poll
func (h *OAuthHandler) PollToken(c echo.Context) error {
	providerName := c.PathParam("provider")
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
	HasToken   bool      `json:"has_token"`
	Expired    bool      `json:"expired"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	Email      string    `json:"email,omitempty"`
	Server     string    `json:"server,omitempty"`
}

// TokenStatus returns the current OAuth token status for a provider.
// GET /admin/api/v1/oauth/:provider/status
func (h *OAuthHandler) TokenStatus(c echo.Context) error {
	providerName := c.PathParam("provider")
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
		HasToken:  true,
		Expired:   time.Now().After(info.ExpiresAt),
		ExpiresAt: info.ExpiresAt,
		Email:     info.Email,
		Server:    info.Server,
	})
}

// ClearToken deletes the stored OAuth token for a provider.
// DELETE /admin/api/v1/oauth/:provider/token
func (h *OAuthHandler) ClearToken(c echo.Context) error {
	providerName := c.PathParam("provider")
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
