package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurora/internal/core"
	"aurora/internal/response_cache"
)

// Definition is one persisted reusable guardrail instance.
type Definition struct {
	Name        string          `json:"name" bson:"name"`
	Type        string          `json:"type" bson:"type"`
	Direction   string          `json:"direction,omitempty" bson:"direction,omitempty"`
	Description string          `json:"description,omitempty" bson:"description,omitempty"`
	UserPath    string          `json:"user_path,omitempty" bson:"user_path,omitempty"`
	Config      json.RawMessage `json:"config" bson:"config"`
	CreatedAt   time.Time       `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" bson:"updated_at"`
}

// View is the admin-facing representation of a persisted guardrail.
type View struct {
	Definition
	Summary string `json:"summary,omitempty"`
}

// ViewFromDefinition projects one guardrail definition into its admin-facing view.
func ViewFromDefinition(def Definition) View {
	return View{
		Definition: cloneDefinition(def),
		Summary:    summarizeDefinition(def),
	}
}

// TypeOption is one allowed option for a typed guardrail config field.
type TypeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TypeField describes one UI field for a guardrail type.
type TypeField struct {
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Input       string       `json:"input"`
	Required    bool         `json:"required"`
	Help        string       `json:"help,omitempty"`
	Placeholder string       `json:"placeholder,omitempty"`
	Options     []TypeOption `json:"options,omitempty"`
}

// TypeDefinition describes one supported guardrail type and its config schema.
type TypeDefinition struct {
	Type        string          `json:"type"`
	Label       string          `json:"label"`
	Description string          `json:"description,omitempty"`
	Defaults    json.RawMessage `json:"defaults"`
	Fields      []TypeField     `json:"fields"`
}

type systemPromptDefinitionConfig struct {
	Mode    string `json:"mode"`
	Content string `json:"content"`
	OnError string `json:"on_error,omitempty"`
}

type llmBasedAlteringDefinitionConfig struct {
	Model             string   `json:"model"`
	Provider          string   `json:"provider,omitempty"`
	Prompt            string   `json:"prompt,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	SkipContentPrefix string   `json:"skip_content_prefix,omitempty"`
	MaxTokens         int      `json:"max_tokens,omitempty"`
	OnError           string   `json:"on_error,omitempty"`
}

type regexBlockDefinitionConfig struct {
	Action      string   `json:"action,omitempty"`
	Patterns    []string `json:"patterns"`
	Replacement string   `json:"replacement,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	OnError     string   `json:"on_error,omitempty"`
}

type piiRedactDefinitionConfig struct {
	Kinds   []string `json:"kinds,omitempty"`
	Roles   []string `json:"roles,omitempty"`
	OnError string   `json:"on_error,omitempty"`
}

type lengthLimitDefinitionConfig struct {
	MaxChars           int    `json:"max_chars,omitempty"`
	MaxEstimatedTokens int    `json:"max_estimated_tokens,omitempty"`
	OnError            string `json:"on_error,omitempty"`
}

var disableAdvancedGuardrails bool

func DisableAdvancedGuardrails() {
	disableAdvancedGuardrails = true
}

func normalizeDefinition(def Definition) (Definition, error) {
	def.Name = strings.TrimSpace(def.Name)
	def.Type = normalizeDefinitionType(def.Type)
	def.Direction = normalizeDirection(def.Direction)
	def.Description = strings.TrimSpace(def.Description)
	userPath, err := core.NormalizeUserPath(def.UserPath)
	if err != nil {
		return Definition{}, newValidationError("invalid user_path", err)
	}
	def.UserPath = userPath

	if def.Name == "" {
		return Definition{}, newValidationError("guardrail name is required", nil)
	}
	if strings.Contains(def.Name, "/") {
		return Definition{}, newValidationError("guardrail name cannot contain '/'", nil)
	}
	if def.Type == "" {
		return Definition{}, newValidationError("guardrail type is required", nil)
	}

	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	case "llm_based_altering":
		if disableAdvancedGuardrails {
			return Definition{}, newValidationError(`guardrail type "llm_based_altering" is not available in this edition`, nil)
		}
		cfg, err := decodeLLMBasedAlteringDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	case "regex_block":
		cfg, err := decodeRegexBlockDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	case "pii_redact":
		cfg, err := decodePIIRedactDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	case "length_limit":
		cfg, err := decodeLengthLimitDefinitionConfig(def.Config)
		if err != nil {
			return Definition{}, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return Definition{}, newValidationError("marshal guardrail config", err)
		}
		def.Config = raw
	default:
		return Definition{}, newValidationError(`unknown guardrail type: "`+def.Type+`"`, nil)
	}

	return def, nil
}

