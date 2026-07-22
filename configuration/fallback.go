package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	decoded := make(map[string][]string)
	decoder := json.NewDecoder(strings.NewReader(expanded))

	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: top-level JSON value must be an object", path)
	}

	seenKeys := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: object key must be a string", path)
		}
		if _, exists := seenKeys[key]; exists {
			return fmt.Errorf("fallback.manual_rules_path: duplicate JSON key %q in %q", key, path)
		}
		seenKeys[key] = struct{}{}

		var rawModels json.RawMessage
		if err := decoder.Decode(&rawModels); err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
		}
		if bytes.Equal(bytes.TrimSpace(rawModels), []byte("null")) {
			return fmt.Errorf("fallback.manual_rules_path: null not allowed for %q in %q", key, path)
		}
		var models []string
		if err := json.Unmarshal(rawModels, &models); err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
		}
		decoded[key] = models
	}

	token, err = decoder.Token()
	if err != nil {
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
	}
	delim, ok = token.(json.Delim)
	if !ok || delim != '}' {
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: top-level JSON value must be an object", path)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: %w", path, err)
		}
		return fmt.Errorf("fallback.manual_rules_path: failed to parse %q: unexpected trailing JSON content", path)
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
		normalized := make([]string, 0, len(models))
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			normalized = append(normalized, model)
		}
		manual[key] = normalized
	}
	cfg.Manual = manual
	return nil
}
