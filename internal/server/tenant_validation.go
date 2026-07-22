package server

import (
	"context"
	"fmt"
	"strings"

	"aurora/internal/authorization_scope"
)

func validateActiveTenant(ctx context.Context, validator TenantStatusValidator, tenantID string) error {
	if validator == nil {
		return nil
	}

	id := strings.TrimSpace(tenantID)
	if id == "" {
		id = scope.DefaultID
	}

	resolved, err := validator.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("tenant is not active")
	}
	if resolved.Status != scope.StatusActive {
		return fmt.Errorf("tenant is not active")
	}
	return nil
}
