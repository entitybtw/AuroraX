package gateway

import "aurora/internal/core"

type ComboFallbackProvider interface {
	FallbacksFor(requestedModel string) []core.ModelSelector
}

type ComboFallbackResolver struct {
	combos ComboFallbackProvider
	next   FallbackResolver
}

func NewComboFallbackResolver(combos ComboFallbackProvider, next FallbackResolver) FallbackResolver {
	if combos == nil {
		return next
	}
	return &ComboFallbackResolver{combos: combos, next: next}
}

func (r *ComboFallbackResolver) ResolveFallbacks(resolution *core.RequestModelResolution, op core.Operation) []core.ModelSelector {
	if r == nil || resolution == nil {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]core.ModelSelector, 0)
	for _, selector := range r.combos.FallbacksFor(resolution.Requested.Model) {
		key := selector.QualifiedModel()
		if key == resolution.ResolvedQualifiedModel() {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, selector)
	}
	if r.next != nil {
		for _, selector := range r.next.ResolveFallbacks(resolution, op) {
			key := selector.QualifiedModel()
			if key == resolution.ResolvedQualifiedModel() {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, selector)
		}
	}
	return out
}
