package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/providers"
)

// ProviderOverride tracks a UI-created or UI-updated provider that may not exist
// in the static config. The fields mirror what users can set from the dashboard.
type ProviderOverride struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	BaseURL    string `json:"base_url"`
	APIVersion string `json:"api_version"`
	APIKey     string `json:"api_key"`
	Models     string `json:"models"`
}

// ProviderOverrideStore holds in-memory provider overrides created via the admin API.
type ProviderOverrideStore struct {
	mu        sync.Mutex
	overrides map[string]ProviderOverride // name -> override
}

// NewProviderOverrideStore creates shared provider override storage for admin CRUD
// and runtime refresh.
func NewProviderOverrideStore() *ProviderOverrideStore {
	return &ProviderOverrideStore{overrides: make(map[string]ProviderOverride)}
}

func (s *ProviderOverrideStore) get(name string) (ProviderOverride, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.overrides[name]
	return v, ok
}

func (s *ProviderOverrideStore) list() []ProviderOverride {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProviderOverride, 0, len(s.overrides))
	for _, v := range s.overrides {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *ProviderOverrideStore) upsert(v ProviderOverride) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides[v.Name] = v
}

func (s *ProviderOverrideStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overrides, name)
}

// RawConfigs returns a config-shaped snapshot suitable for provider runtime rebuilds.
func (s *ProviderOverrideStore) RawConfigs() map[string]config.RawProviderConfig {
	if s == nil {
		return nil
	}
	overrides := s.list()
	out := make(map[string]config.RawProviderConfig, len(overrides))
	for _, override := range overrides {
		name := strings.TrimSpace(override.Name)
		if name == "" {
			continue
		}
		out[name] = config.RawProviderConfig{
			Type:       strings.TrimSpace(override.Type),
			APIKey:     strings.TrimSpace(override.APIKey),
			BaseURL:    strings.TrimSpace(override.BaseURL),
			APIVersion: strings.TrimSpace(override.APIVersion),
			Models:     rawProviderModelsFromOverride(override.Models),
		}
	}
	return out
}

type providerCreateRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	BaseURL    string `json:"base_url"`
	APIVersion string `json:"api_version"`
	APIKey     string `json:"api_key"`
	Models     string `json:"models"`
}

type providerUpdateRequest struct {
	BaseURL    string `json:"base_url"`
	APIVersion string `json:"api_version"`
	APIKey     string `json:"api_key"`
	Models     string `json:"models"`
}

type providerModifyResponse struct {
	Message                string                `json:"message"`
	Provider               string                `json:"provider"`
	RuntimeApplied         bool                  `json:"runtime_applied"`
	RequiresRuntimeRefresh bool                  `json:"requires_runtime_refresh"`
	RuntimeRefresh         *RuntimeRefreshReport `json:"runtime_refresh,omitempty"`
	RuntimeRefreshError    string                `json:"runtime_refresh_error,omitempty"`
}

type runtimeApplyStatus struct {
	Applied bool
	Report  *RuntimeRefreshReport
	Error   string
}

func (h *Handler) applyRuntimeRefresh(c *echo.Context) runtimeApplyStatus {
	if h == nil || h.runtimeRefresher == nil {
		return runtimeApplyStatus{Applied: false, Error: "runtime refresher is unavailable"}
	}
	report, err := h.runtimeRefresher.RefreshRuntime(c.Request().Context())
	status := runtimeApplyStatus{Report: &report}
	if err != nil {
		status.Error = err.Error()
		return status
	}
	if report.Status == RuntimeRefreshStatusOK {
		status.Applied = true
		return status
	}
	status.Error = strings.TrimSpace(report.Status)
	if status.Error == "" {
		status.Error = "runtime refresh did not complete successfully"
	}
	return status
}

