package tokensaver

import (
	"encoding/json"
	"strings"
	"testing"

	"aurora/configuration"
	"aurora/internal/core"
)

func TestApplyChatDisabledReturnsEquivalentCopy(t *testing.T) {
	req := &core.ChatRequest{Model: "gpt-4o", Messages: []core.Message{{Role: "user", Content: "hello"}}}
	service := NewService(config.TokenSaverConfig{})

	got, meta, err := service.ApplyChat(req, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("ApplyChat() error = %v", err)
	}
	if got == req {
		t.Fatal("ApplyChat returned original pointer")
	}
	if meta.Applied {
		t.Fatal("expected disabled token saver to skip")
	}
	if got.Messages[0].Content != "hello" {
		t.Fatalf("content = %v, want hello", got.Messages[0].Content)
	}
}

func TestApplyChatInjectsCavemanInstruction(t *testing.T) {
	cfg := config.TokenSaverConfig{
		Enabled: true,
		Output:  config.TokenSaverOutputConfig{Enabled: true, Profile: config.TokenSaverOutputProfileConcise},
		OnError: config.TokenSaverOnErrorAllow,
	}
	req := &core.ChatRequest{Model: "gpt-4o", Messages: []core.Message{{Role: "user", Content: "be brief"}}}
	service := NewService(cfg)

	got, meta, err := service.ApplyChat(req, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("ApplyChat() error = %v", err)
	}
	if !meta.Applied {
		t.Fatalf("meta.Applied = false, skip=%s", meta.SkipReason)
	}
	if !meta.OutputProfileApplied {
		t.Fatal("expected output profile to be applied")
	}
	found := false
	for _, msg := range got.Messages {
		if msg.Role == "system" && strings.Contains(msg.Content.(string), config.InstructionFull) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("caveman instruction not found in messages")
	}
}

func TestApplyChatConciseProfileSkipsJSONMode(t *testing.T) {
	cfg := config.TokenSaverConfig{
		Enabled: true,
		Output:  config.TokenSaverOutputConfig{Enabled: true, Profile: config.TokenSaverOutputProfileConcise},
		OnError: config.TokenSaverOnErrorAllow,
	}
	req := &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: "user", Content: "return json"}},
		ExtraFields: core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{
			"response_format": json.RawMessage(`{"type":"json_object"}`),
		}),
	}
	service := NewService(cfg)

	got, meta, err := service.ApplyChat(req, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("ApplyChat() error = %v", err)
	}
	if meta.OutputProfileApplied {
		t.Fatal("concise profile should not be applied in JSON mode")
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(got.Messages))
	}
}

func TestApplyChatCavemanInstructionIdempotent(t *testing.T) {
	cfg := config.TokenSaverConfig{
		Enabled: true,
		Output:  config.TokenSaverOutputConfig{Enabled: true, Profile: config.TokenSaverOutputProfileConcise},
		OnError: config.TokenSaverOnErrorAllow,
	}
	service := NewService(cfg)
	req := &core.ChatRequest{Model: "gpt-4o", Messages: []core.Message{{Role: "user", Content: "be brief"}}}

	first, _, err := service.ApplyChat(req, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("first ApplyChat() error = %v", err)
	}
	second, _, err := service.ApplyChat(first, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("second ApplyChat() error = %v", err)
	}
	count := 0
	for _, msg := range second.Messages {
		if msg.Role == "system" && msg.Content == config.InstructionFull {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("caveman instruction count = %d, want 1", count)
	}
}

func TestApplyChatPolicySkipsStreamingWhenDisabled(t *testing.T) {
	cfg := config.TokenSaverConfig{
		Enabled:        true,
		ApplyStreaming: false,
		OnError:        config.TokenSaverOnErrorAllow,
		Output:         config.TokenSaverOutputConfig{Enabled: true, Profile: config.TokenSaverOutputProfileConcise},
	}
	service := NewService(cfg)
	req := &core.ChatRequest{Model: "gpt-4o", Stream: true, Messages: []core.Message{{Role: "user", Content: "hello"}}}

	_, meta, err := service.ApplyChat(req, ChatMeta{Endpoint: EndpointChatCompletions})
	if err != nil {
		t.Fatalf("ApplyChat() error = %v", err)
	}
	if meta.Applied || meta.SkipReason != SkipStreaming {
		t.Fatalf("meta = %+v, want streaming skip", meta)
	}
}
