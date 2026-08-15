package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/configuration"
)

// FallbackRule represents a single fallback mapping: source model -> ordered targets.
type FallbackRule struct {
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
	Enabled bool     `json:"enabled"`
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

	// Normalize: trim whitespace, skip empty sources. Use a slice to
	// preserve the insertion order that the client sent.
	normalized := make([]fallbackRuleEntry, 0, len(req.Rules))
	seen := make(map[string]struct{}, len(req.Rules))
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
		if len(targets) == 0 {
			continue
		}
		if _, dup := seen[source]; dup {
			continue
		}
		seen[source] = struct{}{}
		normalized = append(normalized, fallbackRuleEntry{
			Source:  source,
			Targets: targets,
			Enabled: rule.Enabled,
		})
	}

	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal fallback config"})
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write %s: %v", path, err)})
	}

	// Apply the new rules to the live resolver immediately so edits, toggles,
	// and deletions take effect without a restart.
	if h.fallbackReloader != nil {
		if err := h.fallbackReloader.ReloadFallback(); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("saved %s but live reload failed: %v", path, err)})
		}
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

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// New format: JSON array of {source, targets, enabled} objects.
	if trimmed[0] == '[' {
		var entries []fallbackRuleEntry
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, err
		}
		rules := make([]FallbackRule, 0, len(entries))
		for _, e := range entries {
			source := strings.TrimSpace(e.Source)
			if source == "" {
				continue
			}
			targets := sanitizeTargets(e.Targets)
			if len(targets) == 0 {
				continue
			}
			rules = append(rules, FallbackRule{Source: source, Targets: targets, Enabled: e.Enabled})
		}
		return rules, nil
	}

	// Legacy format: JSON object {"source": {"targets": [...], "enabled": bool}}.
	var data map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &data); err != nil {
		return nil, err
	}

	rules := make([]FallbackRule, 0, len(data))
	for source, rawValue := range data {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		targets, enabled, err := config.DecodeManualRuleValue(rawValue)
		if err != nil {
			return nil, err
		}
		safeTargets := sanitizeTargets(targets)
		if len(safeTargets) == 0 {
			continue
		}
		rules = append(rules, FallbackRule{Source: source, Targets: safeTargets, Enabled: enabled})
	}

	return rules, nil
}

func sanitizeTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// fallbackRuleEntry is the on-disk form of a fallback rule. In the new format
// it appears inside a JSON array with an explicit "source" key. The old
// object-key format (where the source was the JSON key) is also supported.
type fallbackRuleEntry struct {
	Source  string   `json:"source,omitempty"`
	Targets []string `json:"targets"`
	Enabled bool     `json:"enabled"`
}