func normalizeDefinitionType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "system-prompt":
		return "system_prompt"
	case "llm-based-altering":
		return "llm_based_altering"
	case "regex-block":
		return "regex_block"
	case "pii-redact":
		return "pii_redact"
	case "length-limit":
		return "length_limit"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func cloneDefinition(def Definition) Definition {
	cloned := def
	if len(def.Config) > 0 {
		cloned.Config = append(json.RawMessage(nil), def.Config...)
	}
	return cloned
}

func cloneTypeDefinitions(defs []TypeDefinition) []TypeDefinition {
	if len(defs) == 0 {
		return []TypeDefinition{}
	}
	cloned := make([]TypeDefinition, 0, len(defs))
	for _, def := range defs {
		copyDef := def
		if len(def.Defaults) > 0 {
			copyDef.Defaults = append(json.RawMessage(nil), def.Defaults...)
		}
		if len(def.Fields) > 0 {
			copyDef.Fields = append([]TypeField(nil), def.Fields...)
			for i := range copyDef.Fields {
				if len(copyDef.Fields[i].Options) > 0 {
					copyDef.Fields[i].Options = append([]TypeOption(nil), copyDef.Fields[i].Options...)
				}
			}
		}
		cloned = append(cloned, copyDef)
	}
	return cloned
}

func decodeSystemPromptDefinitionConfig(raw json.RawMessage) (systemPromptDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}

	var cfg systemPromptDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return systemPromptDefinitionConfig{}, newValidationError("invalid system_prompt config: "+err.Error(), err)
	}
	if decoder.More() {
		return systemPromptDefinitionConfig{}, newValidationError("invalid system_prompt config: trailing data", nil)
	}

	cfg.Mode = effectiveSystemPromptMode(cfg.Mode)
	if !isValidSystemPromptMode(cfg.Mode) {
		return systemPromptDefinitionConfig{}, newValidationError("system_prompt mode is invalid", nil)
	}
	cfg.Content = strings.TrimSpace(cfg.Content)
	if cfg.Content == "" {
		return systemPromptDefinitionConfig{}, newValidationError("system_prompt content is required", nil)
	}
	onError, err := effectiveOnError(cfg.OnError, OnErrorBlock)
	if err != nil {
		return systemPromptDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.OnError = onError
	return cfg, nil
}

