package server

import (
	"context"
	"testing"

	"aurora/internal/core"
)

func TestAuthKeyModelAuthorizerAllowsUnrestrictedKeys(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	selector := core.ModelSelector{Provider: "openai", Model: "gpt-4o"}

	if !authorizer.AllowsModel(context.Background(), selector) {
		t.Fatal("AllowsModel() = false, want true without auth key restrictions")
	}
	if err := authorizer.ValidateModelAccess(context.Background(), selector); err != nil {
		t.Fatalf("ValidateModelAccess() error = %v, want nil", err)
	}
}

func TestAuthKeyModelAuthorizerEnforcesAllowAndDenyLists(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedProviders: []string{"openai-prod"},
		AllowedModels:    []string{"gpt-4o", "openai-prod/gpt-4o-mini"},
		DeniedModels:     []string{"gpt-4o-mini"},
	})

	allowed := core.ModelSelector{Provider: "openai-prod", Model: "gpt-4o"}
	if !authorizer.AllowsModel(ctx, allowed) {
		t.Fatal("AllowsModel() = false, want true for allowed provider/model")
	}

	deniedProvider := core.ModelSelector{Provider: "openai-dev", Model: "gpt-4o"}
	if authorizer.AllowsModel(ctx, deniedProvider) {
		t.Fatal("AllowsModel() = true, want false for non-allowed provider")
	}

	deniedModel := core.ModelSelector{Provider: "openai-prod", Model: "gpt-4o-mini"}
	if authorizer.AllowsModel(ctx, deniedModel) {
		t.Fatal("AllowsModel() = true, want false when denied_models matches")
	}
	if err := authorizer.ValidateModelAccess(ctx, deniedModel); err == nil {
		t.Fatal("ValidateModelAccess() error = nil, want denial")
	}
}

func TestAuthKeyModelAuthorizerEmptyAllowlistDeniesAll(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedModels: []string{},
	})

	if authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "openai", Model: "gpt-4o"}) {
		t.Fatal("AllowsModel() = true, want false for explicit empty allowed_models")
	}
}

func TestAuthKeyModelAuthorizerProviderScopedModelPatterns(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedModels: []string{"groq/*"},
	})

	if !authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "llama-4-scout-17b-16e-instruct"}) {
		t.Fatal("AllowsModel() = false, want true for a model covered by groq/*")
	}

	// A model the registry had not listed yet when the key was issued is
	// covered too — the pattern is resolved per request, not expanded once.
	if !authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "some-model-discovered-later"}) {
		t.Fatal("AllowsModel() = false, want true for a model added after the key was issued")
	}

	// Model ids that carry their own provider prefix still match on the
	// leading provider segment.
	if !authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "openai/gpt-oss-120b"}) {
		t.Fatal("AllowsModel() = false, want true for groq serving a prefixed model id")
	}

	if authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "openai", Model: "gpt-4o"}) {
		t.Fatal("AllowsModel() = true, want false for a model outside groq/*")
	}
}

func TestAuthKeyModelAuthorizerProviderPrefixWildcards(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedProviders: []string{"openai*"},
	})

	for _, provider := range []string{"openai", "openai-east"} {
		if !authorizer.AllowsModel(ctx, core.ModelSelector{Provider: provider, Model: "gpt-4o"}) {
			t.Fatalf("AllowsModel() = false, want true for provider %q covered by openai*", provider)
		}
	}

	if authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "gpt-4o"}) {
		t.Fatal("AllowsModel() = true, want false for a provider outside the openai* prefix")
	}
}

func TestAuthKeyModelAuthorizerDeniedModelWildcardsWin(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedProviders: []string{"groq"},
		AllowedModels:    []string{"*"},
		DeniedModels:     []string{"groq/llama-4-preview*"},
	})

	if !authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "llama-4-scout-17b-16e-instruct"}) {
		t.Fatal("AllowsModel() = false, want true for a stable groq model")
	}
	if authorizer.AllowsModel(ctx, core.ModelSelector{Provider: "groq", Model: "llama-4-preview-instruct"}) {
		t.Fatal("AllowsModel() = true, want false when the denied pattern wildcard matches")
	}
}

func TestAuthKeyModelAuthorizerFilterPublicModelsKeepsProviderScopedCatalogue(t *testing.T) {
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedProviders: []string{"groq"},
		AllowedModels:    []string{"groq/*"},
	})

	// /v1/models lists bare ids, so the provider only survives in OwnedBy.
	models := []core.Model{
		{ID: "llama-4-scout-17b-16e-instruct", Object: "model", OwnedBy: "groq"},
		{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
		{ID: "groq/llama-4-scout-17b-16e-instruct", Object: "model", OwnedBy: "openrouter"},
	}

	filtered := authorizer.FilterPublicModels(ctx, models)
	if len(filtered) != 2 {
		t.Fatalf("FilterPublicModels() len = %d, want 2", len(filtered))
	}
	if filtered[0].ID != "llama-4-scout-17b-16e-instruct" || filtered[1].ID != "groq/llama-4-scout-17b-16e-instruct" {
		t.Fatalf("FilterPublicModels() = %#v, want the two groq-backed models", filtered)
	}
}

func TestAuthKeyModelAuthorizerFilterPublicModelsResolvesNamedAccounts(t *testing.T) {
	// A key scoped to the "openai-east" account. /v1/models reports the
	// provider *type* ("openai") in owned_by, so the concrete instance name has
	// to come from the router. Without it the listing would come back empty.
	authorizer := AuthKeyModelAuthorizer{
		ProviderName: func(model string) string {
			if model == "gpt-4o" {
				return "openai-east"
			}
			return ""
		},
	}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		AllowedProviders: []string{"openai-east"},
		AllowedModels:    []string{"openai-east/*"},
	})

	models := []core.Model{
		{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
		{ID: "claude-sonnet", Object: "model", OwnedBy: "anthropic"},
	}

	filtered := authorizer.FilterPublicModels(ctx, models)
	if len(filtered) != 1 || filtered[0].ID != "gpt-4o" {
		t.Fatalf("FilterPublicModels() = %#v, want only gpt-4o", filtered)
	}
}

func TestAuthKeyModelAuthorizerFilterPublicModelsHonoursProviderScopedDeny(t *testing.T) {
	// A provider-scoped deny has to hide that provider's models. The fallback
	// form used when the provider is unknown must not win over it.
	authorizer := AuthKeyModelAuthorizer{}
	ctx := core.WithAuthKeyAccessPolicy(context.Background(), core.AuthKeyAccessPolicy{
		DeniedModels: []string{"openai/*"},
	})

	models := []core.Model{
		{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
		{ID: "claude-sonnet", Object: "model", OwnedBy: "anthropic"},
	}

	filtered := authorizer.FilterPublicModels(ctx, models)
	if len(filtered) != 1 || filtered[0].ID != "claude-sonnet" {
		t.Fatalf("FilterPublicModels() = %#v, want only claude-sonnet", filtered)
	}
}
