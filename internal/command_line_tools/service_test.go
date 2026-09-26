package clitools

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestPreviewUsesPlaceholderForEmptyAPIKey(t *testing.T) {
	service := NewService(false, nil)
	for _, apiKey := range []string{"", "   "} {
		preview, err := service.Preview("claude-code", PreviewRequest{
			BaseURL: "http://localhost:8080",
			APIKey:  apiKey,
			Model:   "fallback/model",
		})
		if err != nil {
			t.Fatalf("Preview returned error for api key %q: %v", apiKey, err)
		}
		assertEqual(t, preview.MaskedKey, "<AURORA_API_KEY>")
		if !strings.Contains(preview.Snippets["env"], "ANTHROPIC_AUTH_TOKEN=<AURORA_API_KEY>") {
			t.Fatalf("expected placeholder in env snippet, got %s", preview.Snippets["env"])
		}
		if strings.Contains(preview.Snippets["env"], "********") {
			t.Fatalf("did not expect mask placeholder for empty key, got %s", preview.Snippets["env"])
		}
	}
}

func TestPreviewMasksNonEmptyAPIKey(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key-123456",
		Model:   "fallback/model",
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertEqual(t, preview.MaskedKey, "sk-t…3456")
	if preview.MaskedKey == "<AURORA_API_KEY>" {
		t.Fatal("expected masked key for non-empty api key")
	}
}

func TestApplyRejectsEmptyAPIKey(t *testing.T) {
	service := NewService(true, nil)
	if _, err := service.Apply("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "",
		Model:   "fallback/model",
	}); err == nil {
		t.Fatal("expected Apply to reject empty api_key")
	}
}

