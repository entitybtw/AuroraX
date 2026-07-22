package promptcache

import (
	"encoding/json"
	"testing"

	"aurora/internal/core"
)

func TestResolvePromptCache_Defaults(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := ResolvePromptCache(nil, req)
	if pc == nil || !pc.IsEnabled() {
		t.Fatal("expected enabled prompt cache with nil config")
	}
	if pc.Mode != core.PromptCacheAuto {
		t.Fatalf("Mode = %q, want auto", pc.Mode)
	}
}

func TestResolvePromptCache_Off(t *testing.T) {
	cfg := &core.PromptCacheConfig{Mode: core.PromptCacheOff}
	pc := ResolvePromptCache(cfg, &core.ChatRequest{})
	if pc == nil || pc.IsEnabled() {
		t.Fatal("expected disabled prompt cache when mode is off")
	}
	if pc.Mode != core.PromptCacheOff {
		t.Fatalf("Mode = %q, want off", pc.Mode)
	}
}

func TestResolvePromptCache_Auto(t *testing.T) {
	cfg := &core.PromptCacheConfig{
		Mode:               core.PromptCacheAuto,
		SystemPromptCache:  true,
		FirstMessageCache:  true,
		ToolsCache:         false,
		MinTokensBeforeCache: 1024,
	}
	pc := ResolvePromptCache(cfg, &core.ChatRequest{})
	if pc == nil || !pc.IsEnabled() {
		t.Fatal("expected enabled prompt cache")
	}
	if !pc.SystemCacheBreakpoint {
		t.Error("expected SystemCacheBreakpoint = true")
	}
	if !pc.FirstMessageBreakpoint {
		t.Error("expected FirstMessageBreakpoint = true")
	}
	if pc.ToolsCacheBreakpoint {
		t.Error("expected ToolsCacheBreakpoint = false")
	}
}

func TestResolvePromptCache_ManualWithRequestCache(t *testing.T) {
	cfg := &core.PromptCacheConfig{Mode: core.PromptCacheManual}
	req := &core.ChatRequest{}
	req.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
	pc := ResolvePromptCache(cfg, req)
	if pc == nil || !pc.IsEnabled() {
		t.Fatal("expected enabled prompt cache for manual with explicit request")
	}
	if pc.Mode != core.PromptCacheManual {
		t.Fatalf("Mode = %q, want manual", pc.Mode)
	}
	if pc.SystemCacheBreakpoint || pc.FirstMessageBreakpoint {
		t.Error("expected no breakpoints set in ResolvePromptCache (breakpoints applied per-provider)")
	}
}

func TestApplyPromptCache_Anthropic(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "anthropic")

	if len(req.Messages) < 3 {
		t.Fatal("unexpected message count")
	}

	if req.Messages[0].Content == nil {
		t.Fatal("system message content is nil after apply")
	}
	sysParts, ok := req.Messages[0].Content.([]core.ContentPart)
	if !ok {
		t.Fatalf("system message content type = %T, want []ContentPart", req.Messages[0].Content)
	}
	if len(sysParts) == 0 {
		t.Fatal("system message content parts empty")
	}
	sysPartCC := sysParts[0].CacheControl()
	if sysPartCC.IsEmpty() {
		t.Error("expected system content part to have cache_control")
	}
	if sysPartCC.Type != core.CacheControlEphemeral {
		t.Errorf("system content part cc type = %q, want ephemeral", sysPartCC.Type)
	}
	if cc := req.Messages[0].CacheControl(); cc != nil {
		t.Error("expected NO message-level cache_control on system (Anthropic uses ContentPart level)")
	}

	userParts, ok := req.Messages[1].Content.([]core.ContentPart)
	if !ok {
		t.Fatalf("user message content type = %T, want []ContentPart", req.Messages[1].Content)
	}
	if len(userParts) > 0 {
		userPartCC := userParts[0].CacheControl()
		if userPartCC.IsEmpty() {
			t.Error("expected first user content part to have cache_control")
		}
	}
	if cc := req.Messages[1].CacheControl(); cc != nil {
		t.Error("expected NO message-level cache_control on user (Anthropic uses ContentPart level)")
	}

	if asstParts, ok := req.Messages[2].Content.([]core.ContentPart); ok && len(asstParts) > 0 {
		if !asstParts[0].CacheControl().IsEmpty() {
			t.Error("expected assistant content part to NOT have cache_control")
		}
	}
}

