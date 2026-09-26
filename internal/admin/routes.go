package admin

import "github.com/labstack/echo/v5"

// RouteRegistrar is the subset of *echo.Group / *echo.Echo that RegisterRoutes
// uses. Decoupling from a concrete echo type keeps the admin package useful for
// callers that want to mount the API under a different path prefix or wrap the
// routes with extra middleware.
type RouteRegistrar interface {
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
}

// RegisterRoutes mounts the admin REST API on the given route group.
// Callers typically pass an *echo.Group rooted at /admin/api/v1.
func (h *Handler) RegisterRoutes(g RouteRegistrar) {
	g.GET("/auth/me", h.AuthMe)
	g.GET("/dashboard/config", h.DashboardConfig)
	g.GET("/dashboard/features", h.FeatureStatus)
	g.PUT("/dashboard/settings", h.UpdateDashboardSettings)
	g.GET("/cache/overview", h.CacheOverview)
	g.POST("/cache/debug", h.CacheDebug)

	g.GET("/usage/summary", h.UsageSummary)
	g.GET("/usage/daily", h.DailyUsage)
	g.GET("/usage/models", h.UsageByModel)
	g.GET("/usage/user-paths", h.UsageByUserPath)
	g.GET("/usage/log", h.UsageLog)
	g.POST("/usage/recalculate-pricing", h.RecalculateUsagePricing)

	g.GET("/audit/log", h.AuditLog)
	g.GET("/audit/log/stream", h.AuditLogStream)
	g.GET("/audit/log/export", h.AuditLogExport)
	g.GET("/console/recent", h.ConsoleRecent)
	g.GET("/console/stream", h.ConsoleStream)
	g.GET("/audit/conversation", h.AuditConversation)

	g.GET("/providers/status", h.ProviderStatus)
	g.POST("/providers", h.CreateProvider)
	g.GET("/providers/overrides", h.ListProviderOverrides)
	g.GET("/providers/presets", h.ListProviderPresets)
	g.POST("/providers/detect-preset", h.DetectProviderPreset)
	g.PUT("/providers/:name", h.UpdateProvider)
	g.DELETE("/providers/:name", h.DeleteProvider)
	g.GET("/pools", h.ListPools)
	g.GET("/pools/options", h.PoolOptions)
	g.POST("/pools", h.CreatePool)
	g.PUT("/pools/:name", h.UpdatePool)
	g.DELETE("/pools/:name", h.DeletePool)
	g.POST("/runtime/refresh", h.RefreshRuntime)

	g.GET("/models", h.ListModels)
	g.GET("/models/categories", h.ListCategories)

	g.GET("/model-overrides", h.ListModelOverrides)
	g.PUT("/model-overrides/:selector", h.UpsertModelOverride)
	g.DELETE("/model-overrides/:selector", h.DeleteModelOverride)

	g.GET("/model-pricing", h.ListModelPricing)
	g.GET("/model-pricing/export", h.ExportModelPricing)
	g.POST("/model-pricing/import", h.ImportModelPricing)
	g.GET("/model-pricing/backups", h.ListModelPricingBackups)
	g.POST("/model-pricing/backups/:name/restore", h.RestoreModelPricingBackup)
	g.GET("/model-pricing/:selector", h.GetModelPricing)
	g.PUT("/model-pricing/:selector", h.UpsertModelPricing)
	g.DELETE("/model-pricing/:selector", h.DeleteModelPricing)

	g.GET("/auth-keys", h.ListAuthKeys)
	g.POST("/auth-keys", h.CreateAuthKey)
	g.GET("/auth-keys/:id/stats", h.AuthKeyStats)
	g.POST("/auth-keys/:id/deactivate", h.DeactivateAuthKey)

	g.GET("/aliases", h.ListAliases)
	g.PUT("/aliases/:name", h.UpsertAlias)
	g.DELETE("/aliases/:name", h.DeleteAlias)

	g.GET("/cli-tools", h.ListCLITools)
	g.GET("/cli-tools/:tool", h.GetCLITool)
	g.POST("/cli-tools/:tool/preview", h.PreviewCLITool)
	g.POST("/cli-tools/:tool/apply", h.ApplyCLITool)
	g.POST("/cli-tools/:tool/reset", h.ResetCLITool)

	g.GET("/combos", h.ListCombos)
	g.POST("/combos", h.CreateCombo)
	g.GET("/combos/:id", h.GetCombo)
	g.PUT("/combos/:id", h.UpdateCombo)
	g.DELETE("/combos/:id", h.DeleteCombo)
	g.POST("/combos/:id/validate", h.ValidateCombo)

	g.GET("/fallback", h.FallbackConfig)
	g.PUT("/fallback", h.UpdateFallbackConfig)

	g.GET("/guardrails/types", h.ListGuardrailTypes)
	g.GET("/guardrails", h.ListGuardrails)
	g.PUT("/guardrails/:name", h.UpsertGuardrail)
	g.DELETE("/guardrails/:name", h.DeleteGuardrail)

	g.GET("/workflows", h.ListWorkflows)
	g.GET("/workflows/guardrails", h.ListWorkflowGuardrails)
	g.GET("/workflows/:id", h.GetWorkflow)
	g.POST("/workflows", h.CreateWorkflow)
	g.POST("/workflows/:id/deactivate", h.DeactivateWorkflow)

	// Session hub routes are registered separately via sessionhub.RegisterSessionHubRoutes

	g.GET("/sidecar", h.GetSidecarStatus)
	g.PUT("/sidecar", h.UpdateSidecarSettings)

	g.GET("/sidecar/extensions", h.ListExtensions)
	g.GET("/sidecar/extensions/ui", h.ListExtensionUI)
	g.POST("/sidecar/extensions/import", h.ImportExtension)
	g.POST("/sidecar/extensions/reorder", h.ReorderExtensions)
	g.GET("/sidecar/extensions/stores", h.ListExtensionStores)
	g.POST("/sidecar/extensions/stores", h.AddExtensionStore)
	g.DELETE("/sidecar/extensions/stores", h.DeleteExtensionStore)
	g.POST("/sidecar/extensions/store/browse", h.BrowseExtensionStore)
	g.POST("/sidecar/extensions/store/install", h.InstallExtensionFromStore)
	g.GET("/sidecar/extensions/:id", h.GetExtension)
	g.PUT("/sidecar/extensions/:id", h.UpdateExtension)
	g.GET("/sidecar/extensions/:id/export", h.ExportExtension)
	g.DELETE("/sidecar/extensions/:id", h.DeleteExtension)
	g.POST("/sidecar/extensions/:id/apply", h.ApplyExtension)
	g.POST("/sidecar/extensions/:id/unapply", h.UnapplyExtension)
	g.POST("/sidecar/extensions/:id/full-apply", h.FullApplyExtension)
	g.PUT("/sidecar/extensions/:id/config", h.UpdateExtensionConfig)
	g.GET("/sidecar/extensions/:id/check-update", h.CheckExtensionUpdate)
	g.POST("/sidecar/extensions/:id/update", h.UpdateExtensionFromSource)
	g.GET("/addons", h.ListAddons)

	// Temporary aliases for older dashboards.
	g.GET("/sidecar/presets", h.ListExtensions)
	g.POST("/sidecar/presets/import", h.ImportExtension)
	g.GET("/sidecar/presets/:id", h.GetExtension)
	g.GET("/sidecar/presets/:id/export", h.ExportExtension)
	g.DELETE("/sidecar/presets/:id", h.DeleteExtension)
	g.POST("/sidecar/presets/:id/apply", h.ApplyExtension)

}
