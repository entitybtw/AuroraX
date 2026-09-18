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

func (s *Service) Preview(toolID string, req PreviewRequest) (PreviewResponse, error) {
	tool, req, err := s.toolAndRequest(toolID, req)
	if err != nil {
		return PreviewResponse{}, err
	}
	redacted := req
	redacted.APIKey = maskKey(req.APIKey)
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
	content := snippetsFor(tool.ID, req)["config"]
	if content == "" {
		return ApplyResponse{}, fmt.Errorf("tool %s does not provide an applyable config snippet", toolID)
	}
	if err := s.fs.MkdirAll(filepath.Dir(tool.ConfigPath), 0o700); err != nil {
		return ApplyResponse{}, fmt.Errorf("create config directory: %w", err)
	}
	backup := ""
	existing, readErr := s.fs.ReadFile(tool.ConfigPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return ApplyResponse{}, fmt.Errorf("read existing config: %w", readErr)
	}
	if readErr == nil && len(existing) > 0 {
		backup = tool.ConfigPath + ".aurora.bak"
		if err := s.fs.WriteFile(backup, existing, 0o600); err != nil {
			return ApplyResponse{}, fmt.Errorf("write backup: %w", err)
		}
	}
	if tool.ID == "claude-code" && len(existing) > 0 {
		content, err = mergeClaudeCodeSettings(existing, content)
		if err != nil {
			return ApplyResponse{}, err
		}
	}
	if err := s.fs.WriteFile(tool.ConfigPath, []byte(content+"\n"), 0o600); err != nil {
		return ApplyResponse{}, fmt.Errorf("write config: %w", err)
	}
	return ApplyResponse{Applied: true, Path: tool.ConfigPath, BackupPath: backup}, nil
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
			}},
			Snippet: claudeCodeSnippets,
		},
		{
			Tool: Tool{ID: "codex", Name: "OpenAI Codex CLI / App", Description: "OpenAI Codex CLI provider configuration.", ConfigPath: filepath.Join(home, ".codex", "aurora-config.toml"), CanApply: canApplyToHome, ConfigType: "custom", Color: "#10A37F", ModelFields: []ModelField{
				{Key: "CODEX_MODEL", Label: "Codex model", Description: "Primary model used by Codex CLI."},
				{Key: "CODEX_SUBAGENT_MODEL", Label: "Codex subagent model", Description: "Model used by Codex subagents."},
			}},
			Snippet: codexSnippets,
		},
		{
			Tool: Tool{ID: "opencode", Name: "OpenCode", Description: "OpenCode AI terminal assistant provider configuration.", ConfigPath: filepath.Join(home, ".config", "opencode", "opencode.json"), CanApply: canApplyToHome, ConfigType: "custom", Color: "#E87040", DocsURL: "https://opencode.ai/docs/configuration", Notes: []string{"Config path: ~/.config/opencode/opencode.json", "Adds Aurora as a multi-model provider.", "Select multiple models below — they'll all be available via the aurora/ prefix in OpenCode."}, ModelFields: []ModelField{
				{Key: "OPENCODE_MODEL", Label: "Models", Description: "Select model(s) to make available in OpenCode under the AuroraX provider.", Multi: true},
			}},
			Snippet: opencodeSnippets,
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
	cfg := map[string]any{
		"$schema":         "https://json.schemastore.org/claude-code-settings.json",
		"model":           primaryModel,
		"availableModels": uniqueStrings(primaryModel, haikuModel, sonnetModel, opusModel),
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

func opencodeSnippets(req PreviewRequest) map[string]string {
	models := make(map[string]any)
	primaryModel := req.Model
	for _, m := range req.Models {
		if primaryModel == "auto" || primaryModel == "" {
			primaryModel = m
		}
		models[m] = map[string]any{
			"name": m,
			"modalities": map[string][]string{
				"input":  {"text", "image"},
				"output": {"text"},
			},
		}
	}
	cfg := map[string]any{
		"provider": map[string]any{
			"aurorax": map[string]any{
				"npm": "@ai-sdk/openai-compatible",
				"name": "AuroraX Gateway",
				"options": map[string]string{
					"baseURL": baseV1(req),
					"apiKey":  req.APIKey,
				},
				"models": models,
			},
		},
	}
	if primaryModel != "" && primaryModel != "auto" {
		cfg["model"] = "aurora/" + primaryModel
	}
	return map[string]string{"config": jsonBlock(cfg)}
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

func mergeClaudeCodeSettings(existing []byte, generated string) (string, error) {
	var existingConfig map[string]any
	if err := json.Unmarshal(existing, &existingConfig); err != nil {
		return "", fmt.Errorf("merge Claude Code settings: existing settings.json is invalid JSON: %w", err)
	}
	var generatedConfig map[string]any
	if err := json.Unmarshal([]byte(generated), &generatedConfig); err != nil {
		return "", fmt.Errorf("merge Claude Code settings: generated config is invalid JSON: %w", err)
	}
	merged := make(map[string]any, len(existingConfig)+len(generatedConfig))
	for key, value := range existingConfig {
		merged[key] = value
	}
	for key, value := range generatedConfig {
		if key == "env" {
			continue
		}
		merged[key] = value
	}
	env := map[string]any{}
	if existingEnv, ok := existingConfig["env"].(map[string]any); ok {
		for key, value := range existingEnv {
			env[key] = value
		}
	}
	if generatedEnv, ok := generatedConfig["env"].(map[string]any); ok {
		for key, value := range generatedEnv {
			env[key] = value
		}
	}
	merged["env"] = env
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
