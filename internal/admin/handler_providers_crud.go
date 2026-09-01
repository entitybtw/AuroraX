package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// BindIP optionally sets the local outbound IP for this provider's upstream requests.
	BindIP string `json:"bind_ip,omitempty"`
	// PoolOnly hides this provider's models from the public model list; it is
	// only reachable through a pool that lists it as a member.
	PoolOnly *bool `json:"pool_only,omitempty"`
	// Enabled controls whether the provider participates in the runtime.
	// A nil pointer (legacy entries written before this field) means enabled.
	Enabled *bool `json:"enabled,omitempty"`
	// UserAgent optionally overrides the User-Agent header sent to the upstream provider.
	UserAgent string `json:"user_agent,omitempty"`
	// AutoFetchModels controls whether the gateway calls /models to discover models.
	// A nil pointer means enabled (auto-fetch on).
	AutoFetchModels *bool `json:"auto_fetch_models,omitempty"`
}

// IsEnabled reports whether the override is active. Legacy overrides that
// predate the enabled field are treated as enabled.
func (o ProviderOverride) IsEnabled() bool {
	return o.Enabled == nil || *o.Enabled
}

// ProviderOverrideStore holds provider overrides created via the admin API.
// Persists to a JSON file so overrides survive restarts.
type ProviderOverrideStore struct {
	mu          sync.Mutex
	overrides   map[string]ProviderOverride // name -> override
	persistPath string
}

// NewProviderOverrideStore creates shared provider override storage for admin CRUD
// and runtime refresh. Loads existing overrides from the persist file if available.
func NewProviderOverrideStore() *ProviderOverrideStore {
	s := &ProviderOverrideStore{
		overrides:   make(map[string]ProviderOverride),
		persistPath: os.Getenv("AURORA_PROVIDER_OVERRIDES_PATH"),
	}
	if s.persistPath == "" {
		s.persistPath = "configs/provider-overrides.json"
	}
	s.load()
	return s
}

func (s *ProviderOverrideStore) load() {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return
	}
	var overrides []ProviderOverride
	if err := json.Unmarshal(data, &overrides); err != nil {
		return
	}
	for _, o := range overrides {
		if o.Name != "" {
			s.overrides[o.Name] = o
		}
	}
}

func (s *ProviderOverrideStore) save() {
	overrides := make([]ProviderOverride, 0, len(s.overrides))
	for _, v := range s.overrides {
		overrides = append(overrides, v)
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].Name < overrides[j].Name })
	data, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.persistPath, data, 0644)
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
	s.save()
}

func (s *ProviderOverrideStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overrides, name)
	s.save()
}

// RawConfigs returns a config-shaped snapshot suitable for provider runtime rebuilds.
// Disabled overrides are excluded so they never get built into the runtime.
func (s *ProviderOverrideStore) RawConfigs() map[string]config.RawProviderConfig {
	if s == nil {
		return nil
	}
	overrides := s.list()
	out := make(map[string]config.RawProviderConfig, len(overrides))
	for _, override := range overrides {
		if !override.IsEnabled() {
			continue
		}
		name := strings.TrimSpace(override.Name)
		if name == "" {
			continue
		}
		out[name] = config.RawProviderConfig{
			Type:            strings.TrimSpace(override.Type),
			APIKey:          strings.TrimSpace(override.APIKey),
			BaseURL:         strings.TrimSpace(override.BaseURL),
			APIVersion:      strings.TrimSpace(override.APIVersion),
			Models:          rawProviderModelsFromOverride(override.Models),
			BindIP:          strings.TrimSpace(override.BindIP),
			PoolOnly:        override.PoolOnly != nil && *override.PoolOnly,
			UserAgent:       strings.TrimSpace(override.UserAgent),
			AutoFetchModels: override.AutoFetchModels,
		}
	}
	return out
}

// DisabledNames returns the names of overrides that are explicitly disabled.
// Used by the runtime to drop static providers that have been toggled off.
func (s *ProviderOverrideStore) DisabledNames() []string {
	if s == nil {
		return nil
	}
	overrides := s.list()
	var out []string
	for _, override := range overrides {
		if !override.IsEnabled() {
			out = append(out, strings.TrimSpace(override.Name))
		}
	}
	return out
}

type providerCreateRequest struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	BaseURL         string `json:"base_url"`
	APIVersion      string `json:"api_version"`
	APIKey          string `json:"api_key"`
	Models          string `json:"models"`
	Enabled         *bool  `json:"enabled"`
	PoolOnly        *bool  `json:"pool_only"`
	UserAgent       string `json:"user_agent"`
	AutoFetchModels *bool  `json:"auto_fetch_models"`
}

