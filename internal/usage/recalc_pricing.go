package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurora/internal/core"
)

// RecalculatePricingParams identifies the stored usage rows whose costs should
// be recalculated from the latest model pricing metadata.
type RecalculatePricingParams struct {
	UsageQueryParams
	Model    string
	Provider string
}

// RecalculatePricingResult summarizes a pricing recalculation run.
type RecalculatePricingResult struct {
	Status         string `json:"status"`
	Matched        int64  `json:"matched"`
	Recalculated   int64  `json:"recalculated"`
	WithPricing    int64  `json:"with_pricing"`
	WithoutPricing int64  `json:"without_pricing"`
}

// PricingRecalculator updates persisted usage cost fields from current pricing metadata.
type PricingRecalculator interface {
	RecalculatePricing(ctx context.Context, params RecalculatePricingParams, resolver PricingResolver) (RecalculatePricingResult, error)
}

type recalculationEntry struct {
	ID           string
	Model        string
	Provider     string
	ProviderName string
	Endpoint     string
	InputTokens  int
	OutputTokens int
	RawData      map[string]any
}

type recalculationUpdate struct {
	ID         string
	InputCost  *float64
	OutputCost *float64
	TotalCost  *float64
	CostSource string
	Caveat     string
	HasPricing bool

	// Pricing provenance (carried through recalculation)
	PricingProvenanceSource      string
	PricingProvenanceVersion     string
	PricingProvenanceResolvedAt *time.Time
}

func normalizedRecalculatePricingParams(params RecalculatePricingParams) RecalculatePricingParams {
	params.Model = strings.TrimSpace(params.Model)
	params.Provider = strings.TrimSpace(params.Provider)
	params.CacheMode = CacheModeAll
	return params
}

func recalculateEntryCosts(entry recalculationEntry, resolver PricingResolver) recalculationUpdate {
	pricingProvider := effectiveRecalculationPricingProvider(entry.Provider, entry.ProviderName)
	var pricing *core.ModelPricing
	var provenance *PricingProvenance
	if resolver != nil {
		if pr := resolver.ResolvePricing(entry.Model, pricingProvider); pr != nil {
			pricing = pr.Pricing
			provenance = pr.Provenance
		}
	}
	effectivePricing := pricingForEndpoint(pricing, entry.Endpoint)
	result := CalculateUsageCost(entry.InputTokens, entry.OutputTokens, entry.RawData, entry.Provider, effectivePricing)
	update := recalculationUpdate{
		ID:         entry.ID,
		InputCost:  result.InputCost,
		OutputCost: result.OutputCost,
		TotalCost:  result.TotalCost,
		CostSource: result.Source,
		Caveat:     result.Caveat,
		HasPricing: result.TotalCost != nil || result.InputCost != nil || result.OutputCost != nil,
	}
	switch result.Source {
	case CostSourceOpenRouterCredits, CostSourceXAITicks:
		update.PricingProvenanceSource = "provider_reported"
	default:
		if provenance != nil {
			update.PricingProvenanceSource = provenance.Source
			update.PricingProvenanceVersion = provenance.Version
			if !provenance.ResolvedAt.IsZero() {
				update.PricingProvenanceResolvedAt = &provenance.ResolvedAt
			}
		} else {
			update.PricingProvenanceSource = result.Source
		}
	}
	return update
}

func effectiveRecalculationPricingProvider(provider, providerName string) string {
	if name := strings.TrimSpace(providerName); name != "" {
		return name
	}
	return strings.TrimSpace(provider)
}

func updateRecalculatePricingResult(result *RecalculatePricingResult, update recalculationUpdate) {
	result.Matched++
	result.Recalculated++
	if update.HasPricing {
		result.WithPricing++
	} else {
		result.WithoutPricing++
	}
}

func finalizeRecalculatePricingResult(result RecalculatePricingResult) RecalculatePricingResult {
	if result.Status == "" {
		result.Status = "ok"
	}
	return result
}

func rawDataFromJSON(raw, entryID string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		slog.Warn("failed to unmarshal usage raw_data for pricing recalculation", "id", entryID, "error", err)
		return nil
	}
	return data
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func recalculatePricingUnavailable(resolver PricingResolver) error {
	if resolver == nil {
		return fmt.Errorf("pricing resolver is required")
	}
	return nil
}
