package usage

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/core"
)

const (
	CostSourceModelPricing      = "model_pricing"
	CostSourceOpenRouterCredits = "openrouter_credits"
	CostSourceXAITicks          = "xai_cost_in_usd_ticks"
)

// xAI reports usage.cost_in_usd_ticks as USD-denominated ticks, where 10^10
// ticks equals 1 USD.
const xaiUSDTicksPerUSD = 10_000_000_000

// CostResult holds the result of a granular cost calculation.
type CostResult struct {
	InputCost  *float64
	OutputCost *float64
	TotalCost  *float64
	Caveat     string
	Source     string

	// Provenance is set by the caller to record which pricing data was used.
	// CalculateUsageCost does not populate these; callers must set them from
	// the PricingResult returned by the resolver.
	PricingSource    string
	PricingVersion   string
	PricingResolvedAt *time.Time
}

// costSide indicates whether a token cost contributes to input or output.
type costSide int

const (
	sideInput costSide = iota
	sideOutput
)

// costUnit indicates how the pricing field is applied.
type costUnit int

const (
	unitPerMtok costUnit = iota // divide token count by 1M, multiply by rate
	unitPerItem                 // multiply count directly by rate
)

// tokenCostMapping maps a RawData key to a pricing field and cost side.
type tokenCostMapping struct {
	rawDataKey     string
	pricingField   func(p *core.ModelPricing) *float64
	side           costSide
	unit           costUnit
	includedInBase bool
}

