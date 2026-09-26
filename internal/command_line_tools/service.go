package clitools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const apiKeyPlaceholder = "<AURORA_API_KEY>"

type Service struct {
	applyEnabled bool
	fs           FileSystem
	tools        map[string]Tool
}

type toolDefinition struct {
	Tool
	Snippet func(PreviewRequest) map[string]string
}

func NewService(applyEnabled bool, fs FileSystem) *Service {
	if fs == nil {
		fs = OSFileSystem{}
	}
	tools := make(map[string]Tool)
	for _, definition := range toolDefinitions(applyEnabled, homeDir()) {
		tools[definition.ID] = definition.Tool
	}
	return &Service{applyEnabled: applyEnabled, fs: fs, tools: tools}
}

func (s *Service) ListTools() []Tool {
	out := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) GetTool(id string) (Tool, bool) {
	tool, ok := s.tools[strings.TrimSpace(id)]
	return tool, ok
}

func (s *Service) ListPresets() []ToolPreset {
	return builtInPresets()
}

func (s *Service) Preview(toolID string, req PreviewRequest) (PreviewResponse, error) {
	tool, req, err := s.toolAndRequest(toolID, req)
	if err != nil {
		return PreviewResponse{}, err
	}
	redacted := req
	if strings.TrimSpace(req.APIKey) == "" {
		redacted.APIKey = apiKeyPlaceholder
	} else {
		redacted.APIKey = maskKey(req.APIKey)
	}
	return PreviewResponse{Tool: tool, Snippets: snippetsFor(tool.ID, redacted), MaskedKey: redacted.APIKey}, nil
}

func (s *Service) Apply(toolID string, req PreviewRequest) (ApplyResponse, error) {
	if !s.applyEnabled {
		return ApplyResponse{}, fmt.Errorf("cli tool apply is disabled")
	}
	tool, req, err := s.toolAndRequest(toolID, req)
	if err != nil {
		return ApplyResponse{}, err
	}
	if req.APIKey == "" {
		return ApplyResponse{}, fmt.Errorf("api_key is required to apply CLI tool config")
	}
	if !tool.CanApply || tool.ConfigPath == "" || !filepath.IsAbs(tool.ConfigPath) {
		return ApplyResponse{}, fmt.Errorf("tool %s does not support apply", toolID)
	}
	snippets := snippetsFor(tool.ID, req)
	content := snippets["config"]
	if content == "" {
		return ApplyResponse{}, fmt.Errorf("tool %s does not provide an applyable config snippet", toolID)
	}
	backup, err := s.writeConfigFile(tool.ConfigPath, content, tool.ID == "claude-code" || tool.ID == "opencode")
	if err != nil {
		return ApplyResponse{}, err
	}
	authBackup := ""
	if tool.AuthConfigPath != "" && strings.TrimSpace(snippets["auth"]) != "" {
		authBackup, err = s.writeConfigFile(tool.AuthConfigPath, snippets["auth"], false)
		if err != nil {
			return ApplyResponse{}, err
		}
	}
	return ApplyResponse{Applied: true, Path: tool.ConfigPath, BackupPath: firstNonEmpty(backup, authBackup)}, nil
}

