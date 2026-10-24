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
type AuthKeyModelAuthorizer struct{}

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
		selector, err := core.ParseModelSelector(model.ID, "")
		if err != nil {
			continue
		}
		// /v1/models lists bare model ids, so the id itself carries no
		// provider. OwnedBy does — fall back to it so a provider-scoped key
		// still sees that provider's models instead of an empty catalogue.
		if selector.Provider == "" {
			selector.Provider = strings.TrimSpace(model.OwnedBy)
		}
		if a.AllowsModel(ctx, selector) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func allowsAuthKeyModel(ctx context.Context, selector core.ModelSelector) bool {
	policy, ok := core.GetAuthKeyAccessPolicy(ctx)
	if !ok || policy.Empty() {
		return true
	}

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
