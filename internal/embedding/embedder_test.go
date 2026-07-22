package embedding

import (
	"context"
	"testing"

	"aurora/configuration"
)

func TestNewEmbedder_EmptyProvider(t *testing.T) {
	_, err := NewEmbedder(config.EmbedderConfig{}, map[string]config.RawProviderConfig{})
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestNewEmbedder_LocalRejected(t *testing.T) {
	_, err := NewEmbedder(config.EmbedderConfig{Provider: "local"}, map[string]config.RawProviderConfig{"local": {}})
	if err == nil {
		t.Fatal("expected error for local provider")
	}
}

func TestNewEmbedder_UnknownProvider(t *testing.T) {
	_, err := NewEmbedder(config.EmbedderConfig{Provider: "nonexistent-provider"}, map[string]config.RawProviderConfig{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewEmbedder_ReturnsRemoteEmbedder(t *testing.T) {
	rawProviders := map[string]config.RawProviderConfig{
		"openai": {
			Type:    "openai",
			APIKey:  "sk-test",
			BaseURL: "https://api.openai.com",
		},
	}
	emb, err := NewEmbedder(config.EmbedderConfig{
		Provider: "openai",
		Model:    "text-embedding-3-small",
	}, rawProviders)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() { _ = emb.Close() }()
	r, ok := emb.(*remoteEmbedder)
	if !ok {
		t.Fatalf("expected *remoteEmbedder, got %T", emb)
	}
	if r.endpoint != "https://api.openai.com/v1/embeddings" {
		t.Fatalf("endpoint = %q", r.endpoint)
	}
}

func TestNewEmbedder_GeminiUsesProviderBaseURL(t *testing.T) {
	const geminiURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	rawProviders := map[string]config.RawProviderConfig{
		"gemini": {
			Type:    "gemini",
			APIKey:  "AIza-test",
			BaseURL: geminiURL,
		},
	}
	emb, err := NewEmbedder(config.EmbedderConfig{
		Provider: "gemini",
		Model:    "text-embedding-004",
	}, rawProviders)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = emb.Close() }()
	r, ok := emb.(*remoteEmbedder)
	if !ok {
		t.Fatalf("expected *remoteEmbedder, got %T", emb)
	}
	wantURL := geminiURL + "/v1/embeddings"
	if r.endpoint != wantURL {
		t.Fatalf("endpoint = %q, want %q", r.endpoint, wantURL)
	}
	if r.model != "gemini-embedding-001" {
		t.Fatalf("model = %q, want gemini-embedding-001 (text-embedding-* is not valid on Gemini OpenAI compat)", r.model)
	}
}

func TestNewEmbedder_GeminiEmptyModelDefault(t *testing.T) {
	rawProviders := map[string]config.RawProviderConfig{
		"gemini": {
			Type:    "gemini",
			APIKey:  "k",
			BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		},
	}
	emb, err := NewEmbedder(config.EmbedderConfig{Provider: "gemini", Model: ""}, rawProviders)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = emb.Close() }()
	r := emb.(*remoteEmbedder)
	if r.model != "gemini-embedding-001" {
		t.Fatalf("model = %q", r.model)
	}
}

func TestBuildEmbeddingURL_TrimAndJoin(t *testing.T) {
	got, err := buildEmbeddingURL("https://example.com/custom/")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.com/custom/v1/embeddings"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	got2, err := buildEmbeddingURL("https://api.openai.com/v1")
	if err != nil {
		t.Fatal(err)
	}
	if want2 := "https://api.openai.com/v1/embeddings"; got2 != want2 {
		t.Fatalf("got %q, want %q", got2, want2)
	}
}

func TestRemoteEmbedder_UsesProviderCredentials(t *testing.T) {
	rawProviders := map[string]config.RawProviderConfig{
		"groq": {
			Type:    "groq",
			APIKey:  "gsk-abc",
			BaseURL: "https://api.groq.com/openai",
		},
	}
	emb, err := NewEmbedder(config.EmbedderConfig{
		Provider: "groq",
		Model:    "nomic-embed-text-v1_5",
	}, rawProviders)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r, ok := emb.(*remoteEmbedder)
	if !ok {
		t.Fatalf("expected *remoteEmbedder, got %T", emb)
	}
	if r.apiKey != "gsk-abc" {
		t.Errorf("expected apiKey gsk-abc, got %q", r.apiKey)
	}
	if want := "https://api.groq.com/openai/v1/embeddings"; r.endpoint != want {
		t.Errorf("endpoint = %q, want %q", r.endpoint, want)
	}
}

type MockEmbedder struct {
	Vector []float32
	Err    error
	Calls  int
}

func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	m.Calls++
	return m.Vector, m.Err
}

func (m *MockEmbedder) Identity() string { return "mock" }

func (m *MockEmbedder) Close() error { return nil }
