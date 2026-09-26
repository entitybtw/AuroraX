package admin

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/providers/oauth"
)

// OAuthHandler manages OAuth device flow and authorization-code + PKCE for providers.
type OAuthHandler struct {
	registry         *oauth.Registry
	verificationBase string
	// baseFunc, when set, overrides verificationBase (reads live sidecar
	// settings so extension apply updates the origin without restart).
	baseFunc func() string
	// authCodes tracks in-flight PKCE authorize sessions keyed by state.
	authCodes *oauth.AuthCodeStore
	// authCodeConfigFunc, when set, supplies the current extension-driven
	// authorization-code config (authorize/token endpoints, client, scopes).
	authCodeConfigFunc func() oauth.AuthCodeConfig
	// flowFunc, when set, reports the active grant ("device" | "authorization_code").
	flowFunc func() string
	// enabledFunc, when set, gates every OAuth route: OAuth exists only while
	// an extension providing the oauth feature is applied. Disabled handlers
	// answer 404 so nothing OAuth-shaped is exposed without an extension.
	enabledFunc func() bool
}

// NewOAuthHandler creates a new OAuth admin handler.
func NewOAuthHandler(registry *oauth.Registry) *OAuthHandler {
	return &OAuthHandler{
		registry:  registry,
		authCodes: oauth.NewAuthCodeStore(),
	}
}

// RegisterOAuthRoutes mounts the OAuth admin API routes. When an enabled
// check is wired, every route answers 404 while the oauth feature is not
// provided by an applied extension.
func (h *OAuthHandler) RegisterOAuthRoutes(g RouteRegistrar) {
	g.GET("/oauth/providers", h.gate(h.AllProvidersStatus))
	g.POST("/oauth/:provider/start", h.gate(h.StartDeviceFlow))
	g.POST("/oauth/:provider/poll", h.gate(h.PollToken))
	g.GET("/oauth/:provider/status", h.gate(h.TokenStatus))
	g.POST("/oauth/:provider/refresh", h.gate(h.RefreshToken))
	g.POST("/oauth/refresh", h.gate(h.RefreshAllTokens))
	g.DELETE("/oauth/:provider/token", h.gate(h.ClearToken))
	// Authorization-code + PKCE (extension-driven).
	g.GET("/oauth/:provider/flow", h.gate(h.FlowInfo))
	g.POST("/oauth/:provider/authorize", h.gate(h.StartAuthorize))
	g.POST("/oauth/:provider/authorize/complete", h.gate(h.CompleteAuthorize))
}

// WithEnabledFunc wires the runtime capability check for the oauth feature.
// nil disables the gate (tests).
func (h *OAuthHandler) WithEnabledFunc(fn func() bool) {
	h.enabledFunc = fn
}

// oauthDisabledError is the 404 body returned when no extension provides
// the oauth feature.
func (h *OAuthHandler) oauthDisabledError(c *echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]string{
		"error": "oauth not enabled: apply an extension that provides the oauth feature",
	})
}

// gate wraps a handler so it only runs while the oauth feature is enabled.
func (h *OAuthHandler) gate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if h.enabledFunc != nil && !h.enabledFunc() {
			return h.oauthDisabledError(c)
		}
		return next(c)
	}
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

// WithAuthCodeConfigFunc wires a live resolver for authorization-code + PKCE
// config (extension-supplied endpoints; never hardcoded in core).
func (h *OAuthHandler) WithAuthCodeConfigFunc(fn func() oauth.AuthCodeConfig) {
	h.authCodeConfigFunc = fn
}

// WithFlowFunc wires a live resolver for the active OAuth grant
// ("device" or "authorization_code").
func (h *OAuthHandler) WithFlowFunc(fn func() string) {
	h.flowFunc = fn
}

// FlowInfo reports the active grant for the provider (extension-driven).
// GET /admin/api/v1/oauth/:provider/flow
func (h *OAuthHandler) FlowInfo(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	grant := "device"
	if h.flowFunc != nil {
		if v := strings.TrimSpace(h.flowFunc()); v != "" {
			grant = v
		}
	}
	mgr := h.registry.Get(providerName)
	hasAuthCode := false
	if mgr != nil {
		hasAuthCode = mgr.AuthCodeConfig().TokenURL != ""
	}
	return c.JSON(http.StatusOK, map[string]any{
		"provider":           providerName,
		"grant":              grant,
		"has_token":          mgr != nil && mgr.HasToken(),
		"authorization_code": hasAuthCode || strings.EqualFold(grant, "authorization_code"),
	})
}

// StartAuthorizeRequest is the body for starting an authorization-code flow.
type StartAuthorizeRequest struct {
	// RedirectURI optionally overrides the extension/default loopback redirect.
	RedirectURI string `json:"redirect_uri,omitempty"`
	// Scope optionally overrides the extension-supplied scope list.
	Scope string `json:"scope,omitempty"`
}

// StartAuthorizeResponse is returned after creating a PKCE session.
type StartAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
	RedirectURI  string `json:"redirect_uri"`
	ExpiresIn    int    `json:"expires_in"`
	Grant        string `json:"grant"`
}

