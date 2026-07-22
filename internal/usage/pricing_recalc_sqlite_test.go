package usage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"aurora/internal/core"
)

type staticTestPricingResolver map[string]*core.ModelPricing

func (r staticTestPricingResolver) ResolvePricing(model, providerType string) *PricingResult {
	if p := r[providerType+"/"+model]; p != nil {
		return &PricingResult{Pricing: p}
	}
	return nil
}

// provenanceAwarePricingResolver wraps pricing with provenance for test persistence verification
type provenanceAwarePricingResolver struct {
	pricing    map[string]*core.ModelPricing
	provenance PricingProvenance
}

func (r provenanceAwarePricingResolver) ResolvePricing(model, providerType string) *PricingResult {
	if p := r.pricing[providerType+"/"+model]; p != nil {
		return &PricingResult{Pricing: p, Provenance: &r.provenance}
	}
	return nil
}

func TestSQLiteStoreRecalculatePricing_PreservesProvenance(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	oldCost := 99.0
	now := time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)
	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*UsageEntry{
		{
			ID:           "usage-prov",
			RequestID:    "req-prov",
			ProviderID:   "provider-prov",
			Timestamp:    time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
			Model:        "gpt-4o",
			Provider:     "openai",
			ProviderName: "primary-openai",
			Endpoint:     "/v1/chat/completions",
			InputTokens:  1_000_000,
			OutputTokens: 500_000,
			TotalTokens:  1_500_000,
			InputCost:    &oldCost,
			OutputCost:   &oldCost,
			TotalCost:    &oldCost,
		},
	}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	inputRate := 2.0
	outputRate := 6.0
	resolver := provenanceAwarePricingResolver{
		pricing: map[string]*core.ModelPricing{
			"primary-openai/gpt-4o": {InputPerMtok: &inputRate, OutputPerMtok: &outputRate},
		},
		provenance: PricingProvenance{
			Source:     "test_registry",
			Version:    "100",
			ResolvedAt: now,
		},
	}

	result, err := store.RecalculatePricing(ctx, RecalculatePricingParams{
		UsageQueryParams: UsageQueryParams{
			StartDate: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		},
		Model: "gpt-4o",
	}, resolver)
	if err != nil {
		t.Fatalf("RecalculatePricing() error = %v", err)
	}
	if result.Matched != 1 || result.Recalculated != 1 {
		t.Fatalf("result.Matched = %d, Recalculated = %d, want 1", result.Matched, result.Recalculated)
	}

	// Verify provenance was persisted
	var pricingSource, pricingVersion string
	var pricingResolvedAt *string
	if err := db.QueryRow(
		`SELECT COALESCE(pricing_source, ''), COALESCE(pricing_version, ''), pricing_resolved_at FROM usage WHERE id = 'usage-prov'`,
	).Scan(&pricingSource, &pricingVersion, &pricingResolvedAt); err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if pricingSource != "test_registry" {
		t.Fatalf("pricing_source = %q, want %q", pricingSource, "test_registry")
	}
	if pricingVersion != "100" {
		t.Fatalf("pricing_version = %q, want %q", pricingVersion, "100")
	}
	if pricingResolvedAt == nil {
		t.Fatal("pricing_resolved_at should not be NULL after recalculation with provenance")
	}
	if *pricingResolvedAt != "2026-07-14T12:30:00Z" {
		t.Fatalf("pricing_resolved_at = %q, want %q", *pricingResolvedAt, "2026-07-14T12:30:00Z")
	}
}

