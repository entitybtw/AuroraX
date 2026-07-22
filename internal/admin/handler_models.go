package admin

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/model_overrides"
	"aurora/internal/providers"
)

type modelAccessResponse struct {
	Selector         string                   `json:"selector"`
	DefaultEnabled   bool                     `json:"default_enabled"`
	EffectiveEnabled bool                     `json:"effective_enabled"`
	UserPaths        []string                 `json:"user_paths,omitempty"`
	Override         *modeloverrides.Override `json:"override,omitempty"`
}

type modelInventoryResponse struct {
	providers.ModelWithProvider
	Access modelAccessResponse `json:"access"`
}

// ListModels handles GET /admin/api/v1/models
// Supports optional ?category= query param for filtering by model category.
//
// @Summary      List all registered models with provider info and access state
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        category    query     string  false  "Filter by model category"
// @Success      200  {array}  modelInventoryResponse
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/models [get]
func (h *Handler) ListModels(c *echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []modelInventoryResponse{})
	}

	cat := core.ModelCategory(strings.TrimSpace(c.QueryParam("category")))
	if cat != "" && cat != core.CategoryAll {
		if !isValidCategory(cat) {
			return handleError(c, core.NewInvalidRequestError("invalid category: "+string(cat), nil))
		}
	}

	var models []providers.ModelWithProvider
	if cat != "" && cat != core.CategoryAll {
		models = h.registry.ListModelsWithProviderByCategory(cat)
	} else {
		models = h.registry.ListModelsWithProvider()
	}

	if models == nil {
		models = []providers.ModelWithProvider{}
	}

	poolMembers := h.collectPoolMembers()
	models = h.buildPoolAwareModelInventory(models, poolMembers)

	access := h.modelAccessResolver()
	response := make([]modelInventoryResponse, 0, len(models))
	for _, model := range models {
		selector := core.ModelSelector{
			Provider: strings.TrimSpace(model.ProviderName),
			Model:    strings.TrimSpace(model.Model.ID),
		}
		response = append(response, modelInventoryResponse{
			ModelWithProvider: model,
			Access:            access(selector),
		})
	}

	return c.JSON(http.StatusOK, response)
}

// collectPoolMembers returns a map of provider name → pool name for every
// provider that belongs to a configured pool.
func (h *Handler) collectPoolMembers() map[string]string {
	members := map[string]string{}
	if h.pools == nil {
		return members
	}
	for _, p := range h.pools.Snapshot() {
		for _, member := range p.Members {
			members[member.ProviderName] = p.Name
		}
	}
	return members
}

// buildPoolAwareModelInventory filters out models that belong to pool-member
// providers and replaces them with a single pool-qualified entry per unique
// model ID. Only the Selector is pool-prefixed — Model.ID keeps the raw
// model ID so that frontends that prepend provider_name do not double-prefix.
func (h *Handler) buildPoolAwareModelInventory(models []providers.ModelWithProvider, poolMembers map[string]string) []providers.ModelWithProvider {
	result := make([]providers.ModelWithProvider, 0, len(models))
	seen := map[string]bool{}

	for _, model := range models {
		modelID := strings.TrimSpace(model.Model.ID)
		poolName, isMember := poolMembers[model.ProviderName]
		if !isMember {
			result = append(result, model)
			continue
		}
		if seen[modelID] {
			continue
		}
		seen[modelID] = true
		poolEntry := model
		poolEntry.Selector = poolName + "/" + modelID
		poolEntry.ProviderName = poolName
		poolEntry.Model.OwnedBy = poolName
		result = append(result, poolEntry)
	}
	return result
}

// modelAccessResolver returns a function that produces the access view for a
// given selector. When model overrides are configured the resolver consults
// the service for effective state; otherwise every model is reported as
// default-on.
func (h *Handler) modelAccessResolver() func(core.ModelSelector) modelAccessResponse {
	if h.modelOverrides == nil {
		return func(selector core.ModelSelector) modelAccessResponse {
			return modelAccessResponse{
				Selector:         selector.QualifiedModel(),
				DefaultEnabled:   true,
				EffectiveEnabled: true,
			}
		}
	}
	return func(selector core.ModelSelector) modelAccessResponse {
		effective := h.modelOverrides.EffectiveState(selector)
		access := modelAccessResponse{
			Selector:         effective.Selector,
			DefaultEnabled:   effective.DefaultEnabled,
			EffectiveEnabled: effective.Enabled,
			UserPaths:        append([]string(nil), effective.UserPaths...),
		}
		if override, ok := h.modelOverrides.Get(selector.QualifiedModel()); ok && override != nil {
			overrideCopy := *override
			access.Override = &overrideCopy
		}
		return access
	}
}

// isValidCategory returns true if cat is a recognized model category.
func isValidCategory(cat core.ModelCategory) bool {
	return slices.Contains(core.AllCategories(), cat)
}

// ListCategories handles GET /admin/api/v1/models/categories
//
// @Summary      List model categories with counts
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   providers.CategoryCount
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/models/categories [get]
func (h *Handler) ListCategories(c *echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []providers.CategoryCount{})
	}

	return c.JSON(http.StatusOK, h.registry.GetCategoryCounts())
}

// DashboardConfig handles GET /admin/api/v1/dashboard/config
//
// @Summary      Get dashboard runtime configuration
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  DashboardConfigResponse
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/dashboard/config [get]
func (h *Handler) DashboardConfig(c *echo.Context) error {
	return c.JSON(http.StatusOK, cloneDashboardRuntimeConfig(h.runtimeConfig))
}

func (h *Handler) AuthMe(c *echo.Context) error {
	authHeader := c.Request().Header.Get("Authorization")
	if h.masterKey != "" && authHeader != "" {
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(authHeader, bearerPrefix) {
			token := strings.TrimPrefix(authHeader, bearerPrefix)
			if subtle.ConstantTimeCompare([]byte(token), []byte(h.masterKey)) == 1 {
				return c.JSON(http.StatusOK, map[string]any{
					"user": map[string]any{
						"id":           "admin",
						"email":        "admin@aurora.local",
						"display_name": "Admin",
						"status":       "active",
						"provider":     "master_key",
					},
					"roles": []map[string]any{
						{ "id": "admin", "name": "Admin", "is_system": true },
					},
					"permissions": []map[string]any{
						{ "action": "read", "resource": "*", "effect": "allow" },
						{ "action": "write", "resource": "*", "effect": "allow" },
					},
				})
			}
		}
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]any{
		"identity_enabled": false,
		"oidc_providers":   []any{},
	})
}

// FeatureStatus handles GET /admin/api/v1/dashboard/features.
// It reports effective feature availability from the running server state and
// capability snapshot, not by re-reading config files.
func (h *Handler) FeatureStatus(c *echo.Context) error {
	cfg := cloneDashboardRuntimeConfig(h.runtimeConfig)
	return c.JSON(http.StatusOK, FeatureStatusResponse{
		Edition:      cfg.Edition,
		Capabilities: normalizeCapabilityMap(cfg.CapabilityMap),
		Features:     buildFeatureStatusSnapshots(cfg),
		ServerTime:   time.Now().UTC(),
	})
}
