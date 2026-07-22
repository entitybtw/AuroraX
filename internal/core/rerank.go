package core

import (
	"context"
	json "github.com/goccy/go-json"
)

// RerankRequest represents the incoming rerank request.
//
// The shape mirrors the de-facto OpenAI-compatible rerank contract used by
// Jina (https://api.jina.ai/v1/rerank), Cohere, and vLLM's /rerank endpoint.
type RerankRequest struct {
	Model           string            `json:"model"`
	Provider        string            `json:"provider,omitempty"` // Gateway routing hint; stripped before upstream execution.
	Query           string            `json:"query"`
	Documents       []string          `json:"documents"`
	TopN            *int              `json:"top_n,omitempty"`
	ReturnDocuments *bool             `json:"return_documents,omitempty"`
	ExtraFields     UnknownJSONFields `json:"-" swaggerignore:"true"`
}

func (r *RerankRequest) semanticSelector() (string, string) {
	if r == nil {
		return "", ""
	}
	return r.Model, r.Provider
}

// RerankResponse represents a rerank response.
type RerankResponse struct {
	Object   string         `json:"object"`
	Model    string         `json:"model"`
	Provider string         `json:"provider"`
	Results  []RerankResult `json:"results"`
	Usage    RerankUsage    `json:"usage"`
}

// RerankResult is a single ranked document entry.
type RerankResult struct {
	Index          int             `json:"index"`
	RelevanceScore float64         `json:"relevance_score"`
	Document       *RerankDocument `json:"document,omitempty"`
}

// RerankDocument is the optional echoed document text returned when
// return_documents=true is set on the request.
type RerankDocument struct {
	Text string `json:"text,omitempty"`
}

// UnmarshalJSON handles the fact that some upstreams (e.g. Jina AI) return
// the document field as a bare string ("doc text") instead of an object
// with a "text" key ({"text": "doc text"}).
func (d *RerankDocument) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &d.Text)
	}
	type alias RerankDocument
	var doc alias
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	d.Text = doc.Text
	return nil
}

// RerankUsage captures token usage for a rerank call.
type RerankUsage struct {
	PromptTokens int `json:"prompt_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens"`
}

// RerankProvider is implemented by providers that support /v1/rerank.
//
// This is intentionally a separate capability interface (mirroring
// NativeBatchProvider, NativeFileProvider, etc.) so that providers without
// rerank support stay simple and the gateway can return a clean 501 via
// type assertion at the route handler.
type RerankProvider interface {
	Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error)
}

// RerankRoutableProvider extends Router with rerank dispatch by model selector.
type RerankRoutableProvider interface {
	Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error)
}