func decodeLLMBasedAlteringDefinitionConfig(raw json.RawMessage) (llmBasedAlteringDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}

	var cfg llmBasedAlteringDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return llmBasedAlteringDefinitionConfig{}, newValidationError("invalid llm_based_altering config: "+err.Error(), err)
	}
	if decoder.More() {
		return llmBasedAlteringDefinitionConfig{}, newValidationError("invalid llm_based_altering config: trailing data", nil)
	}

	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		return llmBasedAlteringDefinitionConfig{}, newValidationError("llm_based_altering model is required", nil)
	}
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	selector, err := core.ParseModelSelector(cfg.Model, cfg.Provider)
	if err != nil {
		return llmBasedAlteringDefinitionConfig{}, newValidationError("invalid llm_based_altering model selector: "+err.Error(), err)
	}
	cfg.Model = selector.QualifiedModel()
	cfg.Provider = ""
	cfg.Prompt = strings.TrimSpace(cfg.Prompt)
	cfg.SkipContentPrefix = strings.TrimSpace(cfg.SkipContentPrefix)
	cfg.MaxTokens = EffectiveLLMBasedAlteringMaxTokens(cfg.MaxTokens)

	roles, err := NormalizeLLMBasedAlteringRoles(cfg.Roles)
	if err != nil {
		return llmBasedAlteringDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.Roles = roles
	onError, err := effectiveOnError(cfg.OnError, OnErrorBlock)
	if err != nil {
		return llmBasedAlteringDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.OnError = onError
	return cfg, nil
}

func decodeRegexBlockDefinitionConfig(raw json.RawMessage) (regexBlockDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}
	var cfg regexBlockDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return regexBlockDefinitionConfig{}, newValidationError("invalid regex_block config: "+err.Error(), err)
	}
	cfg.Action = EffectiveRegexBlockAction(cfg.Action)
	if !IsValidRegexBlockAction(cfg.Action) {
		return regexBlockDefinitionConfig{}, newValidationError("regex_block action is invalid", nil)
	}
	if len(cfg.Patterns) == 0 {
		return regexBlockDefinitionConfig{}, newValidationError("regex_block patterns are required", nil)
	}
	for i := range cfg.Patterns {
		cfg.Patterns[i] = strings.TrimSpace(cfg.Patterns[i])
	}
	cfg.Replacement = strings.TrimSpace(cfg.Replacement)
	cfg.Roles = normalizeStringList(cfg.Roles)
	if _, err := NewRegexBlockGuardrail("validate", RegexBlockConfig{Action: RegexBlockAction(cfg.Action), Patterns: cfg.Patterns, Replacement: cfg.Replacement, Roles: cfg.Roles}); err != nil {
		return regexBlockDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	onError, err := effectiveOnError(cfg.OnError, OnErrorBlock)
	if err != nil {
		return regexBlockDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.OnError = onError
	return cfg, nil
}

func decodePIIRedactDefinitionConfig(raw json.RawMessage) (piiRedactDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}
	var cfg piiRedactDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return piiRedactDefinitionConfig{}, newValidationError("invalid pii_redact config: "+err.Error(), err)
	}
	cfg.Kinds = normalizeStringList(normalizePIIKinds(cfg.Kinds))
	cfg.Roles = normalizeStringList(cfg.Roles)
	onError, err := effectiveOnError(cfg.OnError, OnErrorAllow)
	if err != nil {
		return piiRedactDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.OnError = onError
	return cfg, nil
}

