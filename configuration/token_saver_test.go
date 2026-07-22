package config

import (
	"strings"
	"testing"
)

func TestTokenSaverDefaultsDisabled(t *testing.T) {
	cfg := buildDefaultConfig()

	if cfg.TokenSaver.Enabled {
		t.Fatal("expected TokenSaver.Enabled=false")
	}
	if cfg.TokenSaver.OnError != TokenSaverOnErrorAllow {
		t.Fatalf("TokenSaver.OnError = %q, want %q", cfg.TokenSaver.OnError, TokenSaverOnErrorAllow)
	}
	if cfg.TokenSaver.Output.Level != TokenSaverOutputLevelFull {
		t.Fatalf("TokenSaver.Output.Level = %q, want %q", cfg.TokenSaver.Output.Level, TokenSaverOutputLevelFull)
	}
}

func TestTokenSaverEnvOverrides(t *testing.T) {
	t.Setenv("TOKEN_SAVER_ENABLED", "true")
	t.Setenv("TOKEN_SAVER_OUTPUT_ENABLED", "true")
	t.Setenv("TOKEN_SAVER_MODELS_INCLUDE", "claude-sonnet-4-6, gpt-5")
	t.Setenv("TOKEN_SAVER_EMIT_HEADERS", "true")

	cfg := buildDefaultConfig()
	if err := applyEnvOverrides(cfg); err != nil {
		t.Fatalf("applyEnvOverrides() failed: %v", err)
	}

	if !cfg.TokenSaver.Enabled {
		t.Fatal("expected env to enable token saver")
	}
	if !cfg.TokenSaver.Output.Enabled {
		t.Fatal("expected env to enable output profile")
	}
	if got := strings.Join(cfg.TokenSaver.Models.Include, ","); got != "claude-sonnet-4-6,gpt-5" {
		t.Fatalf("TokenSaver.Models.Include = %q", got)
	}
	if !cfg.TokenSaver.EmitHeaders {
		t.Fatal("expected env to enable headers")
	}
}

func TestValidateTokenSaverRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  TokenSaverConfig
		want string
	}{
		{name: "invalid on error", cfg: TokenSaverConfig{OnError: "retry"}, want: "token_saver.on_error"},
		{name: "bad profile", cfg: TokenSaverConfig{Output: TokenSaverOutputConfig{Profile: "caveman-ultra", Level: TokenSaverOutputLevelFull}}, want: "token_saver.output.profile"},
		{name: "bad level", cfg: TokenSaverConfig{Output: TokenSaverOutputConfig{Profile: TokenSaverOutputProfileConcise, Level: "extreme"}}, want: "token_saver.output.level"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokenSaverConfig(&tt.cfg)
			if err == nil {
				t.Fatal("ValidateTokenSaverConfig() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateTokenSaverConfig() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTokenSaverOutputInstructionLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
		check func(string) bool
	}{
		{name: "lite", level: TokenSaverOutputLevelLite, check: func(s string) bool { return strings.Contains(s, "filler") }},
		{name: "full (default)", level: TokenSaverOutputLevelFull, check: func(s string) bool { return strings.Contains(s, "caveman") }},
		{name: "ultra", level: TokenSaverOutputLevelUltra, check: func(s string) bool { return strings.Contains(s, "ultra-terse") }},
		{name: "wenyan", level: TokenSaverOutputLevelWenyan, check: func(s string) bool { return strings.Contains(s, "文言文") }},
		{name: "unknown falls back to full", level: "bogus", check: func(s string) bool { return strings.Contains(s, "caveman") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := TokenSaverOutputConfig{Level: tt.level}
			instruction := o.OutputInstruction()
			if !tt.check(instruction) {
				t.Errorf("OutputInstruction(%q) = %q, does not match expected content", tt.level, instruction)
			}
		})
	}
}
