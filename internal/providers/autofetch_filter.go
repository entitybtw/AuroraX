package providers

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"aurora/configuration"
	"aurora/internal/core"
)

// autofetchFilterMode controls how the individual conditions in an
// AutoFetchFilter are combined.
type autofetchFilterMode string

const (
	// autofetchFilterModeAll requires every condition to match (logical AND).
	// This is the default when no mode is declared.
	autofetchFilterModeAll autofetchFilterMode = "all"
	// autofetchFilterModeAny requires at least one condition to match (logical OR).
	autofetchFilterModeAny autofetchFilterMode = "any"
)

// compiledAutoFetchFilter is the validated, ready-to-match form of a provider's
// auto-fetch filter. A nil *compiledAutoFetchFilter is a no-op: every model passes.
type compiledAutoFetchFilter struct {
	mode       autofetchFilterMode
	conditions []compiledAutoFetchCondition
}

// compiledAutoFetchCondition is a single validated condition.
type compiledAutoFetchCondition struct {
	contains    string
	notContains string
	re          *regexp.Regexp
	// Price ceilings. A non-nil pointer means the condition is active; the
	// model's price must be present and <= the value to match.
	maxPrice           *float64
	maxPromptPrice     *float64
	maxCompletionPrice *float64
	// requireFree reports that the model must be free (price present and zero).
	// Set when max_price is configured as 0.
	raw string
}

// compileAutoFetchFilter validates a raw config.FilterCondition list and returns
// the compiled matcher. An empty condition list yields (nil, nil) — a no-op
// filter, which is what an unset autofetch_filter should behave as.
//
// Errors are returned for malformed input (bad regex, unknown operator, empty
// value) so that misconfiguration surfaces in the admin API and logs rather
// than silently disabling filtering.
func compileAutoFetchFilter(providerName string, raw config.AutoFetchFilter) (*compiledAutoFetchFilter, error) {
	if raw.IsZero() {
		return nil, nil
	}

	mode := autofetchFilterModeAll
	switch strings.ToLower(strings.TrimSpace(raw.Mode)) {
	case "":
		mode = autofetchFilterModeAll
	case string(autofetchFilterModeAll):
		mode = autofetchFilterModeAll
	case string(autofetchFilterModeAny):
		mode = autofetchFilterModeAny
	default:
		return nil, fmt.Errorf("provider %q: autofetch_filter.mode must be %q or %q (got %q)",
			providerName, autofetchFilterModeAll, autofetchFilterModeAny, raw.Mode)
	}

	compiled := &compiledAutoFetchFilter{
		mode:       mode,
		conditions: make([]compiledAutoFetchCondition, 0, len(raw.Conditions)),
	}

	for i, condition := range raw.Conditions {
		c, err := compileAutoFetchCondition(providerName, i, condition)
		if err != nil {
			return nil, err
		}
		compiled.conditions = append(compiled.conditions, c)
	}

	if len(compiled.conditions) == 0 {
		return nil, nil
	}
	return compiled, nil
}

func compileAutoFetchCondition(providerName string, index int, condition config.AutoFetchFilterCondition) (compiledAutoFetchCondition, error) {
	where := fmt.Sprintf("provider %q: autofetch_filter.conditions[%d]", providerName, index)

	var out compiledAutoFetchCondition
	out.raw = strings.TrimSpace(condition.String())

	out.contains = strings.ToLower(strings.TrimSpace(condition.Contains))
	out.notContains = strings.ToLower(strings.TrimSpace(condition.NotContains))

	if pattern := strings.TrimSpace(condition.Regex); pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return out, fmt.Errorf("%s: invalid regex %q: %w", where, pattern, err)
		}
		out.re = re
	}

	if condition.MaxPrice != nil {
		out.maxPrice = condition.MaxPrice
	}
	if condition.MaxPromptPrice != nil {
		out.maxPromptPrice = condition.MaxPromptPrice
	}
	if condition.MaxCompletionPrice != nil {
		out.maxCompletionPrice = condition.MaxCompletionPrice
	}

	if out.isEmpty() {
		return out, fmt.Errorf("%s: condition has no effective content — set one of contains, not_contains, regex, max_price, max_prompt_price, max_completion_price", where)
	}
	return out, nil
}

