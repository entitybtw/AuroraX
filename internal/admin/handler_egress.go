package admin

import (
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/egress"
)

// egressStatusResponse is the payload behind GET /egress: one entry per
// provider that has exits configured, so the dashboard can render the
// drill-down without joining several endpoints.
type egressStatusResponse struct {
	Providers []egress.ProviderEgressStatus `json:"providers"`
}

// egressExitRequest names the exit an operator is turning on or off. The
// exit lives in the body rather than the path because exit names carry
// characters (":", "//") that do not survive round-tripping through a URL
// segment reliably.
type egressExitRequest struct {
	Exit string `json:"exit"`
}

// egressToggleResponse echoes the new state of the exit plus the provider's
// full status so the dashboard can repaint in one round trip.
type egressToggleResponse struct {
	Provider string                       `json:"provider"`
	Exit     string                       `json:"exit"`
	Disabled bool                         `json:"disabled"`
	Status   *egress.ProviderEgressStatus `json:"status,omitempty"`
}

// egressProviderNames is every provider the status panel could show: static
// providers, dashboard-created ones and any provider the registry already
// holds exits for.
func (h *Handler) egressProviderNames() []string {
	if h == nil {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(name string) {
		if name = strings.TrimSpace(name); name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, cfg := range h.configuredProviders {
		add(cfg.Name)
	}
	if h.providerOverrides != nil {
		for _, o := range h.providerOverrides.list() {
			if o.IsEnabled() {
				add(o.Name)
			}
		}
	}
	if h.egress != nil {
		for _, name := range h.egress.Providers() {
			add(name)
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ListEgressStatus handles GET /admin/api/v1/egress
func (h *Handler) ListEgressStatus(c *echo.Context) error {
	if h.egress == nil {
		return handleError(c, featureUnavailableError("egress status is unavailable"))
	}
	out := make([]egress.ProviderEgressStatus, 0)
	for _, name := range h.egressProviderNames() {
		if st, ok := h.egress.Status(name); ok {
			out = append(out, st)
		}
	}
	return c.JSON(http.StatusOK, egressStatusResponse{Providers: out})
}

// GetEgressStatus handles GET /admin/api/v1/egress/:provider
func (h *Handler) GetEgressStatus(c *echo.Context) error {
	if h.egress == nil {
		return handleError(c, featureUnavailableError("egress status is unavailable"))
	}
	provider := strings.TrimSpace(c.Param("provider"))
	st, ok := h.egress.Status(provider)
	if !ok {
		return handleError(c, core.NewNotFoundError("provider \""+provider+"\" has no configured exits"))
	}
	return c.JSON(http.StatusOK, st)
}

// DisableEgressExit handles POST /admin/api/v1/egress/:provider/disable
func (h *Handler) DisableEgressExit(c *echo.Context) error {
	return h.setEgressExitDisabled(c, true)
}

// EnableEgressExit handles POST /admin/api/v1/egress/:provider/enable
func (h *Handler) EnableEgressExit(c *echo.Context) error {
	return h.setEgressExitDisabled(c, false)
}

// setEgressExitDisabled records the operator's choice on the provider
// override and re-reads the egress configuration. The disabled list is
// persisted, so an exit the operator turned off stays off across a restart.
func (h *Handler) setEgressExitDisabled(c *echo.Context, disabled bool) error {
	if h.egress == nil || h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("egress management is unavailable"))
	}

	provider := strings.TrimSpace(c.Param("provider"))
	var req egressExitRequest
	if err := c.Bind(&req); err != nil {
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: "invalid egress payload: " + err.Error(),
		})
	}
	exit := strings.TrimSpace(req.Exit)
	if provider == "" || exit == "" {
		return handleError(c, core.NewInvalidRequestError("provider and exit are required", nil))
	}

	updated, ok := h.overrideForProvider(provider)
	if !ok {
		return handleError(c, core.NewNotFoundError("provider \""+provider+"\" is not configured"))
	}

	next := make([]string, 0, len(updated.EgressDisabled)+1)
	for _, name := range updated.EgressDisabled {
		if name = strings.TrimSpace(name); name != "" && name != exit {
			next = append(next, name)
		}
	}
	if disabled {
		next = append(next, exit)
	}
	updated.EgressDisabled = next
	h.providerOverrides.upsert(updated)

	// A toggle must take effect now: the registry is re-read in place instead
	// of waiting for a full runtime rebuild, which would also re-fetch models.
	if h.egressSyncer != nil {
		h.egressSyncer()
	}

	resp := egressToggleResponse{Provider: provider, Exit: exit, Disabled: disabled}
	if st, ok := h.egress.Status(provider); ok {
		resp.Status = &st
	}
	return c.JSON(http.StatusOK, resp)
}

// overrideForProvider returns the stored override for name, seeded from the
// static configuration when the dashboard has never edited that provider.
func (h *Handler) overrideForProvider(name string) (ProviderOverride, bool) {
	if h.providerOverrides == nil {
		return ProviderOverride{}, false
	}
	if existing, ok := h.providerOverrides.get(name); ok {
		return existing, true
	}
	if sp := h.findStaticProvider(name); sp != nil {
		return seededStaticOverride(name, sp), true
	}
	return ProviderOverride{}, false
}
