package server

import (
	"context"
	"strings"

	"aurora/internal/core"
)

func accountingUserPath(ctx context.Context) string {
	userPath := core.UserPathFromContext(ctx)
	if strings.TrimSpace(userPath) == "" {
		userPath = "/"
	}
	return userPath
}
