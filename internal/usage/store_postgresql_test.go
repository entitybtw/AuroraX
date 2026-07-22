package usage

import (
	"strings"
	"testing"
	"time"
)

func TestBuildUsageInsert(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	inputCost := 0.1
	outputCost := 0.2
	totalCost := 0.3

	query, args := buildUsageInsert([]*UsageEntry{
		{
			ID:                     "usage-1",
			RequestID:              "req-1",
			ProviderID:             "provider-1",
			Timestamp:              now,
			Model:                  "gpt-4o-mini",
			Provider:               "openai",
			ProviderName:           "primary-openai",
			Endpoint:               "/v1/chat/completions",
			TenantID:               "tenant-a",
			UserPath:               "/team",
			CacheType:              CacheTypeExact,
			InputTokens:            10,
			OutputTokens:           5,
			TotalTokens:            15,
			RawData:                map[string]any{"cached_tokens": 3},
			InputCost:              &inputCost,
			OutputCost:             &outputCost,
			TotalCost:              &totalCost,
			CostSource:             CostSourceModelPricing,
			CostsCalculationCaveat: "none",
		},
		{
			ID:                     "usage-2",
			RequestID:              "req-2",
			ProviderID:             "provider-2",
			Timestamp:              now.Add(time.Second),
			Model:                  "gpt-4.1",
			Provider:               "openai",
			Endpoint:               "/v1/responses",
			UserPath:               "/_tenants/tenant-b/team",
			CacheType:              "unexpected-cache-type",
			InputTokens:            20,
			OutputTokens:           8,
			TotalTokens:            28,
			RawData:                nil,
			InputCost:              nil,
			OutputCost:             nil,
			TotalCost:              nil,
			CostsCalculationCaveat: "missing pricing for tool tokens",
		},
	})

	normalized := strings.Join(strings.Fields(query), " ")
	wantQuery := "INSERT INTO usage (id, request_id, provider_id, timestamp, model, provider, provider_name, endpoint, tenant_id, user_path, cache_type, input_tokens, output_tokens, total_tokens, raw_data, input_cost, output_cost, total_cost, cost_source, costs_calculation_caveat, pricing_source, pricing_version, pricing_resolved_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23), ($24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46) ON CONFLICT (id) DO NOTHING"
	if normalized != wantQuery {
		t.Fatalf("query = %q, want %q", normalized, wantQuery)
	}

	if got, want := len(args), 46; got != want {
		t.Fatalf("len(args) = %d, want %d", got, want)
	}
	if got := args[0]; got != "usage-1" {
		t.Fatalf("args[0] = %v, want usage-1", got)
	}
	if got := args[6]; got != "primary-openai" {
		t.Fatalf("args[6] = %v, want primary-openai", got)
	}
	if got := args[8]; got != "tenant-a" {
		t.Fatalf("args[8] = %v, want tenant-a", got)
	}
	if got := args[18]; got != CostSourceModelPricing {
		t.Fatalf("args[18] = %v, want %q", got, CostSourceModelPricing)
	}
	if got := args[20]; got != "" {
		t.Fatalf("args[20] = %v, want empty pricing_source", got)
	}
	if got := args[21]; got != "" {
		t.Fatalf("args[21] = %v, want empty pricing_version", got)
	}
	if got := args[22]; got != nil {
		pricingResolvedAt, ok := got.(*time.Time)
		if !ok || pricingResolvedAt != nil {
			t.Fatalf("args[22] = %v (%T), want nil *time.Time", got, got)
		}
	}
	if got := args[23]; got != "usage-2" {
		t.Fatalf("args[23] = %v, want usage-2", got)
	}
	if got := args[10]; got != CacheTypeExact {
		t.Fatalf("args[10] = %v, want %q", got, CacheTypeExact)
	}
	if got := string(args[14].([]byte)); got != `{"cached_tokens":3}` {
		t.Fatalf("args[14] = %q, want %q", got, `{"cached_tokens":3}`)
	}
	if got := args[31]; got != "tenant-b" {
		t.Fatalf("args[31] = %v, want tenant-b", got)
	}
	if got := args[32]; got != "/team" {
		t.Fatalf("args[32] = %v, want /team", got)
	}
	if got := args[33]; got != nil {
		t.Fatalf("args[33] = %v, want nil cache_type", got)
	}
	rawData, ok := args[37].([]byte)
	if !ok {
		t.Fatalf("args[37] has type %T, want []byte", args[37])
	}
	if rawData != nil {
		t.Fatalf("args[37] = %v, want nil raw_data", rawData)
	}
}

func TestUsageInsertMaxRowsPerQueryRespectsPostgresLimit(t *testing.T) {
	if got := usageInsertMaxRowsPerQuery * usageInsertColumnCount; got > postgresMaxBindParameters {
		t.Fatalf("bind parameters = %d, want <= %d", got, postgresMaxBindParameters)
	}
}
