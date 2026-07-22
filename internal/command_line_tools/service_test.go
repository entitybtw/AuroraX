package clitools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeCodeSnippetsUsesModelOverrides(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "fallback/model",
		ModelOverrides: map[string]string{
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "provider/haiku-model",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/sonnet-model",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "provider/opus-model",
		},
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	env := decodeClaudeEnv(t, preview.Snippets["config"])
	assertEqual(t, env["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "provider/haiku-model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_SONNET_MODEL"], "provider/sonnet-model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_OPUS_MODEL"], "provider/opus-model")
}

func TestClaudeCodeSnippetsFallsBackToModelForMissingOverrides(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "fallback/model",
		ModelOverrides: map[string]string{
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/sonnet-model",
		},
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	env := decodeClaudeEnv(t, preview.Snippets["config"])
	assertEqual(t, env["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "fallback/model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_SONNET_MODEL"], "provider/sonnet-model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_OPUS_MODEL"], "fallback/model")
}

func TestClaudeCodeSnippetsKeepsLegacyModelFallback(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "legacy/model",
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	env := decodeClaudeEnv(t, preview.Snippets["config"])
	assertEqual(t, env["ANTHROPIC_MODEL"], "legacy/model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "legacy/model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_SONNET_MODEL"], "legacy/model")
	assertEqual(t, env["ANTHROPIC_DEFAULT_OPUS_MODEL"], "legacy/model")
}

func TestClaudeCodeToolUsesOfficialSettingsPath(t *testing.T) {
	service := NewService(false, nil)
	tool, ok := service.GetTool("claude-code")
	if !ok {
		t.Fatal("expected claude-code tool")
	}
	if !strings.HasSuffix(tool.ConfigPath, ".claude/settings.json") && !strings.HasSuffix(tool.ConfigPath, `.claude\settings.json`) {
		t.Fatalf("expected Claude Code settings path, got %s", tool.ConfigPath)
	}
}

func TestClaudeCodeConfigIncludesSettingsSchemaAndPermissions(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "fallback/model",
		ModelOverrides: map[string]string{
			"ANTHROPIC_MODEL": "provider/primary-model",
		},
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(preview.Snippets["config"]), &cfg); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	assertEqual(t, cfg["$schema"].(string), "https://json.schemastore.org/claude-code-settings.json")
	assertEqual(t, cfg["model"].(string), "provider/primary-model")
	if _, ok := cfg["permissions"].(map[string]any); !ok {
		t.Fatal("expected permissions config")
	}
	if _, ok := cfg["availableModels"].([]any); !ok {
		t.Fatal("expected availableModels config")
	}
}

func TestPreviewRejectsInvalidModelOverrides(t *testing.T) {
	service := NewService(false, nil)
	cases := []struct {
		name      string
		overrides map[string]string
	}{
		{name: "unsafe model", overrides: map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "bad*model"}},
		{name: "control character", overrides: map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/\nmodel"}},
		{name: "unknown key", overrides: map[string]string{"ANTHROPIC_DEFAULT_FAST_MODEL": "provider/model"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Preview("claude-code", PreviewRequest{
				BaseURL:        "http://localhost:8080",
				APIKey:         "sk-test-key",
				Model:          "fallback/model",
				ModelOverrides: tc.overrides,
			})
			if err == nil {
				t.Fatal("expected Preview to reject invalid model override")
			}
		})
	}
}

func TestToolsIncludeModelFields(t *testing.T) {
	service := NewService(false, nil)
	cases := []struct {
		toolID string
		keys   []string
	}{
		{toolID: "claude-code", keys: []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL"}},
		{toolID: "codex", keys: []string{"CODEX_MODEL", "CODEX_SUBAGENT_MODEL"}},
		{toolID: "opencode", keys: []string{"OPENCODE_MODEL"}},
		{toolID: "generic", keys: []string{"OPENAI_MODEL"}},
	}

	for _, tc := range cases {
		t.Run(tc.toolID, func(t *testing.T) {
			tool, ok := service.GetTool(tc.toolID)
			if !ok {
				t.Fatalf("expected %s tool", tc.toolID)
			}
			keys := map[string]bool{}
			for _, field := range tool.ModelFields {
				keys[field.Key] = true
			}
			for _, key := range tc.keys {
				if !keys[key] {
					t.Fatalf("expected model field %s", key)
				}
			}
		})
	}
}

func TestCodexSnippetsUseModelOverride(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("codex", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "fallback/model",
		ModelOverrides: map[string]string{
			"CODEX_MODEL":          "provider/codex-model",
			"CODEX_SUBAGENT_MODEL": "provider/subagent-model",
		},
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if !strings.Contains(preview.Snippets["config"], `model = "provider/codex-model"`) {
		t.Fatalf("expected codex config to use override, got %s", preview.Snippets["config"])
	}
	if !strings.Contains(preview.Snippets["config"], `model = "provider/subagent-model"`) {
		t.Fatalf("expected codex subagent config to use override, got %s", preview.Snippets["config"])
	}
	if !strings.Contains(preview.Snippets["auth"], `"OPENAI_API_KEY"`) {
		t.Fatalf("expected codex auth snippet, got %s", preview.Snippets["auth"])
	}
}

func decodeClaudeEnv(t *testing.T, config string) map[string]string {
	t.Helper()
	var parsed struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	return parsed.Env
}

func assertEqual(t *testing.T, actual string, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}
