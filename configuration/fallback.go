package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FallbackMode string

const (
	FallbackModeOff    FallbackMode = "off"
	FallbackModeManual FallbackMode = "manual"
	FallbackModeAuto   FallbackMode = "auto"
)

// Valid reports whether mode is one of the supported fallback modes.
func (m FallbackMode) Valid() bool {
	switch normalizeFallbackMode(m) {
	case FallbackModeOff, FallbackModeManual, FallbackModeAuto:
		return true
	default:
		return false
	}
}

func normalizeFallbackMode(mode FallbackMode) FallbackMode {
	return FallbackMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

// ResolveFallbackDefaultMode canonicalizes the global fallback default mode and
// applies the process default when unset.
func ResolveFallbackDefaultMode(mode FallbackMode) FallbackMode {
	mode = normalizeFallbackMode(mode)
	if mode == "" {
		return FallbackModeManual
	}
	return mode
}

// FallbackModelOverride holds per-model mode overrides.
type FallbackModelOverride struct {
	Mode FallbackMode `yaml:"mode" json:"mode"`
}

// FallbackConfig holds translated-route model fallback policy.
type FallbackConfig struct {
	// DefaultMode controls the fallback behavior when no per-model override exists.
	// Supported values: "auto", "manual", "off". Default: "manual".
	DefaultMode FallbackMode `yaml:"default_mode" env:"FEATURE_FALLBACK_MODE"`

	// ManualRulesPath points to a JSON file that maps source model selectors to
	// ordered fallback model selector lists. Empty disables manual rules.
	ManualRulesPath string `yaml:"manual_rules_path" env:"FALLBACK_MANUAL_RULES_PATH"`

	// Overrides controls per-model mode overrides. Keys may be bare models
	// ("gpt-4o") or provider-qualified public selectors ("azure/gpt-4o").
	Overrides map[string]FallbackModelOverride `yaml:"overrides"`

	// Manual holds the parsed manual fallback lists loaded from ManualRulesPath.
	Manual map[string][]string `yaml:"-"`
}

func loadFallbackConfig(cfg *FallbackConfig) error {
	if cfg == nil {
		return nil
	}

	cfg.DefaultMode = ResolveFallbackDefaultMode(cfg.DefaultMode)
	if !cfg.DefaultMode.Valid() {
		return fmt.Errorf("fallback.default_mode must be one of: auto, manual, off")
	}

	if len(cfg.Overrides) > 0 {
		normalized := make(map[string]FallbackModelOverride, len(cfg.Overrides))
		for key, override := range cfg.Overrides {
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("fallback.overrides: model key cannot be empty")
			}
			if _, exists := normalized[key]; exists {
				return fmt.Errorf("fallback.overrides: duplicate model key after trimming: %q", key)
			}
			override.Mode = normalizeFallbackMode(override.Mode)
			if override.Mode == "" {
				return fmt.Errorf("fallback.overrides[%q].mode must be one of: auto, manual, off", key)
			}
			if !override.Mode.Valid() {
				return fmt.Errorf("fallback.overrides[%q].mode must be one of: auto, manual, off", key)
			}
			normalized[key] = override
		}
		cfg.Overrides = normalized
	}

	path := strings.TrimSpace(cfg.ManualRulesPath)
	if path == "" {
		cfg.Manual = nil
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("fallback.manual_rules_path: failed to read %q: %w", path, err)
	}

	expanded := expandString(string(raw))
	trimmed := []byte(strings.TrimSpace(expanded))
	if len(trimmed) == 0 {
		cfg.Manual = nil
		return nil
	}

	decoded := make(map[string][]string)

	// New format: JSON array of {source, targets, enabled} objects.
	if trimmed[0] == '[' {
		type arrayEntry struct {
			Source  string          `json:"source"`
			Targets json.RawMessage `json:"targets"`
			Enabled *bool           `json:"enabled"`
		}
		var entries []arrayEntry
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
		}
		for _, entry := range entries {
			key := strings.TrimSpace(entry.Source)
			if key == "" {
				return fmt.Errorf("fallback.manual_rules_path: model key cannot be empty")
			}
			if _, exists := decoded[key]; exists {
				return fmt.Errorf("fallback.manual_rules_path: duplicate manual rule key after trimming: %q", key)
			}
			enabled := true
			if entry.Enabled != nil {
				enabled = *entry.Enabled
			}
			if !enabled {
				continue
			}
			models, _, err := DecodeManualRuleValue(entry.Targets)
			if err != nil {
				return fmt.Errorf("fallback.manual_rules_path: failed to parse targets for %q in %q: %w", key, path, err)
			}
			decoded[key] = normalizeModels(models)
		}
		cfg.Manual = decoded
		return nil
	}

	// Legacy format: JSON object {"source": {"targets": [...], "enabled": bool}}.
	var data map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &data); err != nil {
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
	}
	for key, rawValue := range data {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("fallback.manual_rules_path: model key cannot be empty")
		}
		if _, exists := decoded[key]; exists {
			return fmt.Errorf("fallback.manual_rules_path: duplicate manual rule key after trimming: %q", key)
		}
		if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
			return fmt.Errorf("fallback.manual_rules_path: null not allowed for %q in %q", key, path)
		}
		models, enabled, err := DecodeManualRuleValue(rawValue)
		if err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q in %q: %w", key, path, err)
		}
		if !enabled {
			continue
		}
		decoded[key] = normalizeModels(models)
	}

	manual := make(map[string][]string, len(decoded))
	for key, models := range decoded {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("fallback.manual_rules_path: model key cannot be empty")
		}
		if _, exists := manual[key]; exists {
			return fmt.Errorf("fallback.manual_rules_path: duplicate manual rule key after trimming: %q", key)
		}
		manual[key] = models
	}
	cfg.Manual = manual
	return nil
}

func normalizeModels(models []string) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}

// ReloadFallbackManualRules re-reads the manual rules file into cfg.Manual at
// runtime. Used for live reload when the dashboard rewrites fallback.json, so
// rule edits apply without a process restart.
func ReloadFallbackManualRules(cfg *FallbackConfig) error {
	if cfg == nil {
		return nil
	}
	return loadFallbackConfig(cfg)
}

// DecodeManualRuleValue parses a single manual rule value from the rules file.
// Both the legacy array form (["t1", "t2"], always enabled) and the object form
// ({"targets": [...], "enabled": true}) are supported. It returns the target
// list and whether the rule is enabled.
func DecodeManualRuleValue(raw json.RawMessage) ([]string, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var models []string
		if err := json.Unmarshal(trimmed, &models); err != nil {
			return nil, false, err
		}
		return models, true, nil
	}

	var obj struct {
		Targets []string `json:"targets"`
		Enabled *bool    `json:"enabled"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, false, err
	}
	enabled := true
	if obj.Enabled != nil {
		enabled = *obj.Enabled
	}
	return obj.Targets, enabled, nil
}