// CreateProvider handles POST /admin/api/v1/providers
func (h *Handler) CreateProvider(c *echo.Context) error {
	if h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("provider management is unavailable"))
	}

	var req providerCreateRequest
	if err := c.Bind(&req); err != nil {
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: "invalid provider payload: " + err.Error(),
		})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Name == "" {
		return handleError(c, core.NewInvalidRequestError("provider name is required", nil))
	}
	if req.Type == "" {
		return handleError(c, core.NewInvalidRequestError("provider type is required", nil))
	}

	h.providerOverrides.upsert(ProviderOverride{
		Name:       req.Name,
		Type:       req.Type,
		BaseURL:    strings.TrimSpace(req.BaseURL),
		APIVersion: strings.TrimSpace(req.APIVersion),
		APIKey:     strings.TrimSpace(req.APIKey),
		Models:     strings.TrimSpace(req.Models),
	})
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusCreated, providerModifyResponse{
		Message:                fmt.Sprintf("provider %q created", req.Name),
		Provider:               req.Name,
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// UpdateProvider handles PUT /admin/api/v1/providers/:name
func (h *Handler) UpdateProvider(c *echo.Context) error {
	if h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("provider management is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("provider name is required", nil))
	}

	existing, exists := h.providerOverrides.get(name)
	staticProvider := h.findStaticProvider(name)

	var req providerUpdateRequest
	if err := c.Bind(&req); err != nil {
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: "invalid provider payload: " + err.Error(),
		})
	}

	updated := ProviderOverride{
		Name: name,
		Type: existing.Type,
	}
	if exists {
		updated.Type = existing.Type
		updated.APIKey = req.APIKey
		updated.BaseURL = req.BaseURL
		updated.APIVersion = req.APIVersion
		updated.Models = req.Models
	} else if staticProvider != nil {
		updated.Type = staticProvider.Type
		updated.APIKey = req.APIKey
		updated.BaseURL = req.BaseURL
		updated.APIVersion = req.APIVersion
		updated.Models = req.Models
	}

	updated.BaseURL = strings.TrimSpace(req.BaseURL)
	updated.APIVersion = strings.TrimSpace(req.APIVersion)
	updated.APIKey = strings.TrimSpace(req.APIKey)
	updated.Models = strings.TrimSpace(req.Models)

	h.providerOverrides.upsert(updated)
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusOK, providerModifyResponse{
		Message:                fmt.Sprintf("provider %q updated", name),
		Provider:               name,
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// DeleteProvider handles DELETE /admin/api/v1/providers/:name
func (h *Handler) DeleteProvider(c *echo.Context) error {
	if h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("provider management is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("provider name is required", nil))
	}

	h.providerOverrides.remove(name)
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusOK, providerModifyResponse{
		Message:                fmt.Sprintf("provider %q deleted", name),
		Provider:               name,
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// ListProviderOverrides handles GET /admin/api/v1/providers/overrides
// Returns the list of UI-created provider overrides.
func (h *Handler) ListProviderOverrides(c *echo.Context) error {
	if h.providerOverrides == nil {
		return c.JSON(http.StatusOK, []ProviderOverride{})
	}
	return c.JSON(http.StatusOK, h.providerOverrides.list())
}

func (h *Handler) ProviderOverrideRawConfigs() map[string]config.RawProviderConfig {
	if h == nil || h.providerOverrides == nil {
		return nil
	}
	return h.providerOverrides.RawConfigs()
}

func rawProviderModelsFromOverride(models string) []config.RawProviderModel {
	ids := parseOverrideModels(models)
	if len(ids) == 0 {
		return nil
	}
	out := make([]config.RawProviderModel, 0, len(ids))
	for _, id := range ids {
		out = append(out, config.RawProviderModel{ID: id})
	}
	return out
}

func (h *Handler) findStaticProvider(name string) *providers.SanitizedProviderConfig {
	for i := range h.configuredProviders {
		if h.configuredProviders[i].Name == name {
			return &h.configuredProviders[i]
		}
	}
	return nil
}

// providerStatusWithSource enriches the provider status response with a config source indicator.
const (
	ConfigSourceConfigFile = "config_file"
	ConfigSourceEnvVar     = "env_var"
	ConfigSourceUI         = "ui"
	ConfigSourceStatic     = "static"
)