func decodeLengthLimitDefinitionConfig(raw json.RawMessage) (lengthLimitDefinitionConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}
	var cfg lengthLimitDefinitionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return lengthLimitDefinitionConfig{}, newValidationError("invalid length_limit config: "+err.Error(), err)
	}
	if _, err := NewLengthLimitGuardrail("validate", LengthLimitConfig{MaxChars: cfg.MaxChars, MaxEstimatedTokens: cfg.MaxEstimatedTokens}); err != nil {
		return lengthLimitDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	onError, err := effectiveOnError(cfg.OnError, OnErrorBlock)
	if err != nil {
		return lengthLimitDefinitionConfig{}, newValidationError(err.Error(), err)
	}
	cfg.OnError = onError
	return cfg, nil
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func llmBasedAlteringRuntimeConfig(cfg llmBasedAlteringDefinitionConfig, userPath string) (LLMBasedAlteringConfig, error) {
	selector, err := core.ParseModelSelector(cfg.Model, cfg.Provider)
	if err != nil {
		return LLMBasedAlteringConfig{}, newValidationError("invalid llm_based_altering model selector: "+err.Error(), err)
	}
	return NormalizeLLMBasedAlteringConfig(LLMBasedAlteringConfig{
		Model:             selector.Model,
		Provider:          selector.Provider,
		UserPath:          userPath,
		Prompt:            cfg.Prompt,
		Roles:             cfg.Roles,
		SkipContentPrefix: cfg.SkipContentPrefix,
		MaxTokens:         cfg.MaxTokens,
	})
}

func buildDefinition(def Definition, executor ChatCompletionExecutor) (Guardrail, responsecache.GuardrailRuleDescriptor, error) {
	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		mode := SystemPromptMode(cfg.Mode)
		instance, err := NewSystemPromptGuardrail(def.Name, mode, cfg.Content)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build system_prompt guardrail: "+err.Error(), err)
		}
		return wrapErrorPolicy(instance, cfg.OnError), responsecache.GuardrailRuleDescriptor{
			Name:      def.Name,
			Type:      def.Type,
			Direction: def.Direction,
			Mode:      string(mode) + ":" + cfg.OnError,
			Content:   cfg.Content,
		}, nil
	case "llm_based_altering":
		if disableAdvancedGuardrails {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError(`guardrail type "llm_based_altering" is not available in this edition`, nil)
		}
		cfg, err := decodeLLMBasedAlteringDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		runtimeCfg, err := llmBasedAlteringRuntimeConfig(cfg, def.UserPath)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build llm_based_altering guardrail: "+err.Error(), err)
		}
		if executor == nil {
			instance := &unavailableGuardrail{
				name: def.Name,
				message: fmt.Sprintf(
					`guardrail %q of type "llm_based_altering" cannot execute because the auxiliary executor is not configured`,
					def.Name,
				),
			}
			return wrapErrorPolicy(instance, cfg.OnError), llmBasedAlteringDescriptor(def.Name, runtimeCfg, def.Direction), nil
		}
		instance, err := NewLLMBasedAlteringGuardrail(def.Name, runtimeCfg, executor)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build llm_based_altering guardrail: "+err.Error(), err)
		}
		return wrapErrorPolicy(instance, cfg.OnError), llmBasedAlteringDescriptor(def.Name, runtimeCfg, def.Direction), nil
	case "regex_block":
		cfg, err := decodeRegexBlockDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		instance, err := NewRegexBlockGuardrail(def.Name, RegexBlockConfig{Action: RegexBlockAction(cfg.Action), Patterns: cfg.Patterns, Replacement: cfg.Replacement, Roles: cfg.Roles})
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build regex_block guardrail: "+err.Error(), err)
		}
		return wrapErrorPolicy(instance, cfg.OnError), regexBlockDescriptor(def.Name, cfg, def.Direction), nil
	case "pii_redact":
		cfg, err := decodePIIRedactDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		instance := NewPIIRedactGuardrail(def.Name, PIIRedactConfig{Kinds: cfg.Kinds, Roles: cfg.Roles})
		return wrapErrorPolicy(instance, cfg.OnError), piiRedactDescriptor(def.Name, cfg, def.Direction), nil
	case "length_limit":
		cfg, err := decodeLengthLimitDefinitionConfig(def.Config)
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, err
		}
		instance, err := NewLengthLimitGuardrail(def.Name, LengthLimitConfig{MaxChars: cfg.MaxChars, MaxEstimatedTokens: cfg.MaxEstimatedTokens})
		if err != nil {
			return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError("build length_limit guardrail: "+err.Error(), err)
		}
		return wrapErrorPolicy(instance, cfg.OnError), lengthLimitDescriptor(def.Name, cfg, def.Direction), nil
	default:
		return nil, responsecache.GuardrailRuleDescriptor{}, newValidationError(`unknown guardrail type: "`+def.Type+`"`, nil)
	}
}

