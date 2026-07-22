package usage

import (
	"testing"
	"time"

	"aurora/internal/core"
)

type recordingPricingResolver struct {
	model    string
	provider string
	pricing  *core.ModelPricing
}

func (r *recordingPricingResolver) ResolvePricing(model, providerType string) *PricingResult {
	r.model = model
	r.provider = providerType
	if r.pricing == nil {
		return nil
	}
	return &PricingResult{Pricing: r.pricing}
}

func TestRecalculateEntryCostsPrefersProviderNameForPricingLookup(t *testing.T) {
	inputRate := 2.0
	cachedRate := 0.5
	resolver := &recordingPricingResolver{
		pricing: &core.ModelPricing{
			InputPerMtok:       &inputRate,
			CachedInputPerMtok: &cachedRate,
		},
	}

	update := recalculateEntryCosts(recalculationEntry{
		ID:           "usage-1",
		Model:        "gpt-4o",
		Provider:     "openai",
		ProviderName: "primary-openai",
		InputTokens:  1_000_000,
		RawData: map[string]any{
			"cached_tokens": 500_000,
		},
	}, resolver)

	if resolver.model != "gpt-4o" || resolver.provider != "primary-openai" {
		t.Fatalf("ResolvePricing called with %q/%q, want gpt-4o/primary-openai", resolver.provider, resolver.model)
	}
	if update.InputCost == nil || *update.InputCost != 1.25 {
		t.Fatalf("InputCost = %v, want 1.25", update.InputCost)
	}
}

// provenanceTestResolver returns pricing + provenance for verification
type provenanceTestResolver struct {
	source    string
	version   string
	resolvedAt time.Time
}

func (r *provenanceTestResolver) ResolvePricing(model, providerType string) *PricingResult {
	return &PricingResult{
		Pricing: &core.ModelPricing{
			InputPerMtok:  floatPtr(3.0),
			OutputPerMtok: floatPtr(15.0),
		},
		Provenance: &PricingProvenance{
			Source:     r.source,
			Version:    r.version,
			ResolvedAt: r.resolvedAt,
		},
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestRecalculateEntryCosts_CarriesProvenance(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	resolver := &provenanceTestResolver{
		source:     "test_registry",
		version:    "99",
		resolvedAt: now,
	}

	update := recalculateEntryCosts(recalculationEntry{
		ID:           "usage-1",
		Model:        "gpt-4o",
		Provider:     "openai",
		ProviderName: "primary-openai",
		InputTokens:  1000,
		OutputTokens: 500,
	}, resolver)

	if update.PricingProvenanceSource != "test_registry" {
		t.Fatalf("PricingProvenanceSource = %q, want %q", update.PricingProvenanceSource, "test_registry")
	}
	if update.PricingProvenanceVersion != "99" {
		t.Fatalf("PricingProvenanceVersion = %q, want %q", update.PricingProvenanceVersion, "99")
	}
	if update.PricingProvenanceResolvedAt == nil || !update.PricingProvenanceResolvedAt.Equal(now) {
		t.Fatalf("PricingProvenanceResolvedAt = %v, want %v", update.PricingProvenanceResolvedAt, now)
	}
}

func TestRecalculateEntryCosts_ProviderReportedUsesProviderReportedSource(t *testing.T) {
	t.Run("openrouter", func(t *testing.T) {
		update := recalculateEntryCosts(recalculationEntry{
			ID:           "usage-or",
			Model:        "openai/gpt-4o",
			Provider:     "openrouter",
			InputTokens:  10,
			OutputTokens: 4,
			RawData:      map[string]any{"cost": 0.00014},
		}, nil)
		if update.PricingProvenanceSource != "provider_reported" {
			t.Fatalf("PricingProvenanceSource = %q, want %q", update.PricingProvenanceSource, "provider_reported")
		}
	})

	t.Run("xai", func(t *testing.T) {
		update := recalculateEntryCosts(recalculationEntry{
			ID:           "usage-xai",
			Model:        "grok-4.3",
			Provider:     "xai",
			InputTokens:  199,
			OutputTokens: 1,
			RawData:      map[string]any{"cost_in_usd_ticks": float64(158_500)},
		}, nil)
		if update.PricingProvenanceSource != "provider_reported" {
			t.Fatalf("PricingProvenanceSource = %q, want %q", update.PricingProvenanceSource, "provider_reported")
		}
	})

	t.Run("provenance from resolver ignored for provider_reported", func(t *testing.T) {
		now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		resolver := &provenanceTestResolver{
			source:     "upstream_registry",
			version:    "42",
			resolvedAt: now,
		}
		update := recalculateEntryCosts(recalculationEntry{
			ID:           "usage-or-resolver",
			Model:        "openai/gpt-4o",
			Provider:     "openrouter",
			InputTokens:  10,
			OutputTokens: 4,
			RawData:      map[string]any{"cost": 0.00014},
		}, resolver)
		if update.PricingProvenanceSource != "provider_reported" {
			t.Fatalf("PricingProvenanceSource = %q, want %q", update.PricingProvenanceSource, "provider_reported")
		}
		// Resolver provenance must NOT leak for provider-reported
		if update.PricingProvenanceVersion != "" {
			t.Fatalf("PricingProvenanceVersion = %q, want empty", update.PricingProvenanceVersion)
		}
		if update.PricingProvenanceResolvedAt != nil {
			t.Fatalf("PricingProvenanceResolvedAt = %v, want nil", update.PricingProvenanceResolvedAt)
		}
	})
}
