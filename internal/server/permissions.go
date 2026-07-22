package server

import (
	"net/http"
	"strings"

	"aurora/configuration"
)

type PermissionCheck func(method, path string) (action, resource string, requiresPermission bool)

var adminRoutePermissions = []struct {
	prefix   string
	resource string
}{
	{"/admin/api/v1/dashboard/settings", "admin/settings"},
	{"/admin/api/v1/dashboard", "admin/dashboard"},
	{"/admin/api/v1/cache", "admin/cache"},
	{"/admin/api/v1/usage", "admin/usage"},
	{"/admin/api/v1/audit", "admin/audit"},
	{"/admin/api/v1/console", "admin/audit"},
	{"/admin/api/v1/cli-tools", "admin/cli-tools"},
	{"/admin/api/v1/combos", "admin/models"},
	{"/admin/api/v1/providers", "admin/providers"},
	{"/admin/api/v1/pools", "admin/pools"},
	{"/admin/api/v1/runtime", "admin/settings"},
	{"/admin/api/v1/tenants", "admin/tenants"},
	{"/admin/api/v1/budgets", "admin/budgets"},
	{"/admin/api/v1/models", "admin/models"},
	{"/admin/api/v1/model-overrides", "admin/settings"},
	{"/admin/api/v1/auth-keys", "admin/keys"},
	{"/admin/api/v1/aliases", "admin/keys"},
	{"/admin/api/v1/guardrails", "admin/guardrails"},
	{"/admin/api/v1/workflows", "admin/workflows"},
	{"/admin/api/v1/identity/users", "admin/users"},
	{"/admin/api/v1/identity/roles", "admin/roles"},
	{"/admin/api/v1/identity/permissions", "admin/identity"},
	{"/admin/api/v1/identity/providers", "admin/identity"},
	{"/admin/api/v1/auth", "admin/identity"},
	{"/admin/api/v1/identity", "admin/identity"},
	{"/admin/api/v1/benchmark", "admin/benchmark"},
}

func RequiredCapabilityForRoute(path string) (config.CapabilityKey, bool) {
	return "", false
}

func actionForMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return "read"
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return "write"
	default:
		return "write"
	}
}

func PermissionForRoute(method, path string) (action, resource string, requiresPermission bool) {
	for _, rp := range adminRoutePermissions {
		if pathMatchesPermissionPrefix(path, rp.prefix) {
			return actionForMethod(method), rp.resource, true
		}
	}
	return "", "", false
}

func pathMatchesPermissionPrefix(requestPath, prefix string) bool {
	return requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/")
}