func summarizeDefinition(def Definition) string {
	switch def.Type {
	case "system_prompt":
		cfg, err := decodeSystemPromptDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		content := strings.Join(strings.Fields(cfg.Content), " ")
		const maxLen = 72
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		if content == "" {
			return cfg.Mode
		}
		return fmt.Sprintf("%s • %s", cfg.Mode, content)
	case "llm_based_altering":
		cfg, err := decodeLLMBasedAlteringDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		runtimeCfg, err := llmBasedAlteringRuntimeConfig(cfg, def.UserPath)
		if err != nil {
			return ""
		}
		target := runtimeCfg.Model
		if runtimeCfg.Provider != "" {
			target = runtimeCfg.Provider + "/" + runtimeCfg.Model
		}
		promptSummary := "default prompt"
		if strings.TrimSpace(cfg.Prompt) != "" {
			prompt := strings.Join(strings.Fields(runtimeCfg.Prompt), " ")
			const maxLen = 48
			if len(prompt) > maxLen {
				prompt = prompt[:maxLen-3] + "..."
			}
			if prompt != "" {
				promptSummary = prompt
			}
		}
		return fmt.Sprintf("%s • %s • %s", target, strings.Join(runtimeCfg.Roles, ","), promptSummary)
	case "regex_block":
		cfg, err := decodeRegexBlockDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%s • %d pattern(s)", cfg.Action, len(cfg.Patterns))
	case "pii_redact":
		cfg, err := decodePIIRedactDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("redact • %s", strings.Join(cfg.Kinds, ","))
	case "length_limit":
		cfg, err := decodeLengthLimitDefinitionConfig(def.Config)
		if err != nil {
			return ""
		}
		parts := []string{}
		if cfg.MaxChars > 0 {
			parts = append(parts, fmt.Sprintf("%d chars", cfg.MaxChars))
		}
		if cfg.MaxEstimatedTokens > 0 {
			parts = append(parts, fmt.Sprintf("%d est. tokens", cfg.MaxEstimatedTokens))
		}
		return strings.Join(parts, " • ")
	default:
		return ""
	}
}

// TypeDefinitions returns the UI-facing definitions for supported guardrail types.
func TypeDefinitions() []TypeDefinition {
	var defs []TypeDefinition

	defs = append(defs, TypeDefinition{
		Type:        "system_prompt",
		Label:       "System Prompt",
		Description: "Injects, overrides, or decorates the system message before the request reaches the provider.",
		Defaults:    mustMarshalRaw(systemPromptDefinitionConfig{Mode: string(SystemPromptInject), Content: "", OnError: OnErrorBlock}),
		Fields: []TypeField{
			{
				Key:      "mode",
				Label:    "Mode",
				Input:    "select",
				Required: true,
				Help:     "Choose whether the prompt is injected only when absent, overrides existing system prompts, or decorates the first one.",
				Options: []TypeOption{
					{Value: string(SystemPromptInject), Label: "Inject"},
					{Value: string(SystemPromptOverride), Label: "Override"},
					{Value: string(SystemPromptDecorator), Label: "Decorator"},
				},
			},
			{
				Key:         "content",
				Label:       "Content",
				Input:       "textarea",
				Required:    true,
				Help:        "The system prompt text applied by this guardrail.",
				Placeholder: "You are a precise assistant. Follow the compliance policy...",
			},
		},
	})

	if !disableAdvancedGuardrails {
		defs = append(defs, TypeDefinition{
			Type:        "llm_based_altering",
			Label:       "LLM-Based Altering",
			Description: "Uses an auxiliary model to rewrite selected message roles before the main request reaches the provider.",
			Defaults: mustMarshalRaw(llmBasedAlteringDefinitionConfig{
				Model:     "",
				Prompt:    DefaultLLMBasedAlteringPrompt,
				Roles:     []string{"user"},
				MaxTokens: DefaultLLMBasedAlteringMaxTokens,
				OnError:   OnErrorBlock,
			}),
			Fields: []TypeField{
				{
					Key:         "model",
					Label:       "Rewrite Model",
					Input:       "text",
					Required:    true,
					Help:        "Model, alias, or {provider}/{model} selector used for the auxiliary rewrite request.",
					Placeholder: "openai/gpt-4o-mini",
				},
				{
					Key:      "roles",
					Label:    "Roles",
					Input:    "checkboxes",
					Required: true,
					Help:     "Choose which conversation roles should be rewritten.",
					Options: []TypeOption{
						{Value: "system", Label: "System"},
						{Value: "user", Label: "User"},
						{Value: "assistant", Label: "Assistant"},
						{Value: "tool", Label: "Tool"},
					},
				},
				{
					Key:         "max_tokens",
					Label:       "Max Tokens",
					Input:       "number",
					Help:        "Upper bound for the auxiliary rewrite completion.",
					Placeholder: fmt.Sprintf("%d", DefaultLLMBasedAlteringMaxTokens),
				},
				{
					Key:         "skip_content_prefix",
					Label:       "Skip Prefix",
					Input:       "text",
					Help:        "If set, messages whose trimmed content starts with this prefix are left unchanged.",
					Placeholder: "### safe",
				},
				{
					Key:         "prompt",
					Label:       "Prompt",
					Input:       "textarea",
					Help:        "Optional custom rewrite prompt. Leave empty to use the built-in LiteLLM-derived anonymization prompt.",
					Placeholder: "Leave empty to use the built-in anonymization prompt.",
				},
			},
		})
	}

	defs = append(defs, regexBlockTypeDefinition(), piiRedactTypeDefinition(), lengthLimitTypeDefinition())
	return cloneTypeDefinitions(defs)
}

