package gateway

import (
	"context"
	"strings"

	"aurora/internal/authorization_scope"
	"aurora/internal/core"
	"aurora/internal/usage"
)

// LogUsage writes one non-streaming usage entry when usage is enabled.
func (o *InferenceOrchestrator) LogUsage(
	ctx context.Context,
	workflow *core.Workflow,
	model, providerType, providerName string,
	extractFn func(*usage.PricingResult) *usage.UsageEntry,
) {
	o.logUsage(ctx, workflow, model, providerType, providerName, extractFn)
}

func (o *InferenceOrchestrator) logUsage(
	ctx context.Context,
	workflow *core.Workflow,
	model, providerType, providerName string,
	extractFn func(*usage.PricingResult) *usage.UsageEntry,
) {
	if o.usageLogger == nil || !o.usageLogger.Config().Enabled || (workflow != nil && !workflow.UsageEnabled()) {
		return
	}
	var pricingResult *usage.PricingResult
	if o.pricingResolver != nil {
		pricingResult = o.pricingResolver.ResolvePricing(model, providerType)
	}
	if entry := extractFn(pricingResult); entry != nil {
		entry.ProviderName = strings.TrimSpace(providerName)
		entry.TenantID = scope.TenantIDFromContext(ctx)
		entry.UserPath = core.UserPathFromContext(ctx)
		o.usageLogger.Write(entry)
	}
}

// ShouldEnforceReturningUsageData reports whether streams should request usage chunks.
func (o *InferenceOrchestrator) ShouldEnforceReturningUsageData() bool {
	if o.usageLogger == nil {
		return false
	}
	cfg := o.usageLogger.Config()
	return cfg.Enabled && cfg.EnforceReturningUsageData
}
