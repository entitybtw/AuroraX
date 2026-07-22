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