func TestSQLiteStoreRecalculatePricingUpdatesFilteredUsageCosts(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	oldCost := 99.0
	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*UsageEntry{
		{
			ID:           "usage-match",
			RequestID:    "req-match",
			ProviderID:   "provider-match",
			Timestamp:    time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
			Model:        "gpt-4o",
			Provider:     "openai",
			ProviderName: "primary-openai",
			Endpoint:     "/v1/chat/completions",
			UserPath:     "/team/alpha",
			InputTokens:  1_000_000,
			OutputTokens: 500_000,
			TotalTokens:  1_500_000,
			InputCost:    &oldCost,
			OutputCost:   &oldCost,
			TotalCost:    &oldCost,
		},
		{
			ID:          "usage-other-model",
			RequestID:   "req-other",
			ProviderID:  "provider-other",
			Timestamp:   time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC),
			Model:       "gpt-4o-mini",
			Provider:    "openai",
			Endpoint:    "/v1/chat/completions",
			UserPath:    "/team/alpha",
			InputTokens: 1_000_000,
			TotalTokens: 1_000_000,
			TotalCost:   &oldCost,
		},
	}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	inputRate := 2.0
	outputRate := 6.0
	result, err := store.RecalculatePricing(ctx, RecalculatePricingParams{
		UsageQueryParams: UsageQueryParams{
			StartDate: time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
			UserPath:  "/team",
		},
		Provider: "primary-openai",
		Model:    "gpt-4o",
	}, staticTestPricingResolver{
		"primary-openai/gpt-4o": {
			InputPerMtok:  &inputRate,
			OutputPerMtok: &outputRate,
		},
	})
	if err != nil {
		t.Fatalf("RecalculatePricing() error = %v", err)
	}
	if result.Matched != 1 || result.Recalculated != 1 || result.WithPricing != 1 || result.WithoutPricing != 0 {
		t.Fatalf("result = %+v, want one recalculated row with pricing", result)
	}

	var inputCost, outputCost, totalCost float64
	if err := db.QueryRow(`SELECT input_cost, output_cost, total_cost FROM usage WHERE id = 'usage-match'`).Scan(&inputCost, &outputCost, &totalCost); err != nil {
		t.Fatalf("query recalculated row: %v", err)
	}
	if inputCost != 2.0 || outputCost != 3.0 || totalCost != 5.0 {
		t.Fatalf("costs = input %.4f output %.4f total %.4f, want 2/3/5", inputCost, outputCost, totalCost)
	}

	var otherTotal float64
	if err := db.QueryRow(`SELECT total_cost FROM usage WHERE id = 'usage-other-model'`).Scan(&otherTotal); err != nil {
		t.Fatalf("query untouched row: %v", err)
	}
	if otherTotal != oldCost {
		t.Fatalf("other total cost = %.4f, want %.4f", otherTotal, oldCost)
	}
}

func TestSQLiteStoreRecalculatePricingProcessesBatches(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	store.recalculationBatchSize = 1

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*UsageEntry{
		{
			ID:          "usage-1",
			RequestID:   "req-1",
			ProviderID:  "provider-1",
			Timestamp:   time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
			Model:       "gpt-4o",
			Provider:    "openai",
			Endpoint:    "/v1/chat/completions",
			InputTokens: 1_000_000,
		},
		{
			ID:           "usage-2",
			RequestID:    "req-2",
			ProviderID:   "provider-2",
			Timestamp:    time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC),
			Model:        "gpt-4o",
			Provider:     "openai",
			ProviderName: "primary-openai",
			Endpoint:     "/v1/chat/completions",
			InputTokens:  2_000_000,
		},
	}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	inputRate := 2.0
	result, err := store.RecalculatePricing(ctx, RecalculatePricingParams{
		Model: "gpt-4o",
	}, staticTestPricingResolver{
		"openai/gpt-4o": {
			InputPerMtok: &inputRate,
		},
		"primary-openai/gpt-4o": {
			InputPerMtok: &inputRate,
		},
	})
	if err != nil {
		t.Fatalf("RecalculatePricing() error = %v", err)
	}
	if result.Matched != 2 || result.Recalculated != 2 || result.WithPricing != 2 {
		t.Fatalf("result = %+v, want two recalculated rows with pricing", result)
	}

	rows, err := db.Query(`SELECT id, input_cost FROM usage ORDER BY id`)
	if err != nil {
		t.Fatalf("query recalculated rows: %v", err)
	}
	defer rows.Close()

	got := map[string]float64{}
	for rows.Next() {
		var id string
		var inputCost float64
		if err := rows.Scan(&id, &inputCost); err != nil {
			t.Fatalf("scan recalculated row: %v", err)
		}
		got[id] = inputCost
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate recalculated rows: %v", err)
	}
	if got["usage-1"] != 2.0 || got["usage-2"] != 4.0 {
		t.Fatalf("input costs = %+v, want usage-1=2 usage-2=4", got)
	}
}
