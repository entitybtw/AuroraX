package admin

import "strings"

type featureDefinition struct {
	key            string
	label          string
	category       string
	capability     string
	dependencies   []string
	endpoint       string
	offBehavior    string
	recommendation string
}

func buildFeatureStatusSnapshots(cfg DashboardConfigResponse) []FeatureStatusSnapshot {
	runtimeByKey := make(map[string]RuntimeFeatureSnapshot, len(cfg.RuntimeFeatures))
	for _, feature := range cfg.RuntimeFeatures {
		runtimeByKey[feature.Key] = feature
	}

	features := make([]FeatureStatusSnapshot, 0, len(featureDefinitions()))
	for _, def := range featureDefinitions() {
		runtime := runtimeByKey[def.key]
		configured := featureConfigured(def.key, cfg, runtime)
		available := featureAvailable(def, cfg)
		effective := configured && available && dependenciesSatisfied(def, cfg)
		status := featureStatus(configured, available, effective, def, cfg)
		features = append(features, FeatureStatusSnapshot{
			Key:               def.key,
			Label:             def.label,
			Category:          def.category,
			Configured:        configured,
			Available:         available,
			Effective:         effective,
			Status:            status,
			Capability:        def.capability,
			Dependencies:      def.dependencies,
			Endpoint:          firstNonEmpty(def.endpoint, runtime.Endpoint),
			OffBehavior:       def.offBehavior,
			Conflict:          featureConflict(configured, available, def),
			Recommendation:    def.recommendation,
			RuntimeStatus:     runtime.Status,
			RuntimeDependency: runtime.Dependency,
		})
	}
	return features
}

func featureDefinitions() []featureDefinition {
	return []featureDefinition{
		{key: "gateway", label: "OpenAI-compatible gateway", category: "core", endpoint: "/v1/*", offBehavior: "Core gateway should always stay available when providers are configured."},
		{key: "providers", label: "Static providers", category: "core", endpoint: "/admin/api/v1/providers/status", offBehavior: "If no providers are configured, model requests fail with provider/model errors."},
		{key: "pools", label: "Provider pools", category: "routing", endpoint: "/admin/api/v1/pools", offBehavior: "Requests fall back to direct provider/model routing when pools are unconfigured."},
		{key: "models", label: "Model list", category: "core", endpoint: "/admin/api/v1/models", offBehavior: "Explicit model requests can still be attempted even if discovery is degraded."},
		{key: "aliases", label: "Aliases", category: "routing", endpoint: "/admin/api/v1/aliases", offBehavior: "Callers use provider/model IDs directly when aliases are unconfigured."},
		{key: "auth_keys", label: "Managed API keys", category: "auth", endpoint: "/admin/api/v1/auth-keys", offBehavior: "Master-key/admin auth and direct provider auth paths remain usable."},
		{key: "tenants", label: "Tenant management", category: "restricted", capability: "ossRestricted", dependencies: []string{"restricted"}, endpoint: "/admin/api/v1/tenants", offBehavior: "OSS uses the implicit default tenant and tenant routes return 403."},
		{key: "usage", label: "Usage/cost analytics", category: "observability", endpoint: "/admin/api/v1/usage/summary", offBehavior: "Gateway routes normally; usage pages show unavailable or empty data."},
		{key: "audit_logs", label: "Audit logs/live console", category: "observability", endpoint: "/admin/api/v1/audit/log", offBehavior: "Gateway routes normally; audit pages show unavailable or empty data."},
		{key: "exact_cache", label: "Exact response cache", category: "performance", endpoint: "/admin/api/v1/cache/overview", offBehavior: "Gateway bypasses cache and calls providers directly."},
		{key: "semantic_cache", label: "Semantic response cache", category: "performance", endpoint: "/admin/api/v1/cache/overview", offBehavior: "Gateway skips semantic lookup and calls providers directly."},
		{key: "workflows", label: "Config-driven workflows", category: "routing", endpoint: "/admin/api/v1/workflows", offBehavior: "Default provider/model routing applies when workflows are unavailable."},
		{key: "budgets", label: "Budgets", category: "restricted", capability: "ossRestricted", dependencies: []string{"usage"}, endpoint: "/admin/api/v1/budgets", offBehavior: "Requests are never blocked by budgets; usage analytics can remain enabled.", recommendation: "Keep disabled in OSS."},
		{key: "guardrails", label: "Basic local guardrails", category: "safety", endpoint: "/admin/api/v1/guardrails", offBehavior: "Gateway skips guardrail checks and routes normally."},
		{key: "advanced_guardrails", label: "Provider-backed guardrails", category: "restricted", capability: "ossRestricted", dependencies: []string{"guardrails"}, endpoint: "/admin/api/v1/guardrails", offBehavior: "Restricted controls are hidden or denied; local guardrails may still work."},
		{key: "identity_rbac", label: "Identity access controls", category: "restricted", capability: "ossRestricted", endpoint: "/admin/api/v1/identity/users", offBehavior: "Identity routes return 403; master-key admin and managed-key inference continue."},
		{key: "oidc_sso", label: "External SSO", category: "restricted", capability: "ossRestricted", dependencies: []string{"restricted"}, endpoint: "/admin/api/v1/auth/oidc/{provider}", offBehavior: "SSO routes return 403; master-key access remains usable."},
		{key: "rbac", label: "Policy management", category: "restricted", capability: "ossRestricted", dependencies: []string{"restricted"}, endpoint: "/admin/api/v1/identity/permissions", offBehavior: "Policy routes have no effect in OSS."},
		{key: "advanced_routing", label: "Adaptive routing", category: "restricted", capability: "ossRestricted", endpoint: "/admin/api/v1/pools", offBehavior: "Static providers, aliases, pools, and ordinary fallback still route normally."},
		{key: "cluster", label: "Cluster control plane", category: "restricted", capability: "ossRestricted", endpoint: "Settings / Routing", offBehavior: "Single-node and externally orchestrated replicas can still run."},
		{key: "mcp", label: "Tool registry", category: "restricted", capability: "ossRestricted", offBehavior: "No tool-registry routes or panels are exposed."},
		{key: "metrics", label: "Basic metrics", category: "observability", endpoint: "/metrics", offBehavior: "Metrics endpoint is unavailable; gateway routing is unaffected."},
		{key: "observability_exports", label: "Observability exports", category: "restricted", capability: "ossRestricted", dependencies: []string{"metrics"}, endpoint: "Settings / Observability", offBehavior: "Exports are rejected in OSS; basic metrics/logs can still work."},
		{key: "compliance", label: "Compliance controls", category: "restricted", capability: "ossRestricted", dependencies: []string{"audit_logs"}, endpoint: "Settings / Security", offBehavior: "Compliance config is rejected in OSS; basic audit can remain enabled."},
	}
}