func onErrorTypeField(defaultValue string) TypeField {
	return TypeField{
		Key:         "on_error",
		Label:       "On Error",
		Input:       "select",
		Help:        "Choose whether this workflow blocks or allows the request if the guardrail itself errors.",
		Options:     []TypeOption{{Value: OnErrorBlock, Label: "Block"}, {Value: OnErrorAllow, Label: "Allow"}},
		Placeholder: defaultValue,
	}
}

func regexBlockTypeDefinition() TypeDefinition {
	return TypeDefinition{
		Type:        "regex_block",
		Label:       "Regex Block / Sanitize",
		Description: "Matches Go regular expressions against selected message roles and either blocks the request or sanitizes matches.",
		Defaults:    mustMarshalRaw(regexBlockDefinitionConfig{Action: string(RegexBlockActionBlock), Replacement: DefaultRegexReplacement, OnError: OnErrorBlock}),
		Fields: []TypeField{
			{Key: "action", Label: "Action", Input: "select", Required: true, Help: "Block rejects the request. Sanitize replaces every match with the replacement text.", Options: []TypeOption{{Value: string(RegexBlockActionBlock), Label: "Block"}, {Value: string(RegexBlockActionSanitize), Label: "Sanitize"}}},
			{Key: "patterns", Label: "Patterns", Input: "textarea_lines", Required: true, Help: "One Go regular expression per line. Use (?i) for case-insensitive matching.", Placeholder: "(?i)api[_-]?key\\s*[:=]\\n(?i)password\\s*[:=]"},
			{Key: "replacement", Label: "Replacement", Input: "text", Help: "Text used when action is sanitize.", Placeholder: DefaultRegexReplacement},
			onErrorTypeField(OnErrorBlock),
			{Key: "roles", Label: "Roles", Input: "checkboxes", Help: "Leave empty to scan all message roles.", Options: []TypeOption{{Value: "system", Label: "System"}, {Value: "user", Label: "User"}, {Value: "assistant", Label: "Assistant"}, {Value: "tool", Label: "Tool"}}},
		},
	}
}

func piiRedactTypeDefinition() TypeDefinition {
	return TypeDefinition{
		Type:        "pii_redact",
		Label:       "PII Redact",
		Description: "Deterministically redacts email, phone, US SSN, and credit-card-like numbers before provider dispatch.",
		Defaults:    mustMarshalRaw(piiRedactDefinitionConfig{Kinds: []string{PIIKindEmail, PIIKindPhone, PIIKindSSN, PIIKindCreditCard}, OnError: OnErrorAllow}),
		Fields: []TypeField{
			{Key: "kinds", Label: "PII Kinds", Input: "checkboxes", Help: "Leave empty to redact all supported PII kinds.", Options: []TypeOption{{Value: PIIKindEmail, Label: "Email"}, {Value: PIIKindPhone, Label: "Phone"}, {Value: PIIKindSSN, Label: "US SSN"}, {Value: PIIKindCreditCard, Label: "Credit Card"}}},
			onErrorTypeField(OnErrorAllow),
			{Key: "roles", Label: "Roles", Input: "checkboxes", Help: "Leave empty to redact all message roles.", Options: []TypeOption{{Value: "system", Label: "System"}, {Value: "user", Label: "User"}, {Value: "assistant", Label: "Assistant"}, {Value: "tool", Label: "Tool"}}},
		},
	}
}