func (c compiledAutoFetchCondition) isEmpty() bool {
	return c.contains == "" &&
		c.notContains == "" &&
		c.re == nil &&
		c.maxPrice == nil &&
		c.maxPromptPrice == nil &&
		c.maxCompletionPrice == nil
}

// matches reports whether the model satisfies this condition.
func (c compiledAutoFetchCondition) matches(model core.Model) bool {
	id := strings.ToLower(strings.TrimSpace(model.ID))

	if c.contains != "" && !strings.Contains(id, c.contains) {
		return false
	}
	if c.notContains != "" && strings.Contains(id, c.notContains) {
		return false
	}
	if c.re != nil && !c.re.MatchString(model.ID) {
		return false
	}

	if !c.matchesPrice(model) {
		return false
	}
	return true
}

// matchesPrice evaluates the price-based conditions. A price condition only
// matches when the model actually carries pricing metadata: a model with no
// metadata cannot be proven free, and treating absent pricing as "free" would
// silently expose paid models.
func (c compiledAutoFetchCondition) matchesPrice(model core.Model) bool {
	if c.maxPrice == nil && c.maxPromptPrice == nil && c.maxCompletionPrice == nil {
		return true
	}
	pricing := modelPricing(model)
	if pricing == nil {
		return false
	}

	if c.maxPrice != nil {
		input, ok := priceValue(pricing.InputPerMtok)
		if !ok || input > *c.maxPrice {
			return false
		}
		output, ok := priceValue(pricing.OutputPerMtok)
		if !ok || output > *c.maxPrice {
			return false
		}
	}
	if c.maxPromptPrice != nil {
		input, ok := priceValue(pricing.InputPerMtok)
		if !ok || input > *c.maxPromptPrice {
			return false
		}
	}
	if c.maxCompletionPrice != nil {
		output, ok := priceValue(pricing.OutputPerMtok)
		if !ok || output > *c.maxCompletionPrice {
			return false
		}
	}
	return true
}

// modelPricing returns the model's pricing metadata, or nil when absent.
func modelPricing(model core.Model) *core.ModelPricing {
	if model.Metadata == nil {
		return nil
	}
	return model.Metadata.Pricing
}

// priceValue dereferences an optional per-million-token price. A missing value
// is not treated as zero: it reports false so callers can reject the model
// instead of assuming a price of 0.
func priceValue(v *float64) (float64, bool) {
	if v == nil {
		return 0, false
	}
	return *v, true
}

// matches reports whether a model passes the filter. A nil filter passes everything.
func (f *compiledAutoFetchFilter) matches(model core.Model) bool {
	if f == nil || len(f.conditions) == 0 {
		return true
	}

	switch f.mode {
	case autofetchFilterModeAny:
		for _, condition := range f.conditions {
			if condition.matches(model) {
				return true
			}
		}
		return false
	default: // autofetchFilterModeAll
		for _, condition := range f.conditions {
			if !condition.matches(model) {
				return false
			}
		}
		return true
	}
}

// applyAutoFetchFilter filters a provider's discovered model list down to the
// models that satisfy the filter. When the filter is nil or the response is nil
// the input is returned unchanged. The response is copied so the caller's
// provider-owned response is never mutated.
//
// Returns the filtered response and the number of models removed.
func applyAutoFetchFilter(providerName string, filter *compiledAutoFetchFilter, resp *core.ModelsResponse) (*core.ModelsResponse, int) {
	if filter == nil || resp == nil || len(resp.Data) == 0 {
		return resp, 0
	}

	kept := make([]core.Model, 0, len(resp.Data))
	removed := 0
	for _, model := range resp.Data {
		if filter.matches(model) {
			kept = append(kept, model)
			continue
		}
		removed++
	}

	if removed == 0 {
		return resp, 0
	}

	filtered := *resp
	filtered.Data = kept

	slog.Info("autofetch filter applied",
		"provider", providerName,
		"kept", len(kept),
		"removed", removed,
		"mode", string(filter.mode),
	)
	return &filtered, removed
}
