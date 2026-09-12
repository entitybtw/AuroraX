package providers

import (
	"testing"

	"aurora/configuration"
	"aurora/internal/core"
)

func floatPtr(v float64) *float64 { return &v }

func modelWithPrice(id string, input, output *float64) core.Model {
	m := core.Model{ID: id}
	if input != nil || output != nil {
		m.Metadata = &core.ModelMetadata{
			Pricing: &core.ModelPricing{
				InputPerMtok:  input,
				OutputPerMtok: output,
			},
		}
	}
	return m
}

func modelsResponse(ids ...string) *core.ModelsResponse {
	data := make([]core.Model, 0, len(ids))
	for _, id := range ids {
		data = append(data, core.Model{ID: id})
	}
	return &core.ModelsResponse{Object: "list", Data: data}
}

func TestCompileAutoFetchFilter_EmptyIsNoOp(t *testing.T) {
	compiled, err := compileAutoFetchFilter("openrouter", config.AutoFetchFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if compiled != nil {
		t.Fatalf("expected nil filter for empty config, got %+v", compiled)
	}
}

func TestCompileAutoFetchFilter_InvalidMode(t *testing.T) {
	_, err := compileAutoFetchFilter("openrouter", config.AutoFetchFilter{
		Mode:       "neither",
		Conditions: []config.AutoFetchFilterCondition{{Contains: "free"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestCompileAutoFetchFilter_InvalidRegex(t *testing.T) {
	_, err := compileAutoFetchFilter("openrouter", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{Regex: "([unclosed"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestCompileAutoFetchFilter_EmptyConditionRejected(t *testing.T) {
	_, err := compileAutoFetchFilter("openrouter", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{}},
	})
	if err == nil {
		t.Fatal("expected error for condition with no content")
	}
}

func TestAutoFetchFilter_ContainsFree(t *testing.T) {
	compiled, err := compileAutoFetchFilter("openrouter", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{Contains: "free"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := modelsResponse("deepseek/deepseek-r1:free", "openai/gpt-4o", "google/gemini:free")
	filtered, removed := applyAutoFetchFilter("openrouter", compiled, resp)

	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if len(filtered.Data) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(filtered.Data))
	}
	for _, model := range filtered.Data {
		if model.ID == "openai/gpt-4o" {
			t.Fatalf("non-free model survived the filter")
		}
	}
	if len(resp.Data) != 3 {
		t.Fatalf("original response was mutated: %d", len(resp.Data))
	}
}

func TestAutoFetchFilter_ContainsIsCaseInsensitive(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{Contains: "FREE"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, _ := applyAutoFetchFilter("p", compiled, modelsResponse("model:Free", "model:paid"))
	if len(filtered.Data) != 1 || filtered.Data[0].ID != "model:Free" {
		t.Fatalf("case-insensitive match failed: %+v", filtered.Data)
	}
}

func TestAutoFetchFilter_ModeAny(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Mode: "any",
		Conditions: []config.AutoFetchFilterCondition{
			{Contains: "free"},
			{Contains: "flash"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, _ := applyAutoFetchFilter("p", compiled, modelsResponse("a:free", "b-flash", "c-pro"))
	if len(filtered.Data) != 2 {
		t.Fatalf("mode=any expected 2 kept, got %d: %+v", len(filtered.Data), filtered.Data)
	}
}

func TestAutoFetchFilter_ModeAllRequiresEveryCondition(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{
			{Contains: "free"},
			{NotContains: "preview"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, _ := applyAutoFetchFilter("p", compiled, modelsResponse("a:free", "a:free-preview"))
	if len(filtered.Data) != 1 || filtered.Data[0].ID != "a:free" {
		t.Fatalf("mode=all expected only a:free, got %+v", filtered.Data)
	}
}

func TestAutoFetchFilter_MaxPriceZeroRequiresFree(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{MaxPrice: floatPtr(0)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &core.ModelsResponse{Object: "list", Data: []core.Model{
		modelWithPrice("free-model", floatPtr(0), floatPtr(0)),
		modelWithPrice("paid-model", floatPtr(0.5), floatPtr(1.5)),
		{ID: "no-metadata"},
		modelWithPrice("half-priced", floatPtr(0), floatPtr(2)),
	}}

	filtered, removed := applyAutoFetchFilter("p", compiled, resp)
	if removed != 3 {
		t.Fatalf("expected 3 removed, got %d", removed)
	}
	if len(filtered.Data) != 1 || filtered.Data[0].ID != "free-model" {
		t.Fatalf("expected only free-model, got %+v", filtered.Data)
	}
}

func TestAutoFetchFilter_MaxPriceZeroRejectsMissingPricing(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{MaxPrice: floatPtr(0)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, removed := applyAutoFetchFilter("p", compiled, modelsResponse("unknown-model"))
	if removed != 1 || len(filtered.Data) != 0 {
		t.Fatalf("model without pricing must not be treated as free; removed=%d kept=%d", removed, len(filtered.Data))
	}
}

func TestAutoFetchFilter_MaxPromptAndCompletionPrice(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{MaxPromptPrice: floatPtr(0)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output price is irrelevant for a prompt-only ceiling.
	resp := &core.ModelsResponse{Object: "list", Data: []core.Model{
		modelWithPrice("cheap-in", floatPtr(0), floatPtr(99)),
		modelWithPrice("pricey-in", floatPtr(1), floatPtr(0)),
	}}
	filtered, _ := applyAutoFetchFilter("p", compiled, resp)
	if len(filtered.Data) != 1 || filtered.Data[0].ID != "cheap-in" {
		t.Fatalf("unexpected result: %+v", filtered.Data)
	}
}

func TestAutoFetchFilter_Regex(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{Regex: `^[a-z]+/[a-z0-9.-]+:free$`}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, _ := applyAutoFetchFilter("p", compiled, modelsResponse("meta/llama-3:free", "META/Llama:free", "meta/llama-3"))
	if len(filtered.Data) != 1 || filtered.Data[0].ID != "meta/llama-3:free" {
		t.Fatalf("unexpected regex result: %+v", filtered.Data)
	}
}

func TestAutoFetchFilter_NilFilterAndNilResponseAreSafe(t *testing.T) {
	if got, removed := applyAutoFetchFilter("p", nil, modelsResponse("a")); removed != 0 || len(got.Data) != 1 {
		t.Fatal("nil filter must pass everything through")
	}
	if got, removed := applyAutoFetchFilter("p", &compiledAutoFetchFilter{mode: autofetchFilterModeAll}, nil); got != nil || removed != 0 {
		t.Fatal("nil response must be handled")
	}
}

func TestAutoFetchFilter_NoMatchesReturnsEmpty(t *testing.T) {
	compiled, err := compileAutoFetchFilter("p", config.AutoFetchFilter{
		Conditions: []config.AutoFetchFilterCondition{{Contains: "nonexistent"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filtered, removed := applyAutoFetchFilter("p", compiled, modelsResponse("a", "b"))
	if removed != 2 || len(filtered.Data) != 0 {
		t.Fatalf("expected everything filtered out, removed=%d kept=%d", removed, len(filtered.Data))
	}
}
