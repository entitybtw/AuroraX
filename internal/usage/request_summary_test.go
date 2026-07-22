package usage

import "testing"

func TestSummarizeRequestUsage_OpenAICompatibleCachedTokens(t *testing.T) {
	summary := SummarizeRequestUsage([]UsageLogEntry{
		{
			Provider:     "openai",
			InputTokens:  120,
			OutputTokens: 30,
			RawData: map[string]any{
				"prompt_cached_tokens": 80,
			},
		},
	})
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.InputTokens != 120 {
		t.Fatalf("InputTokens = %d, want 120", summary.InputTokens)
	}
	if summary.UncachedInputTokens != 40 {
		t.Fatalf("UncachedInputTokens = %d, want 40", summary.UncachedInputTokens)
	}
	if summary.CachedInputTokens != 80 {
		t.Fatalf("CachedInputTokens = %d, want 80", summary.CachedInputTokens)
	}
	if summary.TotalTokens != 150 {
		t.Fatalf("TotalTokens = %d, want 150", summary.TotalTokens)
	}
	if summary.EstimatedCachedCharacters != 320 {
		t.Fatalf("EstimatedCachedCharacters = %d, want 320", summary.EstimatedCachedCharacters)
	}
}

func TestSummarizeRequestUsage_AnthropicSplitCacheAccounting(t *testing.T) {
	summary := SummarizeRequestUsage([]UsageLogEntry{
		{
			Provider:     "anthropic",
			InputTokens:  50,
			OutputTokens: 20,
			RawData: map[string]any{
				"cache_read_input_tokens":     90,
				"cache_creation_input_tokens": 30,
			},
		},
	})
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.InputTokens != 170 {
		t.Fatalf("InputTokens = %d, want 170", summary.InputTokens)
	}
	if summary.UncachedInputTokens != 50 {
		t.Fatalf("UncachedInputTokens = %d, want 50", summary.UncachedInputTokens)
	}
	if summary.CachedInputTokens != 90 {
		t.Fatalf("CachedInputTokens = %d, want 90", summary.CachedInputTokens)
	}
	if summary.CacheWriteInputTokens != 30 {
		t.Fatalf("CacheWriteInputTokens = %d, want 30", summary.CacheWriteInputTokens)
	}
	if summary.TotalTokens != 190 {
		t.Fatalf("TotalTokens = %d, want 190", summary.TotalTokens)
	}
}

func TestSummarizeRequestUsage_AnthropicSplitCacheAccountingWithoutCacheFields(t *testing.T) {
	summary := SummarizeRequestUsage([]UsageLogEntry{
		{
			Provider:     "anthropic",
			InputTokens:  50,
			OutputTokens: 20,
		},
	})
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.InputTokens != 50 {
		t.Fatalf("InputTokens = %d, want 50", summary.InputTokens)
	}
	if summary.UncachedInputTokens != 50 {
		t.Fatalf("UncachedInputTokens = %d, want 50", summary.UncachedInputTokens)
	}
	if summary.CachedInputTokens != 0 {
		t.Fatalf("CachedInputTokens = %d, want 0", summary.CachedInputTokens)
	}
	if summary.CacheWriteInputTokens != 0 {
		t.Fatalf("CacheWriteInputTokens = %d, want 0", summary.CacheWriteInputTokens)
	}
	if summary.TotalTokens != 70 {
		t.Fatalf("TotalTokens = %d, want 70", summary.TotalTokens)
	}
}

func TestSummarizeRequestUsage_AggregatesCosts(t *testing.T) {
	inputCostA := 1.0
	outputCostA := 3.0
	totalCostA := 4.0
	inputCostB := 2.0
	totalCostB := 2.0

	summary := SummarizeRequestUsage([]UsageLogEntry{
		{
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 20,
			InputCost:    &inputCostA,
			OutputCost:   &outputCostA,
			TotalCost:    &totalCostA,
		},
		{
			Provider:     "openai",
			InputTokens:  50,
			OutputTokens: 0,
			InputCost:    &inputCostB,
			TotalCost:    &totalCostB,
		},
	})

	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.InputCost == nil || *summary.InputCost != 3.0 {
		t.Fatalf("InputCost = %v, want 3.0", summary.InputCost)
	}
	if summary.OutputCost == nil || *summary.OutputCost != 3.0 {
		t.Fatalf("OutputCost = %v, want 3.0", summary.OutputCost)
	}
	if summary.TotalCost == nil || *summary.TotalCost != 6.0 {
		t.Fatalf("TotalCost = %v, want 6.0", summary.TotalCost)
	}
}

func TestSummarizeRequestUsage_LeavesCostsNilWhenUnavailable(t *testing.T) {
	summary := SummarizeRequestUsage([]UsageLogEntry{
		{Provider: "openai", InputTokens: 100, OutputTokens: 20},
	})

	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.InputCost != nil || summary.OutputCost != nil || summary.TotalCost != nil {
		t.Fatalf("expected nil cost fields, got input=%v output=%v total=%v", summary.InputCost, summary.OutputCost, summary.TotalCost)
	}
}

func TestSummarizeUsageByRequestID(t *testing.T) {
	summaries := SummarizeUsageByRequestID(map[string][]UsageLogEntry{
		"req-1": {
			{Provider: "openai", InputTokens: 10, OutputTokens: 5},
		},
		"req-2": {
			{Provider: "openai", InputTokens: 20, OutputTokens: 10},
		},
	})
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries["req-1"].TotalTokens != 15 {
		t.Fatalf("summaries[req-1].TotalTokens = %d, want 15", summaries["req-1"].TotalTokens)
	}
	if summaries["req-2"].TotalTokens != 30 {
		t.Fatalf("summaries[req-2].TotalTokens = %d, want 30", summaries["req-2"].TotalTokens)
	}
}
