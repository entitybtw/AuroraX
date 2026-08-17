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

	cfg "aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/providers/pool"
)

// poolsResponse is the JSON shape returned by GET /admin/api/v1/pools.
type poolsResponse struct {
	Summary poolsSummary        `json:"summary"`
	Pools   []pool.PoolSnapshot `json:"pools"`
}

type poolsSummary struct {
	Total          int `json:"total"`
	HealthyMembers int `json:"healthy_members"`
	TotalMembers   int `json:"total_members"`
}

// PoolOverrideStore holds the full pool definitions managed through the admin
// API. Unlike a purely in-memory override, it persists to a JSON file
// (configs/pool-overrides.json by default) so UI-created pools survive
// restarts. On every runtime refresh the persisted pools are merged on top of
// the static config pools (ApplyToRawPools) and used to rebuild the live pool
// registry — no process restart required.
type PoolOverrideStore struct {
	mu          sync.Mutex
	pools       map[string]cfg.RawPoolConfig
	deleted     map[string]bool
	persistPath string
}

// NewPoolOverrideStore creates shared pool override storage. Loads any existing
// overrides from the persist file so dashboard-created pools reappear on boot.
func NewPoolOverrideStore() *PoolOverrideStore {
	s := &PoolOverrideStore{
		pools:       make(map[string]cfg.RawPoolConfig),
		deleted:     make(map[string]bool),
		persistPath: os.Getenv("AURORA_POOL_OVERRIDES_PATH"),
	}
	if s.persistPath == "" {
		s.persistPath = "configs/pool-overrides.json"
	}
	s.load()
	return s
}

type poolOverrideFile struct {
	Pools   map[string]cfg.RawPoolConfig `json:"pools"`
	Deleted []string                     `json:"deleted"`
}

func (s *PoolOverrideStore) load() {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return
	}
	var file poolOverrideFile
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}
	if file.Pools != nil {
		s.pools = file.Pools
	}
	for _, name := range file.Deleted {
		if n := strings.TrimSpace(name); n != "" {
			s.deleted[n] = true
		}
	}
}

func (s *PoolOverrideStore) save() {
	s.mu.Lock()
	deleted := make([]string, 0, len(s.deleted))
	for name := range s.deleted {
		deleted = append(deleted, name)
	}
	sort.Strings(deleted)
	file := poolOverrideFile{Pools: s.pools, Deleted: deleted}
	s.mu.Unlock()

	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.persistPath, raw, 0644)
}

// get returns a UI-managed pool definition (not including static config pools).
func (s *PoolOverrideStore) get(name string) (cfg.RawPoolConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.pools[strings.TrimSpace(name)]
	return v, ok
}

// has reports whether name is managed through the admin API (created or
// overridden via the dashboard). Deleted pools return false.
func (s *PoolOverrideStore) has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if s.deleted[name] {
		return false
	}
	_, ok := s.pools[name]
	return ok
}

// upsert records a pool definition managed by the admin API.
func (s *PoolOverrideStore) upsert(name string, raw cfg.RawPoolConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	delete(s.deleted, name)
	s.pools[name] = raw
}

// delete removes a pool managed by the admin API. When the pool also exists in
// the static config, it is recorded as deleted so the rebuild step drops it.
func (s *PoolOverrideStore) delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	delete(s.pools, name)
	s.deleted[name] = true
}

// RenameMembers replaces every reference to oldName in the members/weights of
// UI-managed pools with newName, so a renamed provider stays wired into pools.
func (s *PoolOverrideStore) RenameMembers(oldName, newName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for poolName, raw := range s.pools {
		changed := false
		members := make([]string, 0, len(raw.Members))
		for _, m := range raw.Members {
			if m == oldName {
				members = append(members, newName)
				changed = true
				continue
			}
			members = append(members, m)
		}
		if w, ok := raw.Weights[oldName]; ok {
			delete(raw.Weights, oldName)
			raw.Weights[newName] = w
			changed = true
		}
		if changed {
			raw.Members = members
			s.pools[poolName] = raw
		}
	}
}