func TestBuiltInPresetsTargetKnownTools(t *testing.T) {
	service := NewService(false, nil)
	presets := service.ListPresets()
	if len(presets) == 0 {
		t.Fatal("expected built-in presets")
	}
	seen := map[string]bool{}
	for _, preset := range presets {
		if preset.ID == "" || preset.Label == "" || preset.ToolID == "" {
			t.Fatalf("preset missing required fields: %+v", preset)
		}
		if seen[preset.ID] {
			t.Fatalf("duplicate preset id %s", preset.ID)
		}
		seen[preset.ID] = true
		if _, ok := service.GetTool(preset.ToolID); !ok {
			t.Fatalf("preset %s targets unknown tool %s", preset.ID, preset.ToolID)
		}
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

func TestPreviewClaudeAvailableModelsMultiSelect(t *testing.T) {
	service := NewService(false, nil)
	preview, err := service.Preview("claude-code", PreviewRequest{
		BaseURL:        "http://localhost:8080",
		APIKey:         "sk-test",
		Model:          "fallback/model",
		ModelOverrides: map[string]string{"ANTHROPIC_MODEL": "primary/model"},
		Models:         []string{"alpha/model", "beta/model", "gamma/model"},
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	var cfg struct {
		Models  []string `json:"availableModels"`
		Primary string   `json:"model"`
		Env     map[string]string
	}
	if err := json.Unmarshal([]byte(preview.Snippets["config"]), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Primary != "primary/model" {
		t.Fatalf("model = %q, want primary/model", cfg.Primary)
	}
	want := map[string]bool{"primary/model": true, "alpha/model": true, "beta/model": true, "gamma/model": true}
	if len(cfg.Models) != len(want) {
		t.Fatalf("availableModels = %v, want %v entries", cfg.Models, len(want))
	}
	for _, m := range cfg.Models {
		if !want[m] {
			t.Fatalf("unexpected model in availableModels: %v", cfg.Models)
		}
	}
	if cfg.Env["ANTHROPIC_MODEL"] != "primary/model" {
		t.Fatalf("env ANTHROPIC_MODEL = %q", cfg.Env["ANTHROPIC_MODEL"])
	}
	// Multi key must not leak into ModelOverrides (rejected by validator).
	if _, ok := preview.Snippets["config"]; !ok {
		t.Fatal("missing config snippet")
	}
}

func TestPreviewRejectsMultiFieldAsModelOverride(t *testing.T) {
	service := NewService(false, nil)
	_, err := service.Preview("claude-code", PreviewRequest{
		BaseURL:        "http://localhost:8080",
		APIKey:         "sk-test",
		Model:          "fallback/model",
		ModelOverrides: map[string]string{"AVAILABLE_MODELS": "should/be/rejected"},
	})
	if err == nil {
		t.Fatal("expected AVAILABLE_MODELS model_override to be rejected")
	}
}

func assertEqual(t *testing.T, actual string, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}

func TestOpenCodeToolGeneratesProviderConfig(t *testing.T) {
	service := NewService(false, nil)
	tool, ok := service.GetTool("opencode")
	if !ok {
		t.Fatal("expected opencode tool")
	}
	if !strings.HasSuffix(tool.ConfigPath, filepath.Join(".config", "opencode", "opencode.json")) {
		t.Fatalf("expected opencode config path, got %s", tool.ConfigPath)
	}
	preview, err := service.Preview("opencode", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "fallback/model",
		ModelOverrides: map[string]string{
			"OPENCODE_MODEL": "provider/coded-model",
		},
		Models: []string{"provider/extra"},
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	var cfg struct {
		Model    string `json:"model"`
		Provider map[string]struct {
			NPM     string            `json:"npm"`
			Options map[string]string `json:"options"`
			Models  map[string]any    `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(preview.Snippets["config"]), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	assertEqual(t, cfg.Model, "aurorax/provider/coded-model")
	aurorax, ok := cfg.Provider["aurorax"]
	if !ok {
		t.Fatal("expected aurorax provider block")
	}
	assertEqual(t, aurorax.NPM, "@ai-sdk/openai-compatible")
	assertEqual(t, aurorax.Options["baseURL"], "http://localhost:8080/v1")
	assertEqual(t, aurorax.Options["apiKey"], preview.MaskedKey)
	if _, ok := aurorax.Models["provider/coded-model"]; !ok {
		t.Fatalf("expected primary model in models map, got %v", aurorax.Models)
	}
	if _, ok := aurorax.Models["provider/extra"]; !ok {
		t.Fatalf("expected extra model in models map, got %v", aurorax.Models)
	}
}

func TestOpenCodePresetTargetsOpenCodeTool(t *testing.T) {
	service := NewService(false, nil)
	found := false
	for _, preset := range service.ListPresets() {
		if preset.ToolID == "opencode" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an opencode preset")
	}
}

func TestCodexAuthConfigPathSet(t *testing.T) {
	service := NewService(true, nil)
	tool, ok := service.GetTool("codex")
	if !ok {
		t.Fatal("expected codex tool")
	}
	if !strings.HasSuffix(tool.AuthConfigPath, filepath.Join(".codex", "auth.json")) {
		t.Fatalf("expected codex auth.json path, got %s", tool.AuthConfigPath)
	}
}

func TestApplyWritesCodexAuthFile(t *testing.T) {
	fs := newMemFS()
	service := NewService(true, fs)
	resp, err := service.Apply("codex", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key-123456",
		Model:   "gpt-5-codex",
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !resp.Applied {
		t.Fatal("expected apply to succeed")
	}
	auth, ok := fs.files[toolPath(t, service, "codex", "auth")]
	if !ok {
		t.Fatal("expected codex auth.json to be written")
	}
	if !strings.Contains(string(auth), `"OPENAI_API_KEY": "sk-test-key-123456"`) {
		t.Fatalf("auth file missing real api key, got %s", auth)
	}
	config, ok := fs.files[toolPath(t, service, "codex", "config")]
	if !ok {
		t.Fatal("expected codex config to be written")
	}
	if !strings.Contains(string(config), `model_provider = "aurora"`) {
		t.Fatalf("config missing aurora provider, got %s", config)
	}
}

func TestOpenCodeApplyMergesExistingConfig(t *testing.T) {
	fs := newMemFS()
	service := NewService(true, fs)
	tool, ok := service.GetTool("opencode")
	if !ok {
		t.Fatal("expected opencode tool")
	}
	existing := []byte(`{"$schema":"https://opencode.ai/config.json","theme":"dark","provider":{"other":{"npm":"@ai-sdk/openai"}}}`)
	if err := fs.WriteFile(tool.ConfigPath, existing, 0o600); err != nil {
		t.Fatalf("seed existing config: %v", err)
	}
	if _, err := service.Apply("opencode", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key-123456",
		Model:   "coded/model",
	}); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	merged, ok := fs.files[tool.ConfigPath]
	if !ok {
		t.Fatal("expected merged config to be written")
	}
	var cfg map[string]any
	if err := json.Unmarshal(merged, &cfg); err != nil {
		t.Fatalf("decode merged config: %v", err)
	}
	assertEqual(t, cfg["theme"].(string), "dark")
	providers, ok := cfg["provider"].(map[string]any)
	if !ok {
		t.Fatal("expected provider map in merged config")
	}
	if _, ok := providers["other"]; !ok {
		t.Fatal("expected unrelated provider to survive merge")
	}
	if _, ok := providers["aurorax"]; !ok {
		t.Fatal("expected aurorax provider in merged config")
	}
	if backup, ok := fs.files[tool.ConfigPath+".aurora.bak"]; !ok || !strings.Contains(string(backup), "dark") {
		t.Fatal("expected backup of the previous config")
	}
}

func TestResetRestoresBackup(t *testing.T) {
	fs := newMemFS()
	service := NewService(true, fs)
	tool, ok := service.GetTool("claude-code")
	if !ok {
		t.Fatal("expected claude-code tool")
	}
	original := []byte(`{"model":"keep/me"}`)
	if err := fs.WriteFile(tool.ConfigPath, original, 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if _, err := service.Apply("claude-code", PreviewRequest{
		BaseURL: "http://localhost:8080",
		APIKey:  "sk-test-key",
		Model:   "new/model",
	}); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	resp, err := service.Reset("claude-code")
	if err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	if !resp.Applied || resp.Path != tool.ConfigPath {
		t.Fatalf("unexpected reset response: %+v", resp)
	}
	restored := fs.files[tool.ConfigPath]
	if !strings.Contains(string(restored), "keep/me") {
		t.Fatalf("expected original config restored, got %s", restored)
	}
}

func TestResetWithoutBackupFails(t *testing.T) {
	fs := newMemFS()
	service := NewService(true, fs)
	if _, err := service.Reset("claude-code"); err == nil {
		t.Fatal("expected Reset to fail without a backup")
	}
}

// memFS is an in-memory FileSystem for apply/reset tests.
type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS { return &memFS{files: map[string][]byte{}} }

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	copied := make([]byte, len(data))
	copy(copied, data)
	m.files[path] = copied
	return nil
}

func (m *memFS) MkdirAll(string, os.FileMode) error { return nil }

func toolPath(t *testing.T, service *Service, toolID string, snippet string) string {
	t.Helper()
	tool, ok := service.GetTool(toolID)
	if !ok {
		t.Fatalf("expected %s tool", toolID)
	}
	if snippet == "auth" {
		if tool.AuthConfigPath == "" {
			t.Fatalf("tool %s has no auth config path", toolID)
		}
		return tool.AuthConfigPath
	}
	return tool.ConfigPath
}
