package config

import (
	"fmt"
	"strings"
)

const (
	TokenSaverOnErrorAllow = "allow"
	TokenSaverOnErrorBlock = "block"

	TokenSaverOutputProfileConcise = "concise"

	TokenSaverOutputLevelLite   = "lite"
	TokenSaverOutputLevelFull   = "full"
	TokenSaverOutputLevelUltra  = "ultra"
	TokenSaverOutputLevelWenyan = "wenyan"
)

const InstructionLite = `Respond concisely. Drop filler words (just/really/basically/actually/simply), hedging, and pleasantries (sure/certainly/of course). Keep articles and full sentences. Professional but tight. Technical terms exact. Code blocks unchanged. Preserve all technical accuracy.`

const InstructionFull = `Respond terse like smart caveman. All technical substance stay. Only fluff die.

Rules:
- Drop: articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries (sure/certainly/of course/happy to), hedging.
- Fragments OK. Short synonyms (big not extensive, fix not "implement a solution for").
- Technical terms exact. Code blocks unchanged. Errors quoted exact.
- Pattern: [thing] [action] [reason]. [next step].

Not: "Sure! I'd be happy to help you with that. The issue you're experiencing is likely caused by..."
Yes: "Bug in auth middleware. Token expiry check use < not <=. Fix:"

Preserve all code, numbers, and technical accuracy. Never omit safety warnings.`

const InstructionUltra = `Respond ultra-terse. Abbreviate prose words (config not configuration, req not request). Strip conjunctions (and/but/or). Use arrows for causality (X → Y). One word when one word enough. No articles. No filler. Code symbols, function names, API names, error strings: never abbreviate. Pattern: [thing] → [action] → [result].`

const InstructionWenyan = `Respond in classical Chinese (文言文). Maximum terseness. Use classical sentence patterns: verbs before objects, subjects often omitted. Use classical particles (之/乃/為/其). Technical terms and code in original English. Preserve all technical accuracy.`

type TokenSaverConfig struct {
	Enabled        bool                         `yaml:"enabled" env:"TOKEN_SAVER_ENABLED"`
	Endpoints      []string                     `yaml:"endpoints" env:"TOKEN_SAVER_ENDPOINTS"`
	ApplyStreaming bool                         `yaml:"apply_streaming" env:"TOKEN_SAVER_APPLY_STREAMING"`
	Output         TokenSaverOutputConfig       `yaml:"output"`
	Models         TokenSaverModelScopeConfig   `yaml:"models"`
	Providers      TokenSaverProviderScopeConfig `yaml:"providers"`
	OnError        string                       `yaml:"on_error" env:"TOKEN_SAVER_ON_ERROR"`
	EmitHeaders    bool                         `yaml:"emit_headers" env:"TOKEN_SAVER_EMIT_HEADERS"`
	Audit          TokenSaverAuditConfig        `yaml:"audit"`
}

type TokenSaverOutputConfig struct {
	Enabled bool   `yaml:"enabled" env:"TOKEN_SAVER_OUTPUT_ENABLED"`
	Profile string `yaml:"profile" env:"TOKEN_SAVER_OUTPUT_PROFILE"`
	Level   string `yaml:"level" env:"TOKEN_SAVER_OUTPUT_LEVEL"`
}

type TokenSaverModelScopeConfig struct {
	Include []string `yaml:"include" env:"TOKEN_SAVER_MODELS_INCLUDE"`
	Exclude []string `yaml:"exclude" env:"TOKEN_SAVER_MODELS_EXCLUDE"`
}

type TokenSaverProviderScopeConfig struct {
	Include []string `yaml:"include" env:"TOKEN_SAVER_PROVIDERS_INCLUDE"`
	Exclude []string `yaml:"exclude" env:"TOKEN_SAVER_PROVIDERS_EXCLUDE"`
}

type TokenSaverAuditConfig struct {
	Enabled bool `yaml:"enabled" env:"TOKEN_SAVER_AUDIT_ENABLED"`
}

func defaultTokenSaverConfig() TokenSaverConfig {
	return TokenSaverConfig{
		Enabled:        false,
		Endpoints:      []string{"chat_completions"},
		ApplyStreaming: true,
		Output:         TokenSaverOutputConfig{Profile: TokenSaverOutputProfileConcise, Level: TokenSaverOutputLevelFull},
		OnError:        TokenSaverOnErrorAllow,
		EmitHeaders:    true,
		Audit:          TokenSaverAuditConfig{Enabled: true},
	}
}

func (o TokenSaverOutputConfig) OutputInstruction() string {
	switch strings.ToLower(strings.TrimSpace(o.Level)) {
	case TokenSaverOutputLevelLite:
		return InstructionLite
	case TokenSaverOutputLevelUltra:
		return InstructionUltra
	case TokenSaverOutputLevelWenyan:
		return InstructionWenyan
	default:
		return InstructionFull
	}
}

func ValidateTokenSaverConfig(cfg *TokenSaverConfig) error {
	cfg.OnError = strings.ToLower(strings.TrimSpace(cfg.OnError))
	if cfg.OnError == "" {
		cfg.OnError = TokenSaverOnErrorAllow
	}
	if !oneOf(cfg.OnError, TokenSaverOnErrorAllow, TokenSaverOnErrorBlock) {
		return fmt.Errorf("token_saver.on_error must be one of: allow, block")
	}

	cfg.Output.Profile = strings.ToLower(strings.TrimSpace(cfg.Output.Profile))
	if cfg.Output.Profile == "" {
		cfg.Output.Profile = TokenSaverOutputProfileConcise
	}
	if cfg.Output.Profile != TokenSaverOutputProfileConcise {
		return fmt.Errorf("token_saver.output.profile must be concise")
	}

	cfg.Output.Level = strings.ToLower(strings.TrimSpace(cfg.Output.Level))
	if cfg.Output.Level == "" {
		cfg.Output.Level = TokenSaverOutputLevelFull
	}
	if !oneOf(cfg.Output.Level, TokenSaverOutputLevelLite, TokenSaverOutputLevelFull, TokenSaverOutputLevelUltra, TokenSaverOutputLevelWenyan) {
		return fmt.Errorf("token_saver.output.level must be one of: lite, full, ultra, wenyan")
	}

	cfg.Endpoints = normalizeStringSlice(cfg.Endpoints)
	cfg.Models.Include = normalizeStringSlice(cfg.Models.Include)
	cfg.Models.Exclude = normalizeStringSlice(cfg.Models.Exclude)
	cfg.Providers.Include = normalizeStringSlice(cfg.Providers.Include)
	cfg.Providers.Exclude = normalizeStringSlice(cfg.Providers.Exclude)
	return nil
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