type providerUpdateRequest struct {
	BaseURL         *string `json:"base_url"`
	APIVersion      *string `json:"api_version"`
	APIKey          *string `json:"api_key"`
	Models          *string `json:"models"`
	BindIP          *string `json:"bind_ip"`
	Enabled         *bool   `json:"enabled"`
	PoolOnly        *bool   `json:"pool_only"`
	UserAgent       *string `json:"user_agent"`
	AutoFetchModels *bool   `json:"auto_fetch_models"`
	// NewName renames the provider (both UI-created and static providers).
	NewName *string `json:"new_name"`
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
		Name:            req.Name,
		Type:            req.Type,
		BaseURL:         strings.TrimSpace(req.BaseURL),
		APIVersion:      strings.TrimSpace(req.APIVersion),
		APIKey:          strings.TrimSpace(req.APIKey),
		Models:          strings.TrimSpace(req.Models),
		Enabled:         boolPtrOrDefault(req.Enabled, true),
		PoolOnly:        req.PoolOnly,
		UserAgent:       strings.TrimSpace(req.UserAgent),
		AutoFetchModels: req.AutoFetchModels,
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
		updated = existing
	} else if staticProvider != nil {
		updated = ProviderOverride{
			Name:       name,
			Type:       staticProvider.Type,
			BaseURL:    staticProvider.BaseURL,
			APIVersion: staticProvider.APIVersion,
			Models:     strings.Join(staticProvider.Models, ", "),
			Enabled:    boolPtr(true),
		}
	}

	// Apply only the fields the caller provided so partial updates (e.g. a
	// bare {"enabled": false} toggle) do not clobber the existing config.
	if req.BaseURL != nil {
		updated.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.APIVersion != nil {
		updated.APIVersion = strings.TrimSpace(*req.APIVersion)
	}
	if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		updated.APIKey = strings.TrimSpace(*req.APIKey)
	}
	if req.Models != nil {
		updated.Models = strings.TrimSpace(*req.Models)
	}
	if req.BindIP != nil {
		updated.BindIP = strings.TrimSpace(*req.BindIP)
	}
	if req.Enabled != nil {
		updated.Enabled = boolPtr(*req.Enabled)
	}
	if req.PoolOnly != nil {
		updated.PoolOnly = boolPtr(*req.PoolOnly)
	}
	if req.UserAgent != nil {
		updated.UserAgent = strings.TrimSpace(*req.UserAgent)
	}
	if req.AutoFetchModels != nil {
		updated.AutoFetchModels = boolPtr(*req.AutoFetchModels)
	}

	// Rename support: the caller may supply a new_name. The new override is
	// stored under the new key; the old name is disabled so the runtime drops
	// it, and any UI-managed pools that referenced the old name are updated.
	if req.NewName != nil {
		newName := strings.TrimSpace(*req.NewName)
		if newName == "" {
			return handleError(c, core.NewInvalidRequestError("new_name must not be empty", nil))
		}
		if err := h.validateProviderRename(name, newName); err != nil {
			return handleError(c, core.NewInvalidRequestError(err.Error(), nil))
		}
		updated.Name = newName
		h.providerOverrides.upsert(updated)
		// Disable the old name so static/UI entries under it stop participating.
		h.providerOverrides.upsert(ProviderOverride{
			Name:    name,
			Type:    updated.Type,
			Enabled: boolPtr(false),
		})
		if h.poolWeights != nil {
			h.poolWeights.RenameMembers(name, newName)
			h.poolWeights.save()
		}
	} else {
		h.providerOverrides.upsert(updated)
	}
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

// validateProviderRename checks that newName is free to use: it must not collide
// with another configured provider instance or a pool name.
func (h *Handler) validateProviderRename(oldName, newName string) error {
	for name := range h.providerTypeByName() {
		if name == oldName {
			continue
		}
		if name == newName {
			return fmt.Errorf("provider %q already exists", newName)
		}
	}
	if h.pools != nil && h.pools.HasPool(newName) {
		return fmt.Errorf("pool name %q collides with the new provider name", newName)
	}
	return nil
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

func boolPtr(v bool) *bool { return &v }

func boolPtrOrDefault(v *bool, fallback bool) *bool {
	if v == nil {
		return boolPtr(fallback)
	}
	return boolPtr(*v)
}