var openAICompatibleTokenCostMappings = []tokenCostMapping{
	{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	{rawDataKey: "reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
	{rawDataKey: "completion_reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
	{rawDataKey: "prompt_audio_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.AudioInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	{rawDataKey: "completion_audio_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.AudioOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
}

// providerMappings defines the per-provider RawData key to pricing field mappings.
var providerMappings = map[string][]tokenCostMapping{
	"openai":     openAICompatibleTokenCostMappings,
	"openrouter": openAICompatibleTokenCostMappings,
	"anthropic": {
		{rawDataKey: "cache_read_input_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok},
		{rawDataKey: "cache_creation_input_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CacheWritePerMtok }, side: sideInput, unit: unitPerMtok},
	},
	"gemini": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "thought_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
	},
	"groq": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "completion_reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok, includedInBase: true},
	},
	"xai": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		// xAI reports reasoning tokens separately from completion_tokens.
		{rawDataKey: "reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok},
		{rawDataKey: "completion_reasoning_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, side: sideOutput, unit: unitPerMtok},
		{rawDataKey: "image_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.InputPerImage }, side: sideInput, unit: unitPerItem},
	},
	"deepseek": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	},
	"minimax": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	},
	"oracle": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	},
	"vllm": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	},
	"zai": {
		{rawDataKey: "cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
		{rawDataKey: "prompt_cached_tokens", pricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, side: sideInput, unit: unitPerMtok, includedInBase: true},
	},
}

// informationalFields are token fields that are known breakdowns of the base
// input/output counts. They never need separate pricing and should not trigger
// "unmapped token field" caveats.
var informationalFields = map[string]struct{}{
	"prompt_text_tokens":                    {},
	"prompt_image_tokens":                   {},
	"completion_accepted_prediction_tokens": {},
	"completion_rejected_prediction_tokens": {},
	"cache_read_input_tokens":               {},
	"cache_creation_input_tokens":           {},
}

// CalculateGranularCost computes input, output, and total costs from token counts,
// raw provider-specific data, and pricing information. It accounts for cached tokens,
// reasoning tokens, audio tokens, and other provider-specific token types.
//
// The caveat field in the result describes any unmapped token fields or missing pricing
// data that prevented full cost calculation.
func CalculateGranularCost(inputTokens, outputTokens int, rawData map[string]any, providerType string, pricing *core.ModelPricing) CostResult {
	if pricing == nil {
		return CostResult{}
	}

	var inputCost, outputCost float64
	var hasInput, hasOutput bool
	var caveats []string

	if inputTokens < 0 {
		caveats = append(caveats, "negative input tokens ignored")
		inputTokens = 0
	}
	if outputTokens < 0 {
		caveats = append(caveats, "negative output tokens ignored")
		outputTokens = 0
	}

	// Track which RawData keys are mapped
	mappedKeys := make(map[string]bool)

	// Base input cost
	if pricing.InputPerMtok != nil {
		inputCost += float64(inputTokens) * *pricing.InputPerMtok / 1_000_000
		hasInput = true
	}

	// Base output cost
	if pricing.OutputPerMtok != nil {
		outputCost += float64(outputTokens) * *pricing.OutputPerMtok / 1_000_000
		hasOutput = true
	}

	// Apply provider-specific mappings.
	// Track applied pricing field pointers to avoid double-counting when multiple
	// rawData keys map to the same pricing field (e.g. cached_tokens and prompt_cached_tokens
	// both map to CachedInputPerMtok).
	appliedFields := make(map[*float64]bool)
	remainingIncludedBaseTokens := map[costSide]int{
		sideInput:  inputTokens,
		sideOutput: outputTokens,
	}
	if mappings, ok := providerMappings[providerType]; ok {
		for _, m := range mappings {
			count := extractInt(rawData, m.rawDataKey)
			if count == 0 {
				continue
			}
			mappedKeys[m.rawDataKey] = true
			if count < 0 {
				caveats = append(caveats, fmt.Sprintf("negative token field ignored: %s", m.rawDataKey))
				continue
			}

			rate := m.pricingField(pricing)
			if rate == nil {
				continue // Base rate covers this token type; no adjustment needed
			}

			if appliedFields[rate] {
				continue // Already applied via a different rawData key for the same pricing field
			}
			appliedFields[rate] = true

			var cost float64
			switch m.unit {
			case unitPerMtok:
				coveredTokens := includedBaseCoveredTokens(count, remainingIncludedBaseTokens[m.side], m)
				cost = tokenMappingCost(count, coveredTokens, *rate, pricing, m)
				if m.includedInBase {
					remainingIncludedBaseTokens[m.side] -= coveredTokens
					if count > coveredTokens {
						caveats = append(caveats, fmt.Sprintf("%s exceeds remaining base %s tokens; overflow priced at mapped rate", m.rawDataKey, sideName(m.side)))
					}
				}
			case unitPerItem:
				cost = float64(count) * *rate
			}

			switch m.side {
			case sideInput:
				inputCost += cost
				hasInput = true
			case sideOutput:
				outputCost += cost
				hasOutput = true
			}
		}
	}

	// Check for unmapped token fields in RawData
	for key := range rawData {
		if mappedKeys[key] {
			continue
		}
		if _, ok := informationalFields[key]; ok {
			continue // Known breakdown of base counts, not separately priced
		}
		if isTokenField(key) {
			count := extractInt(rawData, key)
			if count > 0 {
				caveats = append(caveats, fmt.Sprintf("unmapped token field: %s", key))
			}
		}
	}

	// Add per-request flat fee
	if pricing.PerRequest != nil {
		outputCost += *pricing.PerRequest
		hasOutput = true
	}

	result := CostResult{}

	if hasInput && inputCost < 0 {
		inputCost = 0
		caveats = append(caveats, "negative input cost clamped to zero")
	}
	if hasOutput && outputCost < 0 {
		outputCost = 0
		caveats = append(caveats, "negative output cost clamped to zero")
	}

	if hasInput {
		result.InputCost = &inputCost
	}
	if hasOutput {
		result.OutputCost = &outputCost
	}
	if hasInput || hasOutput {
		total := inputCost + outputCost
		result.TotalCost = &total
		result.Source = CostSourceModelPricing
	}

	// Sort caveats for deterministic output
	sort.Strings(caveats)
	result.Caveat = strings.Join(caveats, "; ")

	return result
}

func includedBaseCoveredTokens(count, remainingBaseTokens int, mapping tokenCostMapping) int {
	if !mapping.includedInBase {
		return 0
	}
	return minInt(count, remainingBaseTokens)
}

func tokenMappingCost(count, coveredTokens int, rate float64, pricing *core.ModelPricing, mapping tokenCostMapping) float64 {
	if !mapping.includedInBase {
		return float64(count) * rate / 1_000_000
	}
	baseRate := baseRateForSide(pricing, mapping.side)
	if baseRate == nil {
		return float64(count) * rate / 1_000_000
	}
	overflowTokens := count - coveredTokens
	return (float64(coveredTokens)*(rate-*baseRate) + float64(overflowTokens)*rate) / 1_000_000
}

func sideName(side costSide) string {
	switch side {
	case sideInput:
		return "input"
	case sideOutput:
		return "output"
	default:
		return "unknown"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxPlatformInt() int {
	return int(^uint(0) >> 1)
}

func minPlatformInt() int {
	return -maxPlatformInt() - 1
}

func baseRateForSide(pricing *core.ModelPricing, side costSide) *float64 {
	if pricing == nil {
		return nil
	}
	switch side {
	case sideInput:
		return pricing.InputPerMtok
	case sideOutput:
		return pricing.OutputPerMtok
	default:
		return nil
	}
}

// extractInt extracts an integer value from a map, handling float64, int, and int64 types.
// Returns 0 if the key is not found or the value is not a numeric type.
func extractInt(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || n > float64(maxPlatformInt()) || n < float64(minPlatformInt()) {
			return 0
		}
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

// isTokenField returns true if the key looks like a token count field.
func isTokenField(key string) bool {
	return strings.HasSuffix(key, "_tokens") || strings.HasSuffix(key, "_count")
}

// CalculateUsageCost prefers provider-supplied exact costs when available and
// falls back to static model pricing otherwise.
func CalculateUsageCost(inputTokens, outputTokens int, rawData map[string]any, providerType string, pricing *core.ModelPricing) CostResult {
	if result, ok := openRouterCreditCost(rawData, providerType); ok {
		return result
	}
	if result, ok := xaiTicksCost(rawData, providerType); ok {
		return result
	}
	return CalculateGranularCost(inputTokens, outputTokens, rawData, providerType, pricing)
}

func openRouterCreditCost(rawData map[string]any, providerType string) (CostResult, bool) {
	if !isOpenRouterProvider(providerType) {
		return CostResult{}, false
	}
	total, ok := extractFloat(rawData, "cost")
	if !ok || !isFiniteCost(total) || total < 0 {
		return CostResult{}, false
	}

	// OpenRouter reports cost in credits; their credit system is USD-based, so
	// this is the right value for aurora's existing USD cost fields.
	result := CostResult{
		TotalCost: &total,
		Source:    CostSourceOpenRouterCredits,
	}
	if inputCost, outputCost, ok := openRouterCreditCostSplit(rawData, total); ok {
		result.InputCost = &inputCost
		result.OutputCost = &outputCost
	}
	return result, true
}

func isOpenRouterProvider(providerType string) bool {
	return strings.EqualFold(strings.TrimSpace(providerType), "openrouter")
}

func xaiTicksCost(rawData map[string]any, providerType string) (CostResult, bool) {
	if !isXAIProvider(providerType) {
		return CostResult{}, false
	}
	ticks, ok := extractFloat(rawData, "cost_in_usd_ticks")
	if !ok || !isFiniteCost(ticks) || ticks < 0 {
		return CostResult{}, false
	}

	total := ticks / xaiUSDTicksPerUSD
	return CostResult{
		TotalCost: &total,
		Source:    CostSourceXAITicks,
	}, true
}

func isXAIProvider(providerType string) bool {
	return strings.EqualFold(strings.TrimSpace(providerType), "xai")
}

func openRouterCreditCostSplit(rawData map[string]any, total float64) (float64, float64, bool) {
	details, ok := nestedUsageMap(rawData["cost_details"])
	if !ok {
		return 0, 0, false
	}

	input, inputOK := firstNestedFloat(details,
		"upstream_inference_prompt_cost",
		"upstream_inference_input_cost",
	)
	output, outputOK := firstNestedFloat(details,
		"upstream_inference_completions_cost",
		"upstream_inference_completion_cost",
		"upstream_inference_output_cost",
	)
	if !inputOK || !outputOK || !isFiniteCost(input) || !isFiniteCost(output) || input < 0 || output < 0 {
		return 0, 0, false
	}

	if !costsNearlyEqual(input+output, total) {
		return 0, 0, false
	}
	return input, output, true
}

func isFiniteCost(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func firstNestedFloat(data map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := extractFloat(data, key); ok {
			return value, true
		}
	}
	return 0, false
}

func nestedUsageMap(value any) (map[string]any, bool) {
	if typed, ok := value.(map[string]any); ok {
		return typed, true
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Map || reflected.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]any, reflected.Len())
	iter := reflected.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}

func extractFloat(data map[string]any, key string) (float64, bool) {
	if len(data) == 0 {
		return 0, false
	}
	value, ok := data[key]
	if !ok {
		return 0, false
	}
	return numericFloat(value)
}

func numericFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		f, err := strconv.ParseFloat(typed.String(), 64)
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func costsNearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= max(1e-12, math.Abs(b)*1e-6)
}
