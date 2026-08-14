package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
)

// FallbackRule represents a single fallback mapping: source model -> ordered targets.
type FallbackRule struct {
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
}

// FallbackConfigResponse is the API response for GET/PUT /admin/api/v1/fallback.
type FallbackConfigResponse struct {
	Rules []FallbackRule `json:"rules"`
}

// FallbackConfig handles GET /admin/api/v1/fallback — reads fallback.json from disk every time.
func (h *Handler) FallbackConfig(c *echo.Context) error {
	path := h.fallbackPath()
	if path == "" {
		return c.JSON(http.StatusOK, FallbackConfigResponse{Rules: []FallbackRule{}})
	}

	rules, err := readFallbackRules(path)
	if err != nil {
		return c.JSON(http.StatusOK, FallbackConfigResponse{Rules: []FallbackRule{}})
	}

	return c.JSON(http.StatusOK, FallbackConfigResponse{Rules: rules})
}

// UpdateFallbackConfig handles PUT /admin/api/v1/fallback — writes fallback.json to disk.
func (h *Handler) UpdateFallbackConfig(c *echo.Context) error {
	path := h.fallbackPath()
	if path == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fallback.manual_rules_path not configured"})
	}

	var req FallbackConfigResponse
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Normalize: trim whitespace, skip empty sources.
	normalized := make(map[string][]string, len(req.Rules))
	for _, rule := range req.Rules {
		source := strings.TrimSpace(rule.Source)
		if source == "" {
			continue
		}
		targets := make([]string, 0, len(rule.Targets))
		for _, t := range rule.Targets {
			t = strings.TrimSpace(t)
			if t != "" {
				targets = append(targets, t)
			}
		}
		if len(targets) > 0 {
			normalized[source] = targets
		}
	}

	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal fallback config"})
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write %s: %v", path, err)})
	}

	// Re-read to return the canonical state.
	rules, err := readFallbackRules(path)
	if err != nil {
		return c.JSON(http.StatusOK, FallbackConfigResponse{Rules: []FallbackRule{}})
	}

	return c.JSON(http.StatusOK, FallbackConfigResponse{Rules: rules})
}

func (h *Handler) fallbackPath() string {
	if h.runtimeConfig.Fallback.ManualRulesConfigured || len(h.runtimeConfig.Fallback.ManualRules) > 0 {
		// We don't have the path directly in the snapshot, but the config loaded it.
		// Use the environment variable or default path.
	}
	// Try env var first, then default path.
	if p := strings.TrimSpace(os.Getenv("FALLBACK_MANUAL_RULES_PATH")); p != "" {
		return p
	}
	return "configs/fallback.json"
}

func readFallbackRules(path string) ([]FallbackRule, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string][]string
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	rules := make([]FallbackRule, 0, len(data))
	for source, targets := range data {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		safeTargets := make([]string, 0, len(targets))
		for _, t := range targets {
			t = strings.TrimSpace(t)
			if t != "" {
				safeTargets = append(safeTargets, t)
			}
		}
		rules = append(rules, FallbackRule{Source: source, Targets: safeTargets})
	}

	return rules, nil
}