func TestApplyPromptCache_Anthropic_AlreadyStructured(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{
				Role: "system",
				Content: []core.ContentPart{
					{Type: "text", Text: "You are helpful."},
					{Type: "text", Text: "Follow the rules."},
				},
			},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "anthropic")

	parts, ok := req.Messages[0].Content.([]core.ContentPart)
	if !ok {
		t.Fatalf("system content type = %T, want []ContentPart", req.Messages[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("system parts count = %d, want 2", len(parts))
	}
	if !parts[0].CacheControl().IsEmpty() {
		t.Error("expected first system part to NOT have cache_control")
	}
	if parts[1].CacheControl().IsEmpty() || parts[1].CacheControl().Type != core.CacheControlEphemeral {
		t.Error("expected last system part to have ephemeral cache_control")
	}

	userParts, ok := req.Messages[1].Content.([]core.ContentPart)
	if !ok {
		t.Fatalf("user content type = %T, want []ContentPart", req.Messages[1].Content)
	}
	if userParts[0].CacheControl().IsEmpty() {
		t.Error("expected first user part to have cache_control")
	}
}

func TestApplyPromptCache_OpenAICompatible(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "openai")

	sysCache := req.Messages[0].CacheControl()
	if sysCache == nil || sysCache.Type != core.CacheControlEphemeral {
		t.Error("expected system message to have cache_control for openai")
	}
	userCache := req.Messages[1].CacheControl()
	if userCache == nil || userCache.Type != core.CacheControlEphemeral {
		t.Error("expected first user message to have cache_control for openai")
	}
}

func TestApplyPromptCache_DeepSeek(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "deepseek")

	if cc := req.Messages[0].CacheControl(); cc != nil {
		t.Error("expected no cache_control on system for deepseek")
	}
	if cc := req.Messages[1].CacheControl(); cc != nil {
		t.Error("expected no cache_control on user for deepseek")
	}
}

func TestApplyPromptCache_MinTokensBeforeCache(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
		Config: core.PromptCacheConfig{
			MinTokensBeforeCache: 10000,
		},
	}
	ApplyPromptCache(req, pc, "anthropic")

	if cc := req.Messages[0].CacheControl(); cc != nil {
		t.Error("expected no cache_control at Message level below min tokens")
	}
	if _, ok := req.Messages[0].Content.([]core.ContentPart); ok {
		t.Error("expected content to stay as string when below min tokens (Anthropic)")
	}
}

func TestApplyPromptCache_MinTokensBeforeCache_OpenAI(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
		Config: core.PromptCacheConfig{
			MinTokensBeforeCache: 10000,
		},
	}
	ApplyPromptCache(req, pc, "openai")

	if cc := req.Messages[0].CacheControl(); cc != nil {
		t.Error("expected no cache_control on openai below min tokens")
	}
}

func TestApplyPromptCache_Groq(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "groq")

	sysCache := req.Messages[0].CacheControl()
	if sysCache != nil {
		t.Error("expected no cache_control on system for groq")
	}
	userCache := req.Messages[1].CacheControl()
	if userCache != nil {
		t.Error("expected no cache_control on user for groq")
	}
}

func TestApplyPromptCache_Gemini(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheAuto,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "gemini")

	sysCache := req.Messages[0].CacheControl()
	if sysCache == nil || sysCache.Type != core.CacheControlEphemeral {
		t.Error("expected system message to have cache_control for gemini")
	}
	userCache := req.Messages[1].CacheControl()
	if userCache == nil || userCache.Type != core.CacheControlEphemeral {
		t.Error("expected first user message to have cache_control for gemini")
	}
}

func TestApplyPromptCache_ManualModeSkipsAutoInjection(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	pc := &core.PromptCache{
		Mode:                   core.PromptCacheManual,
		SystemCacheBreakpoint:  true,
		FirstMessageBreakpoint: true,
	}
	ApplyPromptCache(req, pc, "anthropic")

	if cc := req.Messages[0].CacheControl(); cc != nil {
		t.Error("expected no auto-injected message-level cache_control in manual mode")
	}
	if parts, ok := req.Messages[0].Content.([]core.ContentPart); ok && len(parts) > 0 {
		if !parts[0].CacheControl().IsEmpty() {
			t.Error("expected no auto-injected content-part cache_control in manual mode")
		}
	}
}

