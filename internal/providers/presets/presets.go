// Package presets provides built-in provider configuration presets for common
// AI providers. Presets allow one-click setup with sensible defaults while
// allowing the user to confirm or override before applying.
package presets

import (
	"strings"
)

// Preset defines a pre-configured provider setup.
type Preset struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	BaseURL     string `json:"base_url"`
	AuthMethod  string `json:"auth_method,omitempty"`
	KeyOptional bool   `json:"key_optional,omitempty"`
	Description string `json:"description"`
	Models      string `json:"models,omitempty"`
}

// DetectPreset checks if the given config matches a known preset.
// Returns the matching preset and true if the base URL or provider type matches.
func DetectPreset(name, providerType, baseURL string) (*Preset, bool) {
	cleanURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	cleanType := strings.ToLower(strings.TrimSpace(providerType))

	for i := range builtInPresets {
		p := &builtInPresets[i]
		if cleanType != "" && cleanType == p.Type {
			if cleanURL == "" || cleanURL == p.BaseURL {
				return p, true
			}
		}
		if cleanURL != "" && cleanURL == p.BaseURL {
			return p, true
		}
	}
	return nil, false
}

// builtInPresets contains the full list of provider presets.
var builtInPresets = []Preset{
	{
		Name:        "free tier (Free Tier)",
		Type:        "opencode",
		BaseURL:     "https://opencode.ai/zen/v1",
		AuthMethod:  "oauth",
		KeyOptional: true,
		Description: "free tier free-tier models with OAuth device flow authentication. No API key required — link your upstream account via browser.",
	},
	{
		Name:        "OpenAI",
		Type:        "openai",
		BaseURL:     "https://api.openai.com/v1",
		Description: "OpenAI GPT models (gpt-4o, gpt-4-turbo, etc.)",
	},
	{
		Name:        "Anthropic",
		Type:        "anthropic",
		BaseURL:     "https://api.anthropic.com/v1",
		Description: "Anthropic Claude models (claude-3.5-sonnet, claude-3-opus, etc.)",
	},
	{
		Name:        "Google Gemini",
		Type:        "gemini",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
		Description: "Google Gemini models via OpenAI-compatible endpoint",
	},
	{
		Name:        "DeepSeek",
		Type:        "deepseek",
		BaseURL:     "https://api.deepseek.com",
		Description: "DeepSeek V3/R1 models",
	},
	{
		Name:        "Groq",
		Type:        "groq",
		BaseURL:     "https://api.groq.com/openai/v1",
		Description: "Groq hosted models (Llama 3, Mixtral, Gemma, etc.)",
	},
	{
		Name:        "OpenRouter",
		Type:        "openrouter",
		BaseURL:     "https://openrouter.ai/api/v1",
		Description: "OpenRouter multi-provider gateway (100+ models)",
	},
	{
		Name:        "xAI (Grok)",
		Type:        "xai",
		BaseURL:     "https://api.x.ai/v1",
		Description: "xAI Grok models",
	},
	{
		Name:        "Ollama (Local)",
		Type:        "ollama",
		BaseURL:     "http://localhost:11434/v1",
		KeyOptional: true,
		Description: "Local Ollama server — no API key needed",
	},
	{
		Name:        "vLLM (Local)",
		Type:        "vllm",
		BaseURL:     "http://localhost:8000/v1",
		KeyOptional: true,
		Description: "Local vLLM server — no API key needed",
	},
	{
		Name:        "Azure OpenAI",
		Type:        "azure",
		BaseURL:     "",
		Description: "Azure OpenAI Service (requires base URL with your resource name)",
	},
	{
		Name:        "MiniMax",
		Type:        "minimax",
		BaseURL:     "https://api.minimax.io/v1",
		Description: "MiniMax models (abab series)",
	},
	{
		Name:        "Z.ai",
		Type:        "zai",
		BaseURL:     "https://api.z.ai/api/paas/v4",
		Description: "Z.ai API",
	},
}

// ListPresets returns all built-in presets.
func ListPresets() []Preset {
	out := make([]Preset, len(builtInPresets))
	copy(out, builtInPresets)
	return out
}

// MatchesPreset returns true if the given config matches a preset.
func MatchesPreset(name, providerType, baseURL string) bool {
	_, ok := DetectPreset(name, providerType, baseURL)
	return ok
}
