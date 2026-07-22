package usage

import (
	"time"

	"aurora/internal/core"
)

// PricingProvenance describes the origin and version of pricing data used
// for a cost calculation. This enables traceability when provider prices
// change or admin overrides are adjusted after records were created.
type PricingProvenance struct {
	// Source identifies which pricing layer produced the estimate:
	//   "upstream_registry"  — base registry or local snapshot, possibly with overrides merged
	//   "provider_reported"  — exact cost returned by the provider (OpenRouter, xAI)
	Source string `json:"source"`

	// Version is the ModelList version from the upstream registry at resolution time.
	// Empty when pricing is not from a versioned model list (e.g. provider-reported).
	Version string `json:"version"`

	// ResolvedAt is when the pricing was resolved. For provider-reported costs
	// this is the request completion time.
	ResolvedAt time.Time `json:"resolved_at"`
}

// PricingResult pairs resolved pricing with its provenance metadata.
// When no pricing is available for the model/provider, Pricing is nil
// but Provenance may still be populated (e.g. provider-reported costs).
type PricingResult struct {
	Pricing    *core.ModelPricing
	Provenance *PricingProvenance
}

// PricingResolver resolves pricing metadata for a given model and provider type.
// Returns nil when no pricing is available for the combination.
// Implementations should check the registry first and fall back to a reverse-index
// lookup when the model ID in the usage DB differs from the registry key.
type PricingResolver interface {
	ResolvePricing(model, providerType string) *PricingResult
}