// StartAuthorize creates a PKCE session and returns the browser authorize URL.
// POST /admin/api/v1/oauth/:provider/authorize
func (h *OAuthHandler) StartAuthorize(c *echo.Context) error {
	providerName := strings.TrimSpace(c.Param("provider"))
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider name is required"})
	}
	cfg, err := h.currentAuthCodeConfig(providerName)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var req StartAuthorizeRequest
	_ = c.Bind(&req) // optional body
	if v := strings.TrimSpace(req.RedirectURI); v != "" {
		cfg.RedirectURI = v
	}
	if v := strings.TrimSpace(req.Scope); v != "" {
		cfg.Scopes = v
	}

	if h.authCodes == nil {
		h.authCodes = oauth.NewAuthCodeStore()
	}
	start, err := h.authCodes.Start(cfg)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, StartAuthorizeResponse{
		AuthorizeURL: start.AuthorizeURL,
		State:        start.State,
		RedirectURI:  start.RedirectURI,
		ExpiresIn:    start.ExpiresIn,
		Grant:        "authorization_code",
	})
}

// CompleteAuthorizeRequest exchanges a pasted/redirected authorization code.
type CompleteAuthorizeRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
	// CodeAndState accepts "CODE#STATE" (manual redirect paste) as a single field.
	CodeAndState string `json:"code_and_state,omitempty"`
	// URL accepts a full callback URL containing code/state query params.
	URL string `json:"url,omitempty"`
}

// CompleteAuthorize exchanges the authorization code for tokens.
// POST /admin/api/v1/oauth/:provider/authorize/complete
func (h *OAuthHandler) CompleteAuthorize(c *echo.Context) error {
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
	if h.authCodes == nil {
		h.authCodes = oauth.NewAuthCodeStore()
	}

	var req CompleteAuthorizeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	code, state := strings.TrimSpace(req.Code), strings.TrimSpace(req.State)
	if code == "" || state == "" {
		code, state = splitCodeState(req.CodeAndState)
	}
	if (code == "" || state == "") && strings.TrimSpace(req.URL) != "" {
		code, state = parseCallbackURL(req.URL)
	}
	if code == "" || state == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "code and state are required (or code_and_state / url)",
		})
	}

	token, _, err := h.authCodes.Complete(state, code)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := mgr.SaveTokenFromResponse(token); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to save token: " + err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":       "authorized",
		"access_token": token.AccessToken,
	})
}

// currentAuthCodeConfig prefers live sidecar/extension PKCE settings, then the
// provider manager's attached config, then values from the authorize request.
func (h *OAuthHandler) currentAuthCodeConfig(providerName string) (oauth.AuthCodeConfig, error) {
	var cfg oauth.AuthCodeConfig
	if h.authCodeConfigFunc != nil {
		cfg = h.authCodeConfigFunc()
	}
	if mgr := h.registry.Get(providerName); mgr != nil {
		mcfg := mgr.AuthCodeConfig()
		if cfg.TokenURL == "" {
			cfg = mcfg
		} else {
			if cfg.ClientID == "" {
				cfg.ClientID = mcfg.ClientID
			}
			if cfg.AuthorizeURL == "" {
				cfg.AuthorizeURL = mcfg.AuthorizeURL
			}
			if cfg.TokenURL == "" {
				cfg.TokenURL = mcfg.TokenURL
			}
			if cfg.Scopes == "" {
				cfg.Scopes = mcfg.Scopes
			}
			if cfg.RedirectURI == "" {
				cfg.RedirectURI = mcfg.RedirectURI
			}
			if cfg.TokenStyle == "" {
				cfg.TokenStyle = mcfg.TokenStyle
			}
			cfg.StateIsVerifier = cfg.StateIsVerifier || mcfg.StateIsVerifier
		}
	}
	if cfg.ClientID == "" {
		if mgr := h.registry.Get(providerName); mgr != nil {
			// Manager stores client_id internally; AuthCodeConfig may be empty
			// when only device flow was configured.
			if info := mgr.TokenInfo(); info != nil && info.ClientID != "" {
				cfg.ClientID = info.ClientID
			}
		}
	}
	if cfg.AuthorizeURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" {
		return cfg, fmt.Errorf("authorization_code not configured (apply an extension that supplies authorize_url/token_url/client_id)")
	}
	return cfg, nil
}

// splitCodeState parses "CODE#STATE" (manual redirect paste).
func splitCodeState(s string) (code, state string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	parts := strings.SplitN(s, "#", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", ""
}

// parseCallbackURL extracts code and state from a full callback URL.
func parseCallbackURL(raw string) (code, state string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", ""
	}
	q := u.Query()
	code = strings.TrimSpace(q.Get("code"))
	state = strings.TrimSpace(q.Get("state"))
	// Some providers use fragment: .../callback#CODE#STATE or #code=...
	if code == "" || state == "" {
		if frag := u.Fragment; frag != "" {
			if c, st := splitCodeState(frag); c != "" {
				return c, st
			}
			fq, err := url.ParseQuery(frag)
			if err == nil {
				if code == "" {
					code = strings.TrimSpace(fq.Get("code"))
				}
				if state == "" {
					state = strings.TrimSpace(fq.Get("state"))
				}
			}
		}
	}
	return code, state
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
	HasToken     bool      `json:"has_token"`
	Expired      bool      `json:"expired"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Email        string    `json:"email,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	Server       string    `json:"server,omitempty"`
	ProviderName string    `json:"provider_name,omitempty"`
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
		HasToken:     true,
		Expired:      time.Now().After(info.ExpiresAt),
		ExpiresAt:    info.ExpiresAt,
		Email:        info.Email,
		AccountID:    info.AccountID,
		Server:       info.Server,
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
