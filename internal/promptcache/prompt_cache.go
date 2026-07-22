package promptcache

import (
	"encoding/json"
	"log/slog"

	"aurora/internal/core"
)

func ApplyPromptCache(req *core.ChatRequest, pc *core.PromptCache, providerType string) {
	if req == nil || !pc.IsEnabled() {
		return
	}

	switch providerType {
	case "anthropic":
		applyAnthropicPromptCache(req, pc)
	case "groq":
		return
	case "deepseek":
		return
	case "openai", "azure", "openrouter", "xai", "zai", "minimax", "oracle", "vllm":
		applyOpenAICompatiblePromptCache(req, pc)
	case "gemini":
		applyGeminiPromptCache(req, pc)
	default:
		applyOpenAICompatiblePromptCache(req, pc)
	}
}

func applyAnthropicPromptCache(req *core.ChatRequest, pc *core.PromptCache) {
	if pc.Mode == core.PromptCacheManual {
		return
	}

	if !meetsMinTokens(req, pc) {
		return
	}

	if pc.SystemCacheBreakpoint {
		for i, msg := range req.Messages {
			if msg.Role != "system" {
				break
			}
			ensureStructuredContent(&req.Messages[i])
			applyCacheControlToLastPart(&req.Messages[i])
		}
	}

	if pc.FirstMessageBreakpoint {
		for i, msg := range req.Messages {
			if msg.Role == "user" {
				ensureStructuredContent(&req.Messages[i])
				applyCacheControlToLastPart(&req.Messages[i])
				break
			}
		}
	}

	if pc.ToolsCacheBreakpoint && len(req.Tools) > 0 {
		req.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
	}
}

func ensureStructuredContent(msg *core.Message) {
	if msg == nil || core.HasStructuredContent(msg.Content) {
		return
	}
	text := core.ExtractTextContent(msg.Content)
	if text == "" {
		return
	}
	msg.Content = []core.ContentPart{{Type: "text", Text: text}}
}

func applyCacheControlToLastPart(msg *core.Message) {
	if msg == nil {
		return
	}
	parts, ok := core.NormalizeContentParts(msg.Content)
	if !ok || len(parts) == 0 {
		return
	}
	last := len(parts) - 1
	cc := core.CacheControl{Type: core.CacheControlEphemeral}
	parts[last].SetCacheControl(cc)
	msg.Content = parts
}

func meetsMinTokens(req *core.ChatRequest, pc *core.PromptCache) bool {
	minTokens := pc.Config.MinTokensBeforeCache
	if minTokens <= 0 {
		return true
	}
	var totalChars int
	for _, msg := range req.Messages {
		totalChars += len(core.ExtractTextContent(msg.Content))
	}
	estimatedTokens := totalChars / 4
	if estimatedTokens < minTokens {
		slog.Debug("content below MinTokensBeforeCache threshold, skipping cache breakpoints",
			"estimated_tokens", estimatedTokens,
			"min_tokens", minTokens,
		)
		return false
	}
	return true
}

func applyCacheControlToMessage(msg *core.Message) {
	if msg == nil {
		return
	}
	msg.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
}

func applyOpenAICompatiblePromptCache(req *core.ChatRequest, pc *core.PromptCache) {
	if pc.Mode == core.PromptCacheManual {
		return
	}

	if !meetsMinTokens(req, pc) {
		return
	}

	if pc.SystemCacheBreakpoint || pc.FirstMessageBreakpoint {
		for i := range req.Messages {
			if pc.SystemCacheBreakpoint && req.Messages[i].Role == "system" {
				req.Messages[i].SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
				break
			}
		}
		for i := range req.Messages {
			if pc.FirstMessageBreakpoint && req.Messages[i].Role == "user" {
				req.Messages[i].SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
				break
			}
		}
	}

	if pc.ToolsCacheBreakpoint && len(req.Tools) > 0 {
		req.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
	}
}

func applyGeminiPromptCache(req *core.ChatRequest, pc *core.PromptCache) {
	applyOpenAICompatiblePromptCache(req, pc)

	if pc.IsEnabled() {
		slog.Info("gemini explicit context caching requires the cachedContent API (separate cache creation + cached_content request parameter); cache_control markers on messages are ignored")
	}
}

func ResolvePromptCache(config *core.PromptCacheConfig, req *core.ChatRequest) *core.PromptCache {
	if config == nil {
		cfg := core.DefaultPromptCacheConfig()
		config = &cfg
	}

	reqCache := req.CacheControl()
	if reqCache != nil && config.Mode == core.PromptCacheManual {
		return &core.PromptCache{
			Mode: core.PromptCacheManual,
		}
	}

	pc := &core.PromptCache{
		Mode:   config.Mode,
		Config: *config,
	}

	switch config.Mode {
	case core.PromptCacheAuto:
		pc.SystemCacheBreakpoint = config.SystemPromptCache
		pc.FirstMessageBreakpoint = config.FirstMessageCache
		pc.ToolsCacheBreakpoint = config.ToolsCache
	case core.PromptCacheManual:
		if reqCache != nil && reqCache.Type == core.CacheControlEphemeral {
			pc.SystemCacheBreakpoint = true
			pc.FirstMessageBreakpoint = true
		}
	case core.PromptCacheOff:
		return pc
	}

	return pc
}

type CacheUsageInfo struct {
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens    int `json:"cache_write_tokens,omitempty"`
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
	CachedTokens        int `json:"cached_tokens,omitempty"`
}

func ExtractCacheUsageInfo(rawData map[string]any) CacheUsageInfo {
	if len(rawData) == 0 {
		return CacheUsageInfo{}
	}

	hitTokens := extractInt(rawData, "prompt_cache_hit_tokens")
	_ = extractInt(rawData, "prompt_cache_miss_tokens")

	readTokens := extractInt(rawData, "cache_read_input_tokens")
	writeTokens := extractInt(rawData, "cache_creation_input_tokens")

	cachedTokens := extractInt(rawData, "cached_tokens")
	if cachedTokens == 0 {
		cachedTokens = hitTokens
	}
	if cachedTokens == 0 {
		if details, ok := rawData["prompt_tokens_details"].(map[string]any); ok {
			cachedTokens = extractInt(details, "cached_tokens")
		}
	}

	return CacheUsageInfo{
		CacheReadTokens:     readTokens,
		CacheWriteTokens:    writeTokens,
		CacheCreationTokens: writeTokens,
		CachedTokens:        cachedTokens,
	}
}

func extractInt(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case json.Number:
		val, _ := n.Int64()
		return int(val)
	default:
		return 0
	}
}