// ApplyToRawPools merges UI-managed pool definitions on top of the static config
// pools: deleted pools are removed, managed pools replace or add entries.
func (s *PoolOverrideStore) ApplyToRawPools(base map[string]cfg.RawPoolConfig) map[string]cfg.RawPoolConfig {
	out := make(map[string]cfg.RawPoolConfig, len(base))
	for name, raw := range base {
		out[name] = cloneRawPoolConfig(raw)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range s.deleted {
		delete(out, name)
	}
	for name, raw := range s.pools {
		out[name] = raw
	}
	return out
}

// cloneRawPoolConfig deep-copies a RawPoolConfig so callers can mutate the
// result without affecting the source map.
func cloneRawPoolConfig(raw cfg.RawPoolConfig) cfg.RawPoolConfig {
	cp := cfg.RawPoolConfig{
		Strategy:    raw.Strategy,
		HealthAware: raw.HealthAware,
	}
	if raw.Members != nil {
		cp.Members = make([]string, len(raw.Members))
		copy(cp.Members, raw.Members)
	}
	if raw.Weights != nil {
		cp.Weights = make(map[string]int, len(raw.Weights))
		for k, v := range raw.Weights {
			cp.Weights[k] = v
		}
	}
	return cp
}

// ProviderPoolOption describes a provider that can be added as a pool member.
type ProviderPoolOption struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListPools handles GET /admin/api/v1/pools.
//
// Returns a snapshot of every configured pool, including the LB strategy and
// each member's current active-request count, lifetime totals, and health.
// Used by the dashboard's pool view and by external monitors.
func (h *Handler) ListPools(c *echo.Context) error {
	resp := poolsResponse{Pools: []pool.PoolSnapshot{}}

	if h.pools == nil {
		return c.JSON(http.StatusOK, resp)
	}

	snapshots := h.pools.Snapshot()
	for i := range snapshots {
		if h.poolWeights != nil && h.poolWeights.has(snapshots[i].Name) {
			snapshots[i].Source = "ui"
		} else {
			snapshots[i].Source = "config"
		}
	}
	resp.Pools = snapshots
	resp.Summary.Total = len(snapshots)
	for _, p := range snapshots {
		for _, m := range p.Members {
			resp.Summary.TotalMembers++
			if m.Healthy {
				resp.Summary.HealthyMembers++
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// PoolOptions handles GET /admin/api/v1/pools/options — returns the providers
// that can be used as pool members, with their resolved type.
func (h *Handler) PoolOptions(c *echo.Context) error {
	options := make([]ProviderPoolOption, 0)
	for name, typ := range h.providerTypeByName() {
		options = append(options, ProviderPoolOption{Name: name, Type: typ})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Type != options[j].Type {
			return options[i].Type < options[j].Type
		}
		return options[i].Name < options[j].Name
	})
	return c.JSON(http.StatusOK, map[string]any{"providers": options})
}

type poolConfigRequest struct {
	Name        string         `json:"name"`
	Members     []string       `json:"members"`
	Strategy    string         `json:"strategy"`
	Weights     map[string]int `json:"weights,omitempty"`
	HealthAware *bool          `json:"health_aware,omitempty"`
}

// CreatePool handles POST /admin/api/v1/pools — creates a new load-balanced pool.
func (h *Handler) CreatePool(c *echo.Context) error {
	if h.poolWeights == nil || h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("pool management is unavailable"))
	}

	var req poolConfigRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid pool payload: "+err.Error())
	}

	raw, err := h.validatePoolConfig(req.Name, req)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), nil))
	}

	h.poolWeights.upsert(strings.TrimSpace(req.Name), raw)
	h.poolWeights.save()
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusCreated, poolModifyResponse{
		Message:                fmt.Sprintf("pool %q created", req.Name),
		Pool:                   strings.TrimSpace(req.Name),
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// UpdatePool handles PUT /admin/api/v1/pools/:name — replaces a pool's members,
// strategy, weights, and health-aware flag.
func (h *Handler) UpdatePool(c *echo.Context) error {
	if h.poolWeights == nil || h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("pool management is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("pool name is required", nil))
	}

	var req poolConfigRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid pool payload: "+err.Error())
	}
	req.Name = name

	raw, err := h.validatePoolConfig(name, req)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), nil))
	}

	h.poolWeights.upsert(name, raw)
	h.poolWeights.save()
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusOK, poolModifyResponse{
		Message:                fmt.Sprintf("pool %q updated", name),
		Pool:                   name,
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// DeletePool handles DELETE /admin/api/v1/pools/:name.
func (h *Handler) DeletePool(c *echo.Context) error {
	if h.poolWeights == nil || h.providerOverrides == nil {
		return handleError(c, featureUnavailableError("pool management is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("pool name is required", nil))
	}

	h.poolWeights.delete(name)
	h.poolWeights.save()
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusOK, poolModifyResponse{
		Message:                fmt.Sprintf("pool %q deleted", name),
		Pool:                   name,
		RuntimeApplied:         apply.Applied,
		RequiresRuntimeRefresh: !apply.Applied,
		RuntimeRefresh:         apply.Report,
		RuntimeRefreshError:    apply.Error,
	})
}

// validatePoolConfig normalizes and validates a pool configuration, returning
// the canonical RawPoolConfig or an error describing the first problem.
func (h *Handler) validatePoolConfig(name string, req poolConfigRequest) (cfg.RawPoolConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return cfg.RawPoolConfig{}, fmt.Errorf("pool name is required")
	}

	types := h.providerTypeByName()
	if _, collides := types[name]; collides {
		return cfg.RawPoolConfig{}, fmt.Errorf("pool name %q collides with a configured provider instance — pick a distinct name", name)
	}

	members := normalizeMembers(req.Members)
	if len(members) == 0 {
		return cfg.RawPoolConfig{}, fmt.Errorf("at least one member is required")
	}

	var sharedType string
	for _, m := range members {
		t, ok := types[m]
		if !ok {
			return cfg.RawPoolConfig{}, fmt.Errorf("member %q is not a configured provider", m)
		}
		if sharedType == "" {
			sharedType = t
		} else if sharedType != t {
			return cfg.RawPoolConfig{}, fmt.Errorf("mixed provider types (%q and %q) — all members must share a type", sharedType, t)
		}
	}

	if _, err := pool.ParseStrategy(req.Strategy); err != nil {
		return cfg.RawPoolConfig{}, err
	}

	weights := make(map[string]int, len(req.Weights))
	for k, v := range req.Weights {
		weights[strings.TrimSpace(k)] = v
	}

	return cfg.RawPoolConfig{
		Members:     members,
		Strategy:    strings.TrimSpace(req.Strategy),
		Weights:     weights,
		HealthAware: req.HealthAware,
	}, nil
}

func normalizeMembers(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

// providerTypeByName returns a map of configured provider instance name → lowercased
// provider type, merging static providers and UI-created provider overrides.
func (h *Handler) providerTypeByName() map[string]string {
	out := make(map[string]string)
	for _, p := range h.configuredProviders {
		n := strings.TrimSpace(p.Name)
		if n == "" {
			continue
		}
		out[n] = strings.ToLower(strings.TrimSpace(p.Type))
	}
	if h.providerOverrides != nil {
		for _, o := range h.providerOverrides.list() {
			n := strings.TrimSpace(o.Name)
			if n == "" || !o.IsEnabled() {
				continue
			}
			out[n] = strings.ToLower(strings.TrimSpace(o.Type))
		}
	}
	return out
}

type poolModifyResponse struct {
	Message                string                `json:"message"`
	Pool                   string                `json:"pool"`
	RuntimeApplied         bool                  `json:"runtime_applied"`
	RequiresRuntimeRefresh bool                  `json:"requires_runtime_refresh"`
	RuntimeRefresh         *RuntimeRefreshReport `json:"runtime_refresh,omitempty"`
	RuntimeRefreshError    string                `json:"runtime_refresh_error,omitempty"`
}

func badRequest(c *echo.Context, msg string) error {
	code := "bad_request"
	return c.JSON(http.StatusBadRequest, core.GatewayError{
		Code:    &code,
		Type:    "invalid_request_error",
		Message: msg,
	})
}
