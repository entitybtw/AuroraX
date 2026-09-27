package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"aurora/internal/externalauth"
)

// ExternalAuthHandler proxies extension auth flows to loaded auth addons.
// The gateway ships no grant flows of its own: every route forwards one
// generic call over the bridge and relays the addon's JSON reply. While no
// auth addon is loaded every route answers 404.
type ExternalAuthHandler struct {
	bridge *externalauth.Bridge
}

// NewExternalAuthHandler creates the external auth admin proxy.
func NewExternalAuthHandler(bridge *externalauth.Bridge) *ExternalAuthHandler {
	return &ExternalAuthHandler{bridge: bridge}
}

// RegisterExternalAuthRoutes mounts the external auth admin API routes.
func (h *ExternalAuthHandler) RegisterExternalAuthRoutes(g RouteRegistrar) {
	g.GET("/external-auth/providers", h.gate(h.AllProvidersStatus))
	g.POST("/external-auth/:provider/start", h.gate(h.StartDeviceFlow))
	g.POST("/external-auth/:provider/poll", h.gate(h.PollToken))
	g.GET("/external-auth/:provider/status", h.gate(h.TokenStatus))
	g.POST("/external-auth/:provider/refresh", h.gate(h.RefreshToken))
	g.POST("/external-auth/refresh", h.gate(h.RefreshAllTokens))
	g.DELETE("/external-auth/:provider/token", h.gate(h.ClearToken))
	// Authorization-code + PKCE (extension-driven).
	g.GET("/external-auth/:provider/flow", h.gate(h.FlowInfo))
	g.POST("/external-auth/:provider/authorize", h.gate(h.StartAuthorize))
	g.POST("/external-auth/:provider/authorize/complete", h.gate(h.CompleteAuthorize))
}

// enabled reports whether at least one auth addon is loaded.
func (h *ExternalAuthHandler) enabled() bool {
	return h.bridge != nil && h.bridge.Enabled()
}

// gate wraps a handler so it only runs while an auth addon is loaded.
func (h *ExternalAuthHandler) gate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !h.enabled() {
			return disabledError(c)
		}
		return next(c)
	}
}

func disabledError(c *echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]string{
		"error": "external auth not enabled: apply an extension that ships an auth addon",
	})
}

// relay forwards a single call (or a merged list call) to the auth addons
// and writes their reply. A Go-level addon error maps to 502, a JSON object
// carrying "error" without a "status" field maps to 400, and everything else
// relays as 200.
func (h *ExternalAuthHandler) relay(c *echo.Context, method string, args any, list bool) error {
	if !h.enabled() {
		return disabledError(c)
	}
	var (
		raw string
		err error
	)
	if list {
		raw, err = h.bridge.CallList(method, args)
	} else {
		raw, err = h.bridge.Call(method, args)
	}
	if err != nil {
		if errors.Is(err, externalauth.ErrDisabled) {
			return disabledError(c)
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "auth addon returned an invalid response",
		})
	}
	if obj, ok := payload.(map[string]any); ok {
		if msg, _ := obj["error"].(string); msg != "" {
			if _, hasStatus := obj["status"]; !hasStatus {
				return c.JSON(http.StatusBadRequest, obj)
			}
		}
	}
	return c.JSON(http.StatusOK, payload)
}

// providerArgs builds the call payload: the provider path parameter merged
// with the raw request body (addons pick the fields they need).
func providerArgs(c *echo.Context) map[string]any {
	args := map[string]any{"provider": c.Param("provider")}
	if c.Request() != nil && c.Request().ContentLength > 0 {
		var body map[string]any
		if err := c.Bind(&body); err == nil && body != nil {
			for k, v := range body {
				args[k] = v
			}
		}
	}
	return args
}

// StartDeviceFlow starts an addon-managed device flow.
// POST /admin/api/v1/external-auth/:provider/start
func (h *ExternalAuthHandler) StartDeviceFlow(c *echo.Context) error {
	return h.relay(c, "Start", providerArgs(c), false)
}

// PollToken polls an addon-managed device flow once.
// POST /admin/api/v1/external-auth/:provider/poll
func (h *ExternalAuthHandler) PollToken(c *echo.Context) error {
	return h.relay(c, "Poll", providerArgs(c), false)
}

// TokenStatus returns the stored token state for a provider.
// GET /admin/api/v1/external-auth/:provider/status
func (h *ExternalAuthHandler) TokenStatus(c *echo.Context) error {
	return h.relay(c, "TokenStatus", map[string]any{"provider": c.Param("provider")}, false)
}

// RefreshToken forces an addon-managed token refresh for a provider.
// POST /admin/api/v1/external-auth/:provider/refresh
func (h *ExternalAuthHandler) RefreshToken(c *echo.Context) error {
	return h.relay(c, "Refresh", map[string]any{"provider": c.Param("provider")}, false)
}

// RefreshAllTokens forces a refresh of every stored token.
// POST /admin/api/v1/external-auth/refresh
func (h *ExternalAuthHandler) RefreshAllTokens(c *echo.Context) error {
	return h.relay(c, "RefreshAll", map[string]any{}, true)
}

// ClearToken deletes the stored token for a provider.
// DELETE /admin/api/v1/external-auth/:provider/token
func (h *ExternalAuthHandler) ClearToken(c *echo.Context) error {
	return h.relay(c, "ClearToken", map[string]any{"provider": c.Param("provider")}, false)
}

// FlowInfo reports the active grant for the provider (addon-driven).
// GET /admin/api/v1/external-auth/:provider/flow
func (h *ExternalAuthHandler) FlowInfo(c *echo.Context) error {
	return h.relay(c, "FlowInfo", map[string]any{"provider": c.Param("provider")}, false)
}

// StartAuthorize creates an addon-managed PKCE session and returns the
// browser authorize URL.
// POST /admin/api/v1/external-auth/:provider/authorize
func (h *ExternalAuthHandler) StartAuthorize(c *echo.Context) error {
	return h.relay(c, "StartAuthorize", providerArgs(c), false)
}

// CompleteAuthorize exchanges a pasted/redirected authorization code.
// POST /admin/api/v1/external-auth/:provider/authorize/complete
func (h *ExternalAuthHandler) CompleteAuthorize(c *echo.Context) error {
	return h.relay(c, "CompleteAuthorize", providerArgs(c), false)
}

// AllProvidersStatus returns token status for every provider the auth
// addons know about.
// GET /admin/api/v1/external-auth/providers
func (h *ExternalAuthHandler) AllProvidersStatus(c *echo.Context) error {
	return h.relay(c, "Providers", map[string]any{}, true)
}
