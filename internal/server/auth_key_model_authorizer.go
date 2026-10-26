package server

import (
	"context"
	"net/http"
	"strings"

	"aurora/internal/core"
)

// AuthKeyModelAuthorizer enforces provider/model restrictions attached to the
// authenticated managed auth key. Missing restrictions are intentionally a no-op
// for existing keys; explicit empty allowlists deny all matching resources.
//
// Provider and model lists accept a trailing "*" as a prefix wildcard, so a key
// scoped to "groq" carries "groq/*" in its allowed models and keeps covering
// every model that provider adds later.
type AuthKeyModelAuthorizer struct {
	// ProviderName resolves a model to the concrete configured provider
	// instance name (e.g. "openai-east"). The public model listing only carries
	// the provider *type* in owned_by, so this fills the gap and keeps a key
	// scoped to a named account from being handed an empty model list.
	//
	// Optional: a nil resolver simply skips that extra lookup.
	ProviderName func(model string) string
}

func (AuthKeyModelAuthorizer) ValidateModelAccess(ctx context.Context, selector core.ModelSelector) error {
	if allowsAuthKeyModel(ctx, selector) {
		return nil
	}
	return core.NewInvalidRequestErrorWithStatus(
		http.StatusBadRequest,
		"requested model is not available for this API key",
		nil,
	).WithCode("auth_key_model_access_denied")
}

func (AuthKeyModelAuthorizer) AllowsModel(ctx context.Context, selector core.ModelSelector) bool {
	return allowsAuthKeyModel(ctx, selector)
}

func (a AuthKeyModelAuthorizer) FilterPublicModels(ctx context.Context, models []core.Model) []core.Model {
	policy, ok := core.GetAuthKeyAccessPolicy(ctx)
	if !ok || policy.Empty() || len(models) == 0 {
		return models
	}

	filtered := make([]core.Model, 0, len(models))
	for _, model := range models {
		if a.policyAllowsPublicModel(policy, model) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

// policyAllowsPublicModel answers the allow/deny question for one /v1/models
// entry.
//
// That listing usually carries bare model ids, so the provider is only carried
// by owned_by — and owned_by holds the provider *type*, which is not necessarily
// the configured instance name (an "openai-east" account reports "openai").
// Every provider spelling the entry offers is therefore tried — the one inside
// a qualified id, owned_by, and the concrete instance the gateway routes to —
// and the entry is kept when one of them is allowed. That way a provider-scoped
// key is not handed an empty catalogue over a naming mismatch, while a deny list
// scoped to a provider still hides that provider's models.
//
// The provider-less form is only used when nothing identifies the provider at
// all: with a known provider it would match plain model patterns and quietly
// override a provider-scoped deny.
func (a AuthKeyModelAuthorizer) policyAllowsPublicModel(policy core.AuthKeyAccessPolicy, model core.Model) bool {
	id := strings.TrimSpace(model.ID)
	if id == "" {
		return false
	}

	// A qualified id names its own provider, and the bare model is what the
	// policy patterns are matched against.
	bare := id
	providers := make([]string, 0, 3)
	if parsed, err := core.ParseModelSelector(id, ""); err == nil {
		if parsed.Provider != "" {
			providers = append(providers, parsed.Provider)
			bare = parsed.Model
		}
	}
	if ownedBy := strings.TrimSpace(model.OwnedBy); ownedBy != "" {
		providers = append(providers, ownedBy)
	}
	if a.ProviderName != nil {
		if resolved := strings.TrimSpace(a.ProviderName(id)); resolved != "" {
			providers = append(providers, resolved)
		}
	}

	if len(providers) == 0 {
		return selectorAllows(policy, core.ModelSelector{Model: bare})
	}
	for _, provider := range providers {
		if selectorAllows(policy, core.ModelSelector{Provider: provider, Model: bare}) {
			return true
		}
	}
	return false
}

func allowsAuthKeyModel(ctx context.Context, selector core.ModelSelector) bool {
	policy, ok := core.GetAuthKeyAccessPolicy(ctx)
	if !ok || policy.Empty() {
		return true
	}
	return selectorAllows(policy, selector)
}

// selectorAllows applies a key policy to one concrete selector. Deny wins.
func selectorAllows(policy core.AuthKeyAccessPolicy, selector core.ModelSelector) bool {
	provider := strings.TrimSpace(selector.Provider)
	model := strings.TrimSpace(selector.Model)
	qualified := selector.QualifiedModel()

	if policy.AllowedProviders != nil && !matchesAny(policy.AllowedProviders, provider) {
		return false
	}
	if matchesAny(policy.DeniedModels, model) || matchesAny(policy.DeniedModels, qualified) {
		return false
	}
	if policy.AllowedModels != nil && !matchesAny(policy.AllowedModels, model) && !matchesAny(policy.AllowedModels, qualified) {
		return false
	}
	return true
}

// matchesAny reports whether value matches one of patterns.
//
// Patterns are exact matches, with a trailing "*" turning them into prefix
// matches. "*" therefore matches everything and "groq/*" matches every model
// the groq provider serves — including models the gateway discovers after the
// key was issued. An empty pattern never matches.
func matchesAny(patterns []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
			if strings.HasPrefix(value, prefix) {
				return true
			}
			continue
		}
		if pattern == value {
			return true
		}
	}
	return false
}

type compositeModelAuthorizer struct {
	authorizers []RequestModelAuthorizer
}

// ComposeModelAuthorizers combines request model authorizers with deny-wins semantics.
func ComposeModelAuthorizers(authorizers ...RequestModelAuthorizer) RequestModelAuthorizer {
	filtered := make([]RequestModelAuthorizer, 0, len(authorizers))
	for _, authorizer := range authorizers {
		if authorizer != nil {
			filtered = append(filtered, authorizer)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return compositeModelAuthorizer{authorizers: filtered}
}

func (a compositeModelAuthorizer) ValidateModelAccess(ctx context.Context, selector core.ModelSelector) error {
	for _, authorizer := range a.authorizers {
		if err := authorizer.ValidateModelAccess(ctx, selector); err != nil {
			return err
		}
	}
	return nil
}

func (a compositeModelAuthorizer) AllowsModel(ctx context.Context, selector core.ModelSelector) bool {
	for _, authorizer := range a.authorizers {
		if !authorizer.AllowsModel(ctx, selector) {
			return false
		}
	}
	return true
}

func (a compositeModelAuthorizer) FilterPublicModels(ctx context.Context, models []core.Model) []core.Model {
	filtered := models
	for _, authorizer := range a.authorizers {
		filtered = authorizer.FilterPublicModels(ctx, filtered)
	}
	return filtered
}
