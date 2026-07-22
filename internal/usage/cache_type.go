package usage

import (
	"strings"

	"aurora/internal/authorization_scope"
	"aurora/internal/core"
)

const legacyTenantPathPrefix = "/_tenants/"

const (
	CacheTypeExact    = "exact"
	CacheTypeSemantic = "semantic"

	CacheModeUncached = "uncached"
	CacheModeCached   = "cached"
	CacheModeAll      = "all"
)

func normalizeCacheType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CacheTypeExact:
		return CacheTypeExact
	case CacheTypeSemantic:
		return CacheTypeSemantic
	default:
		return ""
	}
}

func normalizeCacheMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CacheModeCached:
		return CacheModeCached
	case CacheModeAll:
		return CacheModeAll
	default:
		return CacheModeUncached
	}
}

func cacheTypeValue(value string) any {
	if normalized := normalizeCacheType(value); normalized != "" {
		return normalized
	}
	return nil
}

func tenantIDValue(value string) any {
	if normalized := strings.TrimSpace(value); normalized != "" {
		return normalized
	}
	return nil
}

func normalizedUsageEntryForStorage(entry *UsageEntry) *UsageEntry {
	if entry == nil {
		return nil
	}

	normalized := normalizeCacheType(entry.CacheType)
	providerName := strings.TrimSpace(entry.ProviderName)
	costSource := strings.TrimSpace(entry.CostSource)
	tenantID := strings.TrimSpace(entry.TenantID)
	userPath := normalizeUsageEntryUserPath(entry.UserPath)
	if legacyTenantID, legacyUserPath, ok := splitLegacyTenantUserPath(userPath); ok {
		if tenantID == "" || tenantID == scope.DefaultID {
			tenantID = legacyTenantID
		}
		userPath = legacyUserPath
	}
	if normalized == entry.CacheType && providerName == entry.ProviderName && costSource == entry.CostSource && tenantID == entry.TenantID && userPath == entry.UserPath {
		return entry
	}

	cloned := *entry
	cloned.CacheType = normalized
	cloned.ProviderName = providerName
	cloned.CostSource = costSource
	cloned.TenantID = tenantID
	cloned.UserPath = userPath
	return &cloned
}

func normalizeUsageEntryUserPath(value string) string {
	normalized, err := core.NormalizeUserPath(value)
	if err != nil || normalized == "" {
		return "/"
	}
	return normalized
}

func splitLegacyTenantUserPath(userPath string) (string, string, bool) {
	if !strings.HasPrefix(userPath, legacyTenantPathPrefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(userPath, legacyTenantPathPrefix)
	if remainder == "" {
		return "", "", false
	}
	parts := strings.SplitN(remainder, "/", 2)
	tenantID := strings.TrimSpace(parts[0])
	if tenantID == "" {
		return "", "", false
	}
	if len(parts) == 1 || parts[1] == "" {
		return tenantID, "/", true
	}
	return tenantID, "/" + parts[1], true
}