// writeConfigFile backs up the current file, optionally merges JSON configs
// (Claude Code / OpenCode keep unrelated keys), and writes the new content.
func (s *Service) writeConfigFile(path string, content string, mergeJSON bool) (string, error) {
	if err := s.fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	backup := ""
	existing, readErr := s.fs.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("read existing config: %w", readErr)
	}
	if readErr == nil && len(existing) > 0 {
		backup = path + ".aurora.bak"
		if err := s.fs.WriteFile(backup, existing, 0o600); err != nil {
			return "", fmt.Errorf("write backup: %w", err)
		}
	}
	if mergeJSON && len(existing) > 0 {
		merged, err := mergeJSONSettings(existing, content, "env", "provider")
		if err != nil {
			return "", err
		}
		content = merged
	}
	if err := s.fs.WriteFile(path, []byte(content+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return backup, nil
}

// Reset restores a previously applied config from its .aurora.bak backup.
func (s *Service) Reset(toolID string) (ApplyResponse, error) {
	if !s.applyEnabled {
		return ApplyResponse{}, fmt.Errorf("cli tool apply is disabled")
	}
	tool, ok := s.GetTool(toolID)
	if !ok {
		return ApplyResponse{}, fmt.Errorf("unknown CLI tool: %s", toolID)
	}
	if !tool.CanApply || tool.ConfigPath == "" || !filepath.IsAbs(tool.ConfigPath) {
		return ApplyResponse{}, fmt.Errorf("tool %s does not support apply", toolID)
	}
	return s.resetFile(tool.ConfigPath)
}

func (s *Service) resetFile(path string) (ApplyResponse, error) {
	backupPath := path + ".aurora.bak"
	backup, err := s.fs.ReadFile(backupPath)
	if err != nil || len(backup) == 0 {
		return ApplyResponse{}, fmt.Errorf("no backup found for %s", path)
	}
	if err := s.fs.WriteFile(path, backup, 0o600); err != nil {
		return ApplyResponse{}, fmt.Errorf("restore config: %w", err)
	}
	return ApplyResponse{Applied: true, Path: path, BackupPath: backupPath}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) toolAndRequest(toolID string, req PreviewRequest) (Tool, PreviewRequest, error) {
	tool, ok := s.GetTool(toolID)
	if !ok {
		return Tool{}, PreviewRequest{}, fmt.Errorf("unknown CLI tool: %s", toolID)
	}
	normalized, err := normalizePreview(tool, req)
	if err != nil {
		return Tool{}, PreviewRequest{}, err
	}
	return tool, normalized, nil
}

func toolDefinitions(applyEnabled bool, home string) []toolDefinition {
	canApplyToHome := applyEnabled && strings.TrimSpace(home) != ""
	return []toolDefinition{
		{
			Tool: Tool{ID: "claude-code", Name: "Claude Code", Description: "Anthropic Claude Code CLI.", ConfigPath: filepath.Join(home, ".claude", "settings.json"), CanApply: canApplyToHome, ConfigType: "json", Color: "#D97757", DocsURL: "https://code.claude.com/docs/en/settings", DefaultCommand: "claude", Notes: []string{"Config path: Linux/macOS ~/.claude/settings.json • Windows %USERPROFILE%\\.claude\\settings.json", "Sets ANTHROPIC_BASE_URL and ANTHROPIC_AUTH_TOKEN in Claude Code's env block for gateway routing.", "Adds Claude Code model, availableModels, default model environment values, and conservative secret-file deny permissions."}, ModelFields: []ModelField{
				{Key: "ANTHROPIC_MODEL", Label: "Primary model", Description: "Default Claude Code model used by /model, --model, and ANTHROPIC_MODEL."},
				{Key: "ANTHROPIC_DEFAULT_HAIKU_MODEL", Label: "Haiku default", Description: "Fast, low-latency Claude Code default model."},
				{Key: "ANTHROPIC_DEFAULT_SONNET_MODEL", Label: "Sonnet default", Description: "Balanced Claude Code default model."},
				{Key: "ANTHROPIC_DEFAULT_OPUS_MODEL", Label: "Opus default", Description: "Highest-capability Claude Code default model."},
				{Key: "AVAILABLE_MODELS", Label: "Available models", Description: "Models listed in availableModels (multi-select). Empty = primary + tier defaults.", Multi: true},
			}},
			Snippet: claudeCodeSnippets,
		},
		{
			Tool: Tool{ID: "codex", Name: "OpenAI Codex CLI / App", Description: "OpenAI Codex CLI provider configuration.", ConfigPath: filepath.Join(home, ".codex", "aurora-config.toml"), AuthConfigPath: filepath.Join(home, ".codex", "auth.json"), CanApply: canApplyToHome, ConfigType: "custom", Color: "#10A37F", Notes: []string{"Apply writes aurora-config.toml and auth.json; add `include = \"aurora-config.toml\"` to ~/.codex/config.toml to load it."}, ModelFields: []ModelField{
				{Key: "CODEX_MODEL", Label: "Codex model", Description: "Primary model used by Codex CLI."},
				{Key: "CODEX_SUBAGENT_MODEL", Label: "Codex subagent model", Description: "Model used by Codex subagents."},
			}},
			Snippet: codexSnippets,
		},
		{
			Tool:    Tool{ID: "opencode", Name: "OpenCode", Description: "OpenCode coding agent configured against the AuroraX gateway.", ConfigPath: filepath.Join(home, ".config", "opencode", "opencode.json"), CanApply: canApplyToHome, ConfigType: "json", Color: "#E87040", DefaultCommand: "opencode", DocsURL: "https://opencode.ai/docs/config/", Notes: []string{"Config path: Linux/macOS ~/.config/opencode/opencode.json", "Registers AuroraX as an openai-compatible provider; the primary model is referenced as aurorax/<model>."}, ModelFields: singleModelFields("OPENCODE_MODEL", "Primary model", "Default OpenCode model routed through AuroraX.")},
			Snippet: openCodeSnippets,
		},
		{
			Tool:    Tool{ID: "openclaw", Name: "Open Claw", Description: "Open Claw AI assistant using OpenAI-compatible environment variables.", CanApply: false, ConfigType: "custom", Color: "#FF6B35", ModelFields: singleModelFields("OPENCLAW_MODEL", "OpenClaw primary model", "Primary model configured for OpenClaw agents.")},
			Snippet: openClawSnippets,
		},
		{
			Tool:    Tool{ID: "cursor", Name: "Cursor", Description: "Cursor AI code editor OpenAI-compatible setup guide.", CanApply: false, ConfigType: "guide", Color: "#000000", Notes: []string{"Requires Cursor Pro account.", "Cursor routes requests through its own server; use a public tunnel/cloud URL rather than localhost."}, ModelFields: singleModelFields("CURSOR_MODEL", "Cursor custom model", "Custom model to add in Cursor settings.")},
			Snippet: cursorSnippets,
		},
		{
			Tool:    Tool{ID: "cline", Name: "Cline", Description: "Cline AI coding assistant setup.", CanApply: false, ConfigType: "custom", Color: "#00D1B2", ModelFields: singleModelFields("CLINE_MODEL", "Cline model", "Model used in Cline's OpenAI-compatible provider config.")},
			Snippet: clineSnippets,
		},
		{
			Tool:    Tool{ID: "kilo", Name: "Kilo Code", Description: "Kilo Code AI assistant setup.", CanApply: false, ConfigType: "custom", Color: "#FF6B6B", ModelFields: singleModelFields("KILO_MODEL", "Kilo model", "Model configured for the openai-compatible provider.")},
			Snippet: kiloSnippets,
		},
		{
			Tool:    Tool{ID: "roo", Name: "Roo", Description: "Roo AI assistant setup guide.", CanApply: false, ConfigType: "guide", Color: "#FF6B6B", ModelFields: singleModelFields("ROO_MODEL", "Roo model", "Model to enter in Roo settings.")},
			Snippet: rooSnippets,
		},
		{
			Tool:    Tool{ID: "continue", Name: "Continue", Description: "Continue AI assistant model configuration.", CanApply: false, ConfigType: "guide", Color: "#7C3AED", ModelFields: singleModelFields("CONTINUE_MODEL", "Continue model", "Model and title used in Continue config.")},
			Snippet: continueSnippets,
		},
		{
			Tool:    Tool{ID: "amp", Name: "Amp CLI", Description: "Sourcegraph Amp coding assistant CLI.", CanApply: false, ConfigType: "guide", Color: "#F97316", DefaultCommand: "amp", Notes: []string{"Use stable shorthand mappings for model aliases when your local Amp config supports them."}, ModelFields: singleModelFields("AMP_MODEL", "Amp model", "Model passed to amp --model.")},
			Snippet: ampSnippets,
		},
		{
			Tool:    Tool{ID: "qwen", Name: "Qwen Code", Description: "Alibaba Qwen Code CLI using AuroraX as an OpenAI-compatible endpoint.", CanApply: false, ConfigType: "guide", Color: "#10B981", DefaultCommand: "qwen", DocsURL: "https://qwenlm.github.io/qwen-code-docs/en/users/configuration/model-providers/", Notes: []string{"Config path: Linux/macOS ~/.qwen/settings.json • Windows %USERPROFILE%\\.qwen\\settings.json", "Qwen Code can use any Aurora model through the OpenAI-compatible provider."}, ModelFields: singleModelFields("QWEN_MODEL", "Qwen model", "Model name used in Qwen Code settings.")},
			Snippet: qwenSnippets,
		},
		{
			Tool:    Tool{ID: "deepseek-tui", Name: "DeepSeek TUI", Description: "DeepSeek terminal coding agent Rust TUI.", CanApply: false, ConfigType: "custom", Color: "#4D6BFE", DefaultCommand: "deepseek", DocsURL: "https://github.com/DeepSeek-TUI/DeepSeek-TUI", Notes: []string{"Config path: Linux/macOS ~/.deepseek/config.toml • Windows %USERPROFILE%\\.deepseek\\config.toml"}, ModelFields: singleModelFields("DEEPSEEK_TUI_MODEL", "DeepSeek TUI model", "Model used in the DeepSeek TUI provider config.")},
			Snippet: deepseekTUISnippets,
		},
		{
			Tool:    Tool{ID: "jcode", Name: "jcode", Description: "High-performance Rust-based coding agent harness.", CanApply: false, ConfigType: "guide", Color: "#FF6B35", DocsURL: "https://github.com/1jehuang/jcode", Notes: []string{"Configure AuroraX as an OpenAI-compatible provider."}, ModelFields: singleModelFields("JCODE_MODEL", "jcode model", "Model used in jcode provider config.")},
			Snippet: jcodeSnippets,
		},
		{
			Tool:    Tool{ID: "generic", Name: "Generic OpenAI Compatible", Description: "Copy generic OpenAI-compatible environment variables.", CanApply: false, ConfigType: "env", ModelFields: singleModelFields("OPENAI_MODEL", "OpenAI model", "Model exported through OPENAI_MODEL.")},
			Snippet: openAIEnvSnippets,
		},
	}
}

func builtInPresets() []ToolPreset {
	return []ToolPreset{
		{
			ID:          "claude-balanced",
			Label:       "Claude Code — balanced",
			Description: "Claude Code through Aurora with Sonnet and Haiku defaults.",
			ToolID:      "claude-code",
			Model:       "claude-sonnet",
			ModelOverrides: map[string]string{
				"ANTHROPIC_DEFAULT_SONNET_MODEL": "claude-sonnet",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "claude-haiku",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
		{
			ID:          "codex-default",
			Label:       "Codex — default",
			Description: "OpenAI Codex CLI pointed at the Aurora gateway.",
			ToolID:      "codex",
			Model:       "gpt-5-codex",
			ModelOverrides: map[string]string{
				"CODEX_MODEL":          "gpt-5-codex",
				"CODEX_SUBAGENT_MODEL": "gpt-5-codex",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
		{
			ID:          "opencode-default",
			Label:       "OpenCode — default",
			Description: "OpenCode agent using AuroraX as an openai-compatible provider.",
			ToolID:      "opencode",
			Model:       "claude-sonnet",
			ModelOverrides: map[string]string{
				"OPENCODE_MODEL": "claude-sonnet",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
		{
			ID:          "openclaw-default",
			Label:       "Open Claw — default",
			Description: "Open Claw agent defaults routed through Aurora.",
			ToolID:      "openclaw",
			Model:       "claude-sonnet",
			ModelOverrides: map[string]string{
				"OPENCLAW_MODEL": "claude-sonnet",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
		{
			ID:          "qwen-default",
			Label:       "Qwen Code — default",
			Description: "Qwen Code using Aurora as an OpenAI-compatible provider.",
			ToolID:      "qwen",
			Model:       "qwen3-coder",
			ModelOverrides: map[string]string{
				"QWEN_MODEL": "qwen3-coder",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
		{
			ID:          "generic-openai",
			Label:       "Generic OpenAI-compatible",
			Description: "Export OPENAI_* environment variables for any OpenAI-compatible client.",
			ToolID:      "generic",
			Model:       "gpt-4o-mini",
			ModelOverrides: map[string]string{
				"OPENAI_MODEL": "gpt-4o-mini",
			},
			APIKeyPlaceholder: apiKeyPlaceholder,
		},
	}
}

func snippetsFor(toolID string, req PreviewRequest) map[string]string {
	for _, definition := range toolDefinitions(false, homeDir()) {
		if definition.ID == toolID {
			return definition.Snippet(req)
		}
	}
	return openAIEnvSnippets(req)
}

func singleModelFields(key string, label string, description string) []ModelField {
	return []ModelField{{Key: key, Label: label, Description: description}}
}

func claudeCodeSnippets(req PreviewRequest) map[string]string {
	base := trimSlash(req.BaseURL)
	primaryModel := modelForField(req, "ANTHROPIC_MODEL")
	haikuModel := modelForField(req, "ANTHROPIC_DEFAULT_HAIKU_MODEL")
	sonnetModel := modelForField(req, "ANTHROPIC_DEFAULT_SONNET_MODEL")
	opusModel := modelForField(req, "ANTHROPIC_DEFAULT_OPUS_MODEL")
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":           req.APIKey,
		"ANTHROPIC_BASE_URL":             base,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  haikuModel,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   opusModel,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": sonnetModel,
		"ANTHROPIC_MODEL":                primaryModel,
		"API_TIMEOUT_MS":                 "600000",
	}
	// availableModels: explicit multi-select list wins; otherwise tier defaults.
	available := req.Models
	if len(available) == 0 {
		available = uniqueStrings(primaryModel, haikuModel, sonnetModel, opusModel)
	} else {
		available = uniqueStrings(append([]string{primaryModel}, available...)...)
	}
	cfg := map[string]any{
		"$schema":         "https://json.schemastore.org/claude-code-settings.json",
		"model":           primaryModel,
		"availableModels": available,
		"env":             env,
		"permissions": map[string]any{
			"defaultMode": "default",
			"deny": []string{
				"Read(./.env)",
				"Read(./.env.*)",
				"Read(./secrets/**)",
				"Read(~/.aws/credentials)",
				"Read(~/.config/gcloud/**)",
			},
		},
		"enableAllProjectMcpServers": false,
	}
	return map[string]string{"env": envBlock(env), "config": jsonBlock(cfg)}
}

func codexSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "CODEX_MODEL")
	subagentModel := modelForField(req, "CODEX_SUBAGENT_MODEL")
	config := fmt.Sprintf("model = %s\nmodel_provider = \"aurora\"\n\n[model_providers.aurora]\nname = \"Aurora Gateway\"\nbase_url = %s\nwire_api = \"responses\"\n\n[agents.subagent]\nmodel = %s", tomlString(model), tomlString(baseV1(req)), tomlString(subagentModel))
	auth := jsonBlock(map[string]string{"auth_mode": "apikey", "OPENAI_API_KEY": req.APIKey})
	return map[string]string{"config": config, "auth": auth}
}

func cursorSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "CURSOR_MODEL")
	return map[string]string{"guide": strings.Join([]string{"1. Open Settings → Models.", "2. Enable OpenAI API key.", "3. Base URL: " + baseV1(req), "4. API key: " + req.APIKey, "5. Add custom model: " + model}, "\n")}
}

func clineSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "CLINE_MODEL")
	cfg := map[string]any{"provider": "openai-compatible", "baseUrl": baseV1(req), "apiKey": req.APIKey, "model": model}
	return map[string]string{"config": jsonBlock(cfg), "guide": "Choose API Provider → OpenAI Compatible, then paste the base URL, API key, and model."}
}

func kiloSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "KILO_MODEL")
	cfg := map[string]any{"openai-compatible": map[string]string{"type": "api-key", "apiKey": req.APIKey, "baseUrl": baseV1(req), "model": model}}
	return map[string]string{"config": jsonBlock(cfg)}
}

func openClawSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "OPENCLAW_MODEL")
	cfg := map[string]any{
		"agents": map[string]any{
			"defaults": map[string]any{
				"model": map[string]string{"primary": "aurora/" + model},
			},
		},
		"models": map[string]any{
			"providers": map[string]any{
				"aurorax": map[string]any{
					"baseUrl": baseV1(req),
					"apiKey":  req.APIKey,
					"api":     "openai-completions",
					"models":  []map[string]string{{"id": model, "name": modelName(model)}},
				},
			},
		},
	}
	return map[string]string{"config": jsonBlock(cfg)}
}

func rooSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "ROO_MODEL")
	return map[string]string{"guide": strings.Join([]string{"1. Open Roo Settings panel.", "2. Choose API Provider → Ollama or OpenAI Compatible.", "3. Base URL: " + baseV1(req), "4. API key: " + req.APIKey, "5. Model: " + model}, "\n")}
}

func continueSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "CONTINUE_MODEL")
	cfg := map[string]any{"apiBase": baseV1(req), "title": model, "model": model, "provider": "openai", "apiKey": req.APIKey}
	return map[string]string{"config": jsonBlock(cfg)}
}

func ampSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "AMP_MODEL")
	return map[string]string{"env": envBlock(map[string]string{"OPENAI_API_KEY": req.APIKey, "OPENAI_BASE_URL": baseV1(req), "OPENAI_MODEL": model}), "command": fmt.Sprintf("amp --model %q", model)}
}

func qwenSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "QWEN_MODEL")
	cfg := map[string]any{"security": map[string]any{"auth": map[string]string{"selectedType": "openai", "apiKey": req.APIKey, "baseUrl": baseV1(req)}}, "model": map[string]string{"name": model}}
	return map[string]string{"config": jsonBlock(cfg)}
}

func deepseekTUISnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "DEEPSEEK_TUI_MODEL")
	return map[string]string{"config": fmt.Sprintf("[provider]\ntype = \"openai\"\nbase_url = %s\napi_key = %s\nmodel = %s", tomlString(baseV1(req)), tomlString(req.APIKey), tomlString(model))}
}

func jcodeSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "JCODE_MODEL")
	cfg := map[string]any{"providers": map[string]any{"aurorax": map[string]string{"type": "openai-compatible", "base_url": baseV1(req), "api_key": req.APIKey}}, "model": model}
	return map[string]string{"config": jsonBlock(cfg)}
}

func openCodeSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "OPENCODE_MODEL")
	models := uniqueStrings(append([]string{model}, req.Models...)...)
	modelMap := make(map[string]any, len(models))
	for _, m := range models {
		modelMap[m] = map[string]string{"name": modelName(m)}
	}
	cfg := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   "aurorax/" + model,
		"provider": map[string]any{
			"aurorax": map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "AuroraX Gateway",
				"options": map[string]string{
					"baseURL": baseV1(req),
					"apiKey":  req.APIKey,
				},
				"models": modelMap,
			},
		},
	}
	return map[string]string{"config": jsonBlock(cfg)}
}

func openAIEnvSnippets(req PreviewRequest) map[string]string {
	model := modelForField(req, "OPENAI_MODEL")
	return map[string]string{"env": envBlock(map[string]string{"OPENAI_API_KEY": req.APIKey, "OPENAI_BASE_URL": baseV1(req), "OPENAI_MODEL": model})}
}

func normalizePreview(tool Tool, req PreviewRequest) (PreviewRequest, error) {
	normalized := PreviewRequest{
		BaseURL: strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
		APIKey:  strings.TrimSpace(req.APIKey),
		Model:   strings.TrimSpace(req.Model),
	}
	if normalized.BaseURL == "" {
		normalized.BaseURL = "http://localhost:8080"
	}
	parsed, err := url.Parse(normalized.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return PreviewRequest{}, fmt.Errorf("base_url must be an http or https URL without credentials")
	}
	if normalized.Model == "" {
		normalized.Model = "auto"
	}
	if len(normalized.BaseURL) > 2048 || len(normalized.APIKey) > 4096 || len(normalized.Model) > 256 {
		return PreviewRequest{}, fmt.Errorf("cli tool values exceed allowed length")
	}
	for _, value := range []string{normalized.BaseURL, normalized.APIKey, normalized.Model} {
		if hasControl(value) {
			return PreviewRequest{}, fmt.Errorf("cli tool values must not contain control characters")
		}
	}
	if !safeModelName(normalized.Model) {
		return PreviewRequest{}, fmt.Errorf("model must contain only letters, numbers, slash, colon, dot, dash, underscore, at sign, or spaces")
	}
	overrides, err := normalizeModelOverrides(tool, req.ModelOverrides)
	if err != nil {
		return PreviewRequest{}, err
	}
	normalized.ModelOverrides = overrides
	normalized.Models = normalizeModels(req.Models)
	return normalized, nil
}

