package admin

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"

	"aurora/configuration"
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

// PoolOverrideStore holds per-pool overrides applied via the admin API.
type PoolOverrideStore struct {
	mu    sync.Mutex
	pools map[string]PoolOverrideData
}

// PoolOverrideData contains a dashboard-managed pool strategy/weight override.
type PoolOverrideData struct {
	Strategy string         `json:"strategy,omitempty"`
	Weights  map[string]int `json:"weights,omitempty"`
}

func NewPoolOverrideStore() *PoolOverrideStore {
	return &PoolOverrideStore{pools: make(map[string]PoolOverrideData)}
}

func (p *PoolOverrideStore) get(name string) (PoolOverrideData, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.pools[name]
	return v, ok
}

func (p *PoolOverrideStore) upsert(name string, data PoolOverrideData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pools[name] = data
}

func (p *PoolOverrideStore) ApplyToRawPools(rawPools map[string]config.RawPoolConfig) map[string]config.RawPoolConfig {
	out := make(map[string]config.RawPoolConfig, len(rawPools))
	for name, raw := range rawPools {
		raw.Members = append([]string(nil), raw.Members...)
		if len(raw.Weights) > 0 {
			weights := make(map[string]int, len(raw.Weights))
			for member, weight := range raw.Weights {
				weights[member] = weight
			}
			raw.Weights = weights
		}
		out[name] = raw
	}
	if p == nil {
		return out
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, override := range p.pools {
		raw, ok := out[name]
		if !ok {
			continue
		}
		if override.Strategy != "" {
			raw.Strategy = override.Strategy
		}
		if len(override.Weights) > 0 {
			raw.Weights = make(map[string]int, len(override.Weights))
			for member, weight := range override.Weights {
				raw.Weights[member] = weight
			}
		}
		out[name] = raw
	}
	return out
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

type poolUpdateRequest struct {
	Strategy string         `json:"strategy,omitempty"`
	Weights  map[string]int `json:"weights,omitempty"`
}

// UpdatePool handles PUT /admin/api/v1/pools/:name
// Updates load balancing strategy and per-member weights for a pool.
func (h *Handler) UpdatePool(c *echo.Context) error {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("pool name is required", nil))
	}

	if h.pools == nil || h.poolWeights == nil {
		return handleError(c, featureUnavailableError("pool management is unavailable"))
	}

	var req poolUpdateRequest
	if err := c.Bind(&req); err != nil {
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: "invalid pool payload: " + err.Error(),
		})
	}

	data := PoolOverrideData{
		Strategy: strings.TrimSpace(req.Strategy),
		Weights:  req.Weights,
	}
	if data.Weights == nil {
		data.Weights = make(map[string]int)
	}
	// Normalize member weight keys to lowercase
	normalized := make(map[string]int, len(data.Weights))
	for k, v := range data.Weights {
		normalized[strings.ToLower(strings.TrimSpace(k))] = v
	}
	data.Weights = normalized

	h.poolWeights.upsert(name, data)
	apply := h.applyRuntimeRefresh(c)

	return c.JSON(http.StatusOK, map[string]any{
		"message":                  fmt.Sprintf("pool %q updated", name),
		"pool_name":                name,
		"strategy":                 data.Strategy,
		"runtime_applied":          apply.Applied,
		"requires_runtime_refresh": !apply.Applied,
		"runtime_refresh":          apply.Report,
		"runtime_refresh_error":    apply.Error,
	})
}

// UpdatePoolStrategy handles a simpler strategy-only update.
// poolOverrideConfig returns the effective pool config including overrides.
func poolEffectiveStrategy(name string, base string, overrides *PoolOverrideStore) string {
	if overrides == nil {
		return base
	}
	data, ok := overrides.get(name)
	if !ok || data.Strategy == "" {
		return base
	}
	return data.Strategy
}

// poolEffectiveWeights returns effective weights including any overrides.
func poolEffectiveWeights(name string, base map[string]int, overrides *PoolOverrideStore) map[string]int {
	if overrides == nil {
		return base
	}
	data, ok := overrides.get(name)
	if !ok || len(data.Weights) == 0 {
		return base
	}
	return data.Weights
}
