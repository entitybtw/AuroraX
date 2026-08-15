package gateway

import (
	"sync/atomic"

	"aurora/internal/core"
)

// SwappableFallbackResolver forwards fallback resolution to a delegate that can
// be replaced at runtime (e.g. when manual fallback rules change) without
// rebuilding the server stack. The delegate is expected to be immutable once
// constructed; swaps are atomic.
type SwappableFallbackResolver struct {
	current atomic.Pointer[FallbackResolver]
}

// NewSwappableFallbackResolver wraps an initial resolver. Pass nil to start
// with fallback disabled.
func NewSwappableFallbackResolver(initial FallbackResolver) *SwappableFallbackResolver {
	r := &SwappableFallbackResolver{}
	r.swap(initial)
	return r
}

// ResolveFallbacks implements FallbackResolver.
func (r *SwappableFallbackResolver) ResolveFallbacks(resolution *core.RequestModelResolution, op core.Operation) []core.ModelSelector {
	if r == nil {
		return nil
	}
	current := r.current.Load()
	if current == nil || *current == nil {
		return nil
	}
	return (*current).ResolveFallbacks(resolution, op)
}

// Swap atomically replaces the delegate. Pass nil to disable fallback.
func (r *SwappableFallbackResolver) Swap(next FallbackResolver) {
	if r == nil {
		return
	}
	r.swap(next)
}

func (r *SwappableFallbackResolver) swap(next FallbackResolver) {
	var ptr FallbackResolver
	if next != nil {
		ptr = next
	}
	r.current.Store(&ptr)
}