func TestExtractCacheUsageInfo(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want CacheUsageInfo
	}{
		{
			name: "anthropic format",
			data: map[string]any{
				"cache_read_input_tokens":    100,
				"cache_creation_input_tokens": 50,
			},
			want: CacheUsageInfo{
				CacheReadTokens:     100,
				CacheWriteTokens:    50,
				CacheCreationTokens: 50,
			},
		},
		{
			name: "openai-compat format",
			data: map[string]any{
				"cached_tokens": 200,
			},
			want: CacheUsageInfo{
				CachedTokens: 200,
			},
		},
		{
			name: "deepseek format",
			data: map[string]any{
				"prompt_cache_hit_tokens":  42,
				"prompt_cache_miss_tokens": 10,
			},
			want: CacheUsageInfo{
				CachedTokens: 42,
			},
		},
		{
			name: "openai nested cached_tokens",
			data: map[string]any{
				"prompt_tokens_details": map[string]any{
					"cached_tokens": 77,
				},
			},
			want: CacheUsageInfo{
				CachedTokens: 77,
			},
		},
		{
			name: "deepseek with nested prompt_tokens_details (both present, top-level wins)",
			data: map[string]any{
				"prompt_cache_hit_tokens":  42,
				"prompt_cache_miss_tokens": 10,
				"prompt_tokens_details": map[string]any{
					"cached_tokens": 77,
				},
			},
			want: CacheUsageInfo{
				CachedTokens: 42,
			},
		},
		{
			name: "empty",
			data: map[string]any{},
			want: CacheUsageInfo{},
		},
		{
			name: "nil",
			data: nil,
			want: CacheUsageInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCacheUsageInfo(tt.data)
			if got != tt.want {
				t.Errorf("ExtractCacheUsageInfo(%s) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCacheControlJSONRoundTrip(t *testing.T) {
	cc := core.CacheControl{Type: core.CacheControlEphemeral}
	data, err := json.Marshal(cc)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded core.CacheControl
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if decoded.Type != core.CacheControlEphemeral {
		t.Errorf("Type = %q, want %q", decoded.Type, core.CacheControlEphemeral)
	}
}

func TestCacheControlExtraFieldsRoundTrip(t *testing.T) {
	extra := core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{
		"custom_field": json.RawMessage(`"custom_value"`),
	})
	cc := core.CacheControl{
		Type:        core.CacheControlEphemeral,
		ExtraFields: extra,
	}
	data, err := json.Marshal(cc)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if string(result["type"]) != `"ephemeral"` {
		t.Errorf("type = %s, want \"ephemeral\"", string(result["type"]))
	}
	if string(result["custom_field"]) != `"custom_value"` {
		t.Errorf("custom_field = %s, want \"custom_value\"", string(result["custom_field"]))
	}
}

func TestContentPartCacheControl(t *testing.T) {
	part := core.ContentPart{Type: "text", Text: "hello"}
	if cc := part.CacheControl(); !cc.IsEmpty() {
		t.Error("expected empty cache control initially")
	}

	part.SetCacheControl(core.CacheControl{Type: core.CacheControlEphemeral})
	cc := part.CacheControl()
	if cc.IsEmpty() {
		t.Fatal("expected non-empty cache control after SetCacheControl")
	}
	if cc.Type != core.CacheControlEphemeral {
		t.Errorf("Type = %q, want %q", cc.Type, core.CacheControlEphemeral)
	}
}

func TestMessageCacheControl(t *testing.T) {
	msg := core.Message{Role: "user", Content: "hello"}
	if cc := msg.CacheControl(); cc != nil {
		t.Error("expected nil cache control initially")
	}

	msg.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
	cc := msg.CacheControl()
	if cc == nil {
		t.Fatal("expected non-nil cache control after SetCacheControl")
	}
	if cc.Type != core.CacheControlEphemeral {
		t.Errorf("Type = %q, want %q", cc.Type, core.CacheControlEphemeral)
	}
}

func TestChatRequestCacheControl(t *testing.T) {
	req := &core.ChatRequest{Model: "test-model"}
	if cc := req.CacheControl(); cc != nil {
		t.Error("expected nil cache control initially")
	}

	req.SetCacheControl(&core.CacheControl{Type: core.CacheControlEphemeral})
	cc := req.CacheControl()
	if cc == nil {
		t.Fatal("expected non-nil cache control after SetCacheControl")
	}
	if cc.Type != core.CacheControlEphemeral {
		t.Errorf("Type = %q, want %q", cc.Type, core.CacheControlEphemeral)
	}
}

func TestPromptCacheApplyDefaults(t *testing.T) {
	cfg := core.PromptCacheConfig{}
	cfg.ApplyDefaults()
	if cfg.Mode != core.PromptCacheAuto {
		t.Errorf("Mode = %q, want auto", cfg.Mode)
	}
	if cfg.MinTokensBeforeCache != 1024 {
		t.Errorf("MinTokensBeforeCache = %d, want 1024", cfg.MinTokensBeforeCache)
	}
}

func TestDefaultPromptCacheConfig(t *testing.T) {
	d := core.DefaultPromptCacheConfig()
	if d.Mode != core.PromptCacheAuto {
		t.Errorf("Mode = %q, want auto", d.Mode)
	}
	pc := &core.PromptCache{Mode: d.Mode, Config: d}
	if !pc.IsEnabled() {
		t.Error("expected default config to be enabled")
	}
}

func TestCacheControlIsEmpty(t *testing.T) {
	var empty core.CacheControl
	if !empty.IsEmpty() {
		t.Error("expected empty CacheControl to be empty")
	}
	nonEmpty := core.CacheControl{Type: core.CacheControlEphemeral}
	if nonEmpty.IsEmpty() {
		t.Error("expected non-empty CacheControl to not be empty")
	}
}

func TestCacheControlRawJSON(t *testing.T) {
	cc := core.CacheControl{Type: core.CacheControlEphemeral}
	raw := cc.RawJSON()
	if raw == nil {
		t.Fatal("RawJSON() returned nil")
	}
	var decoded core.CacheControl
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if decoded.Type != core.CacheControlEphemeral {
		t.Errorf("Type = %q, want %q", decoded.Type, core.CacheControlEphemeral)
	}
}
