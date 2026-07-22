package modeldata

import (
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/core"
)

func TestLoadUserOverrides_EmptyPathOrMissing(t *testing.T) {
	o, err := LoadUserOverrides("")
	if err != nil || o != nil {
		t.Errorf("empty path: want nil/nil, got %v / %v", o, err)
	}

	dir := t.TempDir()
	o, err = LoadUserOverrides(filepath.Join(dir, "missing.yaml"))
	if err != nil || o != nil {
		t.Errorf("missing file: want nil/nil, got %v / %v", o, err)
	}
}

func TestLoadUserOverrides_ParsesPricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_pricing.yaml")
	body := []byte(`
models:
  sora-2:
    pricing:
      currency: USD
      per_second_output: 0.10
  gpt-4o:
    context_window: 200000
    pricing:
      input_per_mtok: 2.40
provider_models:
  "openai/gpt-4o":
    pricing:
      currency: USD
      input_per_mtok: 2.30
      output_per_mtok: 9.50
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	o, err := LoadUserOverrides(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil overrides")
	}

	sora, ok := o.Models["sora-2"]
	if !ok || sora.Pricing == nil || sora.Pricing.PerSecondOutput == nil {
		t.Fatalf("sora-2 pricing not parsed: %+v", sora)
	}
	if got := *sora.Pricing.PerSecondOutput; got != 0.10 {
		t.Errorf("sora per_second_output = %v, want 0.10", got)
	}

	gpt, ok := o.Models["gpt-4o"]
	if !ok {
		t.Fatal("gpt-4o not parsed")
	}
	if gpt.ContextWindow == nil || *gpt.ContextWindow != 200000 {
		t.Errorf("gpt-4o context_window not parsed: %+v", gpt.ContextWindow)
	}
	if gpt.Pricing == nil || gpt.Pricing.InputPerMtok == nil || *gpt.Pricing.InputPerMtok != 2.40 {
		t.Errorf("gpt-4o input_per_mtok not parsed: %+v", gpt.Pricing)
	}

	pm, ok := o.ProviderModels["openai/gpt-4o"]
	if !ok || pm.Pricing == nil {
		t.Fatalf("provider_model override missing: %+v", pm)
	}
	if *pm.Pricing.InputPerMtok != 2.30 {
		t.Errorf("provider_model input = %v, want 2.30", *pm.Pricing.InputPerMtok)
	}
}

func TestApplyUserOverrides_FillsMissingPricing(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"sora-2": {DisplayName: "Sora 2", Modes: []string{"video_generation"}},
		},
		ProviderModels: map[string]ProviderModelEntry{},
	}
	persec := 0.10
	overrides := &UserOverrides{
		Models: map[string]ModelOverride{
			"sora-2": {
				Pricing: &core.ModelPricing{
					Currency:        "USD",
					PerSecondOutput: &persec,
				},
			},
		},
	}

	models, pms := ApplyUserOverrides(list, overrides)
	if models != 1 || pms != 0 {
		t.Errorf("counts: models=%d pms=%d, want 1/0", models, pms)
	}

	got := list.Models["sora-2"]
	if got.DisplayName != "Sora 2" {
		t.Errorf("display_name should be preserved; got %q", got.DisplayName)
	}
	if got.Pricing == nil || got.Pricing.PerSecondOutput == nil || *got.Pricing.PerSecondOutput != 0.10 {
		t.Errorf("pricing not applied: %+v", got.Pricing)
	}
}

func TestApplyUserOverrides_PerFieldMerge(t *testing.T) {
	in := 5.0
	out := 10.0
	cached := 2.5

	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName: "gpt-4o",
				Pricing: &core.ModelPricing{
					Currency:           "USD",
					InputPerMtok:       &in,
					OutputPerMtok:      &out,
					CachedInputPerMtok: &cached,
				},
			},
		},
	}

	newIn := 2.40
	overrides := &UserOverrides{
		Models: map[string]ModelOverride{
			"gpt-4o": {
				Pricing: &core.ModelPricing{
					InputPerMtok: &newIn,
				},
			},
		},
	}

	ApplyUserOverrides(list, overrides)
	got := list.Models["gpt-4o"].Pricing

	if got.InputPerMtok == nil || *got.InputPerMtok != 2.40 {
		t.Errorf("input override should win; got %v", got.InputPerMtok)
	}
	if got.OutputPerMtok == nil || *got.OutputPerMtok != 10.0 {
		t.Errorf("output should be preserved from base; got %v", got.OutputPerMtok)
	}
	if got.CachedInputPerMtok == nil || *got.CachedInputPerMtok != 2.5 {
		t.Errorf("cached should be preserved; got %v", got.CachedInputPerMtok)
	}
	if got.Currency != "USD" {
		t.Errorf("currency should be preserved; got %q", got.Currency)
	}
}

func TestApplyUserOverrides_InsertsNewModelEntry(t *testing.T) {
	list := &ModelList{
		Models:         map[string]ModelEntry{},
		ProviderModels: map[string]ProviderModelEntry{},
	}
	displayName := "Acme Custom"
	ctx := 32768
	overrides := &UserOverrides{
		Models: map[string]ModelOverride{
			"acme-custom": {
				DisplayName:   &displayName,
				ContextWindow: &ctx,
			},
		},
	}

	ApplyUserOverrides(list, overrides)
	got, ok := list.Models["acme-custom"]
	if !ok {
		t.Fatal("expected new entry to be inserted")
	}
	if got.DisplayName != "Acme Custom" {
		t.Errorf("display_name = %q", got.DisplayName)
	}
	if got.ContextWindow == nil || *got.ContextWindow != 32768 {
		t.Errorf("context_window not set")
	}
}

func TestApplyUserOverrides_NilSafe(t *testing.T) {
	if m, p := ApplyUserOverrides(nil, nil); m != 0 || p != 0 {
		t.Errorf("nil/nil: counts should be 0; got %d/%d", m, p)
	}
	list := &ModelList{Models: map[string]ModelEntry{"a": {DisplayName: "A"}}}
	if m, p := ApplyUserOverrides(list, nil); m != 0 || p != 0 {
		t.Errorf("nil overrides: counts should be 0; got %d/%d", m, p)
	}
}

func TestMergePricing_AllFields(t *testing.T) {
	makePtr := func(v float64) *float64 { return &v }

	base := &core.ModelPricing{
		Currency:               "USD",
		InputPerMtok:           makePtr(1),
		OutputPerMtok:          makePtr(2),
		CachedInputPerMtok:     makePtr(3),
		CacheWritePerMtok:      makePtr(4),
		ReasoningOutputPerMtok: makePtr(5),
	}
	override := &core.ModelPricing{
		InputPerMtok:  makePtr(10),
		OutputPerMtok: makePtr(20),
	}

	merged := mergePricing(base, override)
	if *merged.InputPerMtok != 10 || *merged.OutputPerMtok != 20 {
		t.Errorf("override scalars should win")
	}
	if *merged.CachedInputPerMtok != 3 || *merged.CacheWritePerMtok != 4 || *merged.ReasoningOutputPerMtok != 5 {
		t.Errorf("non-overridden scalars should persist from base")
	}
	// Mutating merged must not affect base.
	*merged.InputPerMtok = 999
	if *base.InputPerMtok != 1 {
		t.Error("merge result must be a deep copy; base was mutated")
	}
}

func TestMergePricing_NilCases(t *testing.T) {
	if got := mergePricing(nil, nil); got != nil {
		t.Errorf("nil/nil should be nil; got %+v", got)
	}
	in := 5.0
	override := &core.ModelPricing{Currency: "USD", InputPerMtok: &in}
	got := mergePricing(nil, override)
	if got == nil || got.InputPerMtok == nil || *got.InputPerMtok != 5 {
		t.Errorf("override-only merge failed")
	}
	// Returned must not share pointers with override.
	*got.InputPerMtok = 999
	if *override.InputPerMtok != 5 {
		t.Error("nil-base merge must clone override")
	}
}