func lengthLimitTypeDefinition() TypeDefinition {
	return TypeDefinition{
		Type:        "length_limit",
		Label:       "Length Limit",
		Description: "Rejects oversized requests before they reach upstream providers.",
		Defaults:    mustMarshalRaw(lengthLimitDefinitionConfig{MaxChars: 50000, OnError: OnErrorBlock}),
		Fields: []TypeField{
			{Key: "max_chars", Label: "Max Characters", Input: "number", Help: "Maximum combined character count across all message content.", Placeholder: "50000"},
			onErrorTypeField(OnErrorBlock),
			{Key: "max_estimated_tokens", Label: "Max Estimated Tokens", Input: "number", Help: "Approximate token cap using max(words, chars/4). Set either this or max characters.", Placeholder: "12000"},
		},
	}
}

func llmBasedAlteringDescriptor(name string, cfg LLMBasedAlteringConfig, direction string) responsecache.GuardrailRuleDescriptor {
	return responsecache.GuardrailRuleDescriptor{
		Name:      name,
		Type:      "llm_based_altering",
		Direction: normalizeDirection(direction),
		Mode:      strings.Join(cfg.Roles, ","),
		Content: strings.Join([]string{
			cfg.Model,
			cfg.Provider,
			cfg.UserPath,
			cfg.SkipContentPrefix,
			fmt.Sprintf("%d", cfg.MaxTokens),
			cfg.Prompt,
		}, "\x1f"),
	}
}

func regexBlockDescriptor(name string, cfg regexBlockDefinitionConfig, direction string) responsecache.GuardrailRuleDescriptor {
	return responsecache.GuardrailRuleDescriptor{
		Name:      name,
		Type:      "regex_block",
		Direction: normalizeDirection(direction),
		Mode:      cfg.Action + ":" + cfg.OnError,
		Content:   strings.Join(append(append([]string{}, cfg.Patterns...), cfg.Replacement, strings.Join(cfg.Roles, ",")), "\x1f"),
	}
}

func piiRedactDescriptor(name string, cfg piiRedactDefinitionConfig, direction string) responsecache.GuardrailRuleDescriptor {
	return responsecache.GuardrailRuleDescriptor{
		Name:      name,
		Type:      "pii_redact",
		Direction: normalizeDirection(direction),
		Mode:      strings.Join(cfg.Kinds, ",") + ":" + cfg.OnError,
		Content:   strings.Join(cfg.Roles, ","),
	}
}

func lengthLimitDescriptor(name string, cfg lengthLimitDefinitionConfig, direction string) responsecache.GuardrailRuleDescriptor {
	return responsecache.GuardrailRuleDescriptor{
		Name:      name,
		Type:      "length_limit",
		Direction: normalizeDirection(direction),
		Mode:      "block:" + cfg.OnError,
		Content:   fmt.Sprintf("%d:%d", cfg.MaxChars, cfg.MaxEstimatedTokens),
	}
}

type unavailableGuardrail struct {
	name    string
	message string
}

func (g *unavailableGuardrail) Name() string {
	if g == nil {
		return ""
	}
	return g.name
}

func (g *unavailableGuardrail) Process(context.Context, []Message) ([]Message, error) {
	if g == nil {
		return nil, core.NewProviderError("", http.StatusBadGateway, "guardrail is unavailable", nil)
	}
	return nil, core.NewProviderError("", http.StatusBadGateway, g.message, nil)
}

func mustMarshalRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