func featureConfigured(key string, cfg DashboardConfigResponse, runtime RuntimeFeatureSnapshot) bool {
	switch key {
	case "gateway", "providers", "models", "aliases", "workflows":
		return true
	case "pools":
		return true
	case "auth_keys":
		return true
	case "tenants":
		return capabilityOn(cfg, "identity")
	case "rbac":
		return false
	case "advanced_guardrails":
		return false && flagStringOn(cfg.GuardrailsEnabled)
	case "advanced_routing":
		return false
	case "mcp":
		return false
	}
	if runtime.Key != "" {
		return runtime.Configured
	}
	return false
}

func featureAvailable(def featureDefinition, cfg DashboardConfigResponse) bool {
	if strings.TrimSpace(def.capability) == "" {
		return true
	}
	return capabilityOn(cfg, def.capability)
}

func dependenciesSatisfied(def featureDefinition, cfg DashboardConfigResponse) bool {
	for _, dep := range def.dependencies {
		switch dep {
		case "usage":
			if !flagStringOn(cfg.UsageEnabled) {
				return false
			}
		case "audit_logs":
			if !flagStringOn(cfg.LoggingEnabled) {
				return false
			}
		case "guardrails":
			if !flagStringOn(cfg.GuardrailsEnabled) {
				return false
			}
		case "restricted":
			return false
		case "identity", "identity_rbac":
		case "metrics":
			// Exports can run without Prometheus scraping enabled, but metrics are the OSS fallback.
			continue
		}
	}
	return true
}

func featureStatus(configured, available, effective bool, def featureDefinition, cfg DashboardConfigResponse) string {
	if !available {
		return "edition_restricted"
	}
	if configured && !dependenciesSatisfied(def, cfg) {
		return "dependency_missing"
	}
	if effective {
		return "enabled"
	}
	if configured {
		return "configured"
	}
	return "disabled"
}

func featureConflict(configured, available bool, def featureDefinition) string {
	if configured && !available {
		return "configured but missing required runtime capability"
	}
	return ""
}

func capabilityOn(cfg DashboardConfigResponse, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return true
	}
	if cfg.CapabilityMap != nil && cfg.CapabilityMap[capability] {
		return true
	}
	for _, item := range cfg.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func flagStringOn(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "yes", "enabled":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