func normalizeModelOverrides(tool Tool, overrides map[string]string) (map[string]string, error) {
	if len(overrides) == 0 {
		return nil, nil
	}
	if len(overrides) > 20 {
		return nil, fmt.Errorf("model_overrides must contain 20 or fewer entries")
	}
	allowedKeys := modelFieldKeys(tool.ModelFields)
	// Multi fields only receive models via PreviewRequest.Models, not overrides.
	for _, f := range tool.ModelFields {
		if f.Multi {
			delete(allowedKeys, f.Key)
		}
	}
	normalized := make(map[string]string, len(overrides))
	for rawKey, rawValue := range overrides {
		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}
		if !safeModelOverrideKey(key) {
			return nil, fmt.Errorf("model override key must contain only uppercase letters, numbers, or underscore")
		}
		if len(allowedKeys) > 0 && !allowedKeys[key] {
			return nil, fmt.Errorf("unknown model override key: %s", key)
		}
		if len(value) > 256 || hasControl(value) {
			return nil, fmt.Errorf("model override values exceed allowed length or contain control characters")
		}
		if !safeModelName(value) {
			return nil, fmt.Errorf("model override must contain only letters, numbers, slash, colon, dot, dash, underscore, at sign, or spaces")
		}
		normalized[key] = value
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func modelFieldKeys(fields []ModelField) map[string]bool {
	keys := make(map[string]bool, len(fields))
	for _, field := range fields {
		keys[field.Key] = true
	}
	return keys
}

func modelForField(req PreviewRequest, key string) string {
	if value := strings.TrimSpace(req.ModelOverrides[key]); value != "" {
		return value
	}
	// Multi-select list: first entry becomes the fallback when no override.
	if key == "AVAILABLE_MODELS" && len(req.Models) > 0 {
		return req.Models[0]
	}
	return req.Model
}

func normalizeModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]bool)
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || m == "auto" || seen[m] {
			continue
		}
		if !safeModelName(m) {
			continue
		}
		if len(m) > 256 || hasControl(m) {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func modelName(model string) string {
	parts := strings.Split(strings.TrimSpace(model), "/")
	if len(parts) == 0 {
		return model
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return model
	}
	return name
}

// mergeJSONSettings folds the generated config into the existing one:
// top-level generated keys win, while the named nested maps (e.g. "env",
// "provider") are merged key-by-key so unrelated entries survive.
func mergeJSONSettings(existing []byte, generated string, nestedMaps ...string) (string, error) {
	var existingConfig map[string]any
	if err := json.Unmarshal(existing, &existingConfig); err != nil {
		return "", fmt.Errorf("merge settings: existing config is invalid JSON: %w", err)
	}
	var generatedConfig map[string]any
	if err := json.Unmarshal([]byte(generated), &generatedConfig); err != nil {
		return "", fmt.Errorf("merge settings: generated config is invalid JSON: %w", err)
	}
	nested := make(map[string]bool, len(nestedMaps))
	for _, key := range nestedMaps {
		nested[key] = true
	}
	merged := make(map[string]any, len(existingConfig)+len(generatedConfig))
	for key, value := range existingConfig {
		merged[key] = value
	}
	for key, value := range generatedConfig {
		if !nested[key] {
			merged[key] = value
			continue
		}
		combined := map[string]any{}
		if existingMap, ok := existingConfig[key].(map[string]any); ok {
			for subKey, subValue := range existingMap {
				combined[subKey] = subValue
			}
		}
		if generatedMap, ok := value.(map[string]any); ok {
			for subKey, subValue := range generatedMap {
				combined[subKey] = subValue
			}
		}
		merged[key] = combined
	}
	return jsonBlock(merged), nil
}

func uniqueStrings(values ...string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		unique = append(unique, trimmed)
	}
	return unique
}

func jsonBlock(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func envBlock(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}
	return strings.Join(lines, "\n")
}

func tomlString(value string) string { return fmt.Sprintf("%q", value) }
func trimSlash(value string) string  { return strings.TrimRight(strings.TrimSpace(value), "/") }
func baseV1(req PreviewRequest) string {
	return trimSlash(req.BaseURL) + "/v1"
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func safeModelName(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '/', ':', '.', '-', '_', '@', ' ':
			continue
		default:
			return false
		}
	}
	return value != ""
}

func safeModelOverrideKey(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}
