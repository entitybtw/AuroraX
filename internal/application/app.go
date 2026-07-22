// Package app provides the main application struct for centralized dependency management
// and lifecycle control of the aurora server.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurora/configuration"
	"aurora/internal/admin"
	"aurora/internal/admin/dashboard"
	"aurora/internal/audit_logging"
	"aurora/internal/authentication_keys"
	"aurora/internal/batch"
	"aurora/internal/command_line_tools"
	"aurora/internal/console"
	"aurora/internal/core"
	"aurora/internal/failover"
	"aurora/internal/gateway"
	"aurora/internal/guardrails"
	httpclient "aurora/internal/http_client"
	"aurora/internal/model_aliases"
	"aurora/internal/model_combinations"
	"aurora/internal/model_overrides"

	"aurora/internal/providers"
	"aurora/internal/providers/pool"
	"aurora/internal/response_cache"
	"aurora/internal/server"
	"aurora/internal/storage"
	"aurora/internal/telemetry"
	"aurora/internal/usage"
	"aurora/internal/version"
	"aurora/internal/workflow"
)

// App represents the main application with all its dependencies.
// It provides centralized lifecycle management for all components.
type App struct {
	config            *config.Config
	rawProviders      map[string]config.RawProviderConfig
	rawPools          map[string]config.RawPoolConfig
	providerOverrides *admin.ProviderOverrideStore
	poolOverrides     *admin.PoolOverrideStore
	providers         *providers.InitResult
	audit             *auditlog.Result
	usage             *usage.Result
	batch             *batch.Result
	aliases           *aliases.Result
	combos            *combos.Result
	modelOverrides    *modeloverrides.Result
	authKeys          *authkeys.Result
	guardrails        *guardrails.Result
	workflows         *workflow.Result
	server            *server.Server

	poolCountersPath string

	shutdownMu  sync.Mutex
	shutdown    bool
	serverMu    sync.Mutex
	serverStop  context.CancelFunc
	serverDone  chan error
	refreshCh   chan struct{}
	refreshOnce sync.Once

	authKeyRateLimiterCloser io.Closer
}

// Config holds the configuration options for creating an App.
type Config struct {
	// AppConfig holds the loaded application configuration and raw provider data
	// produced by config.Load.
	AppConfig *config.LoadResult

	// Factory provides the ProviderFactory used to construct provider instances.
	Factory *providers.ProviderFactory
}

// New creates a new App with all dependencies initialized.
// The caller must call Shutdown to release resources.
func New(ctx context.Context, cfg Config) (*App, error) {
	if cfg.AppConfig == nil {
		return nil, fmt.Errorf("app config is required")
	}

	if cfg.AppConfig.Config == nil {
		return nil, fmt.Errorf("app config contains nil Config")
	}

	if cfg.Factory == nil {
		return nil, fmt.Errorf("factory is required")
	}

	appCfg := cfg.AppConfig.Config
	applyHTTPClientConfig(appCfg.HTTP)
	app := &App{
		config:            appCfg,
		rawProviders:      cloneRawProviderConfigs(cfg.AppConfig.RawProviders),
		rawPools:          cloneRawPoolConfigs(cfg.AppConfig.RawPools),
		providerOverrides: admin.NewProviderOverrideStore(),
		poolOverrides:     admin.NewPoolOverrideStore(),
	}

	providerResult, err := providers.Init(ctx, cfg.AppConfig, cfg.Factory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}
	app.providers = providerResult

	app.poolCountersPath = poolCountersFilePath(appCfg.Storage)
	if providerResult.Pools != nil {
		if loadErr := providerResult.Pools.LoadCounters(app.poolCountersPath); loadErr != nil {
			slog.Warn("could not restore pool counters", "error", loadErr)
		}
	}

	// Initialize audit logging
	auditResult, err := auditlog.New(ctx, appCfg)
	if err != nil {
		closeErr := app.providers.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize audit logging: %w (also: providers close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize audit logging: %w", err)
	}
	app.audit = auditResult

	// In bench mode, force usage tracking off for maximum throughput.
	// Usage tracking adds per-request overhead even when the storage layer is
	// buffered — at 5000 req/s the SQLite write pressure competes with request
	// processing. Benchmarks prove this single toggle recovers ~4000 req/s.
	if appCfg.Server.MinimalBenchMode {
		appCfg.Usage.Enabled = false
	}

	// Initialize usage tracking
	// Use shared storage if both audit logging and usage tracking use the same backend
	var usageResult *usage.Result
	if auditResult.Storage != nil && appCfg.Usage.Enabled {
		// Share storage connection with audit logging
		usageResult, err = usage.NewWithSharedStorage(ctx, appCfg, auditResult.Storage)
	} else {
		// Create separate storage or return noop logger
		usageResult, err = usage.New(ctx, appCfg)
	}
	if err != nil {
		closeErr := errors.Join(app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize usage tracking: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize usage tracking: %w", err)
	}
	if usageResult == nil || usageResult.Logger == nil {
		var usageCloseErr error
		if usageResult != nil {
			usageCloseErr = usageResult.Close()
		}
		closeErr := errors.Join(usageCloseErr, app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("usage tracking initialization returned nil result (also: close error: %v)", closeErr)
		}
		return nil, fmt.Errorf("usage tracking initialization returned nil result")
	}
	app.usage = usageResult

	// Initialize batch lifecycle storage.
	var batchResult *batch.Result
	if auditResult.Storage != nil {
		batchResult, err = batch.NewWithSharedStorage(ctx, auditResult.Storage)
	} else if usageResult.Storage != nil {
		batchResult, err = batch.NewWithSharedStorage(ctx, usageResult.Storage)
	} else {
		batchResult, err = batch.New(ctx, appCfg)
	}
	if err != nil {
		closeErr := errors.Join(app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize batch storage: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize batch storage: %w", err)
	}
	app.batch = batchResult

	// Initialize aliases using shared storage when already available.
	var aliasResult *aliases.Result
	if auditResult.Storage != nil {
		aliasResult, err = aliases.NewWithSharedStorage(ctx, appCfg, auditResult.Storage, providerResult.Registry)
	} else if usageResult.Storage != nil {
		aliasResult, err = aliases.NewWithSharedStorage(ctx, appCfg, usageResult.Storage, providerResult.Registry)
	} else if batchResult.Storage != nil {
		aliasResult, err = aliases.NewWithSharedStorage(ctx, appCfg, batchResult.Storage, providerResult.Registry)
	} else {
		aliasResult, err = aliases.New(ctx, appCfg, providerResult.Registry)
	}
	if err != nil {
		closeErr := errors.Join(app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize aliases: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize aliases: %w", err)
	}
	app.aliases = aliasResult

	var comboResult *combos.Result
	if appCfg.Combos.Enabled {
		staticCombos := combos.CombosFromConfig(appCfg.Combos.Definitions)
		sharedComboStorage := firstSharedStorage(auditResult.Storage, usageResult.Storage, batchResult.Storage, aliasResult.Storage)
		if sharedComboStorage != nil {
			comboResult, err = combos.NewWithSharedStorage(ctx, appCfg, sharedComboStorage, providerResult.Registry, app.aliases.Service, staticCombos)
		} else {
			comboResult, err = combos.New(ctx, appCfg, providerResult.Registry, app.aliases.Service, staticCombos)
		}
		if err != nil {
			closeErr := errors.Join(app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
			if closeErr != nil {
				return nil, fmt.Errorf("failed to initialize combos: %w (also: close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to initialize combos: %w", err)
		}
	} else {
		comboResult = &combos.Result{}
		slog.Info("combos disabled")
	}
	app.combos = comboResult

	var modelOverrideResult *modeloverrides.Result
	if appCfg.Models.OverridesEnabled {
		sharedModelOverrideStorage := firstSharedStorage(auditResult.Storage, usageResult.Storage, batchResult.Storage, aliasResult.Storage)
		if sharedModelOverrideStorage != nil {
			modelOverrideResult, err = modeloverrides.NewWithSharedStorage(ctx, appCfg, sharedModelOverrideStorage, providerResult.Registry)
		} else {
			modelOverrideResult, err = modeloverrides.New(ctx, appCfg, providerResult.Registry)
		}
		if err != nil {
			closeErr := errors.Join(app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
			if closeErr != nil {
				return nil, fmt.Errorf("failed to initialize model overrides: %w (also: close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to initialize model overrides: %w", err)
		}
	} else {
		modelOverrideResult = &modeloverrides.Result{}
		slog.Info("model overrides disabled")
	}
	app.modelOverrides = modelOverrideResult

	refreshInterval := workflowRefreshInterval(appCfg)
	var guardrailExecutor guardrails.ChatCompletionExecutor = app.providers.Router
	if app.aliases != nil && app.aliases.Service != nil {
		guardrailExecutor = aliases.NewProviderWithOptions(app.providers.Router, app.aliases.Service, aliases.Options{})
	}

	// Initialize reusable guardrail definitions using shared storage when already available.
	var guardrailResult *guardrails.Result
	sharedGuardrailStorage := firstSharedStorage(auditResult.Storage, usageResult.Storage, batchResult.Storage, aliasResult.Storage, app.combos.Storage, modelOverrideResult.Storage)
	if sharedGuardrailStorage != nil {
		guardrailResult, err = guardrails.NewWithSharedStorage(ctx, sharedGuardrailStorage, appCfg, refreshInterval, guardrailExecutor)
	} else {
		guardrailResult, err = guardrails.New(ctx, appCfg, refreshInterval, guardrailExecutor)
	}
	if err != nil {
		closeErr := errors.Join(app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize guardrails: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize guardrails: %w", err)
	}
	app.guardrails = guardrailResult

	seedGuardrails, err := configGuardrailDefinitions(appCfg.Guardrails)
	if err != nil {
		closeErr := errors.Join(app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to prepare guardrail definitions: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to prepare guardrail definitions: %w", err)
	}
	if err := guardrailResult.Service.UpsertDefinitions(ctx, seedGuardrails); err != nil {
		closeErr := errors.Join(app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to upsert guardrails: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to upsert guardrails: %w", err)
	}

	// Build runtime execution dependencies. Policy is passed explicitly into the
	// server; the live provider dependency remains the bare router.
	var provider core.RoutableProvider = app.providers.Router
	var translatedRequestPatcher server.TranslatedRequestPatcher
	var batchRequestPreparers []server.BatchRequestPreparer
	featureCaps := runtimeWorkflowFeatureCaps(appCfg)

	var workflowResult *workflow.Result
	sharedWorkflowStorage := firstSharedStorage(auditResult.Storage, usageResult.Storage, batchResult.Storage, aliasResult.Storage, app.combos.Storage, modelOverrideResult.Storage, guardrailResult.Storage)
	workflowCompiler := workflow.NewCompilerWithFeatureCaps(guardrailResult.Service, featureCaps)
	if sharedWorkflowStorage != nil {
		workflowResult, err = workflow.NewWithSharedStorage(ctx, sharedWorkflowStorage, workflowCompiler, refreshInterval)
	} else {
		workflowResult, err = workflow.New(ctx, appCfg, workflowCompiler, refreshInterval)
	}
	if err != nil {
		closeErr := errors.Join(app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize workflows: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize workflows: %w", err)
	}
	defaultWorkflow := defaultWorkflowInput(appCfg, guardrailResult.Service.Names(), seedGuardrails)
	if err := workflowResult.Service.EnsureDefaultGlobal(ctx, defaultWorkflow); err != nil {
		closeErr := errors.Join(workflowResult.Close(), app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to seed workflows: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to seed workflows: %w", err)
	}
	if err := workflowResult.Service.Refresh(ctx); err != nil {
		closeErr := errors.Join(workflowResult.Close(), app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to load workflows: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to load workflows: %w", err)
	}
	app.workflows = workflowResult

	var authKeyResult *authkeys.Result
	sharedAuthKeyStorage := firstSharedStorage(
		auditResult.Storage,
		usageResult.Storage,
		batchResult.Storage,
		aliasResult.Storage,
		modelOverrideResult.Storage,
		guardrailResult.Storage,
		workflowResult.Storage,
	)
	if sharedAuthKeyStorage != nil {
		authKeyResult, err = authkeys.NewWithSharedStorage(ctx, sharedAuthKeyStorage)
	} else {
		authKeyResult, err = authkeys.New(ctx, appCfg)
	}
	if err != nil {
		closeErr := errors.Join(workflowResult.Close(), app.guardrails.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize auth keys: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize auth keys: %w", err)
	}
	app.authKeys = authKeyResult
	// Log configuration status after auth has been initialized so the startup
	// message reflects both bootstrap and managed auth modes.
	app.logStartupInfo()

	if featureCaps.Guardrails {
		if app.guardrails != nil && app.guardrails.Service != nil {
			translatedRequestPatcher = guardrails.NewWorkflowRequestPatcher(workflowResult.Service)
			if appCfg.Guardrails.EnableForBatchProcessing {
				batchRequestPreparers = append(batchRequestPreparers, guardrails.NewWorkflowBatchPreparer(provider, workflowResult.Service))
			}
			slog.Info(
				"guardrails enabled",
				"count", app.guardrails.Service.Len(),
				"enable_for_batch_processing", appCfg.Guardrails.EnableForBatchProcessing,
			)
		}
	}
	if app.aliases != nil && app.aliases.Service != nil {
		batchRequestPreparers = append([]server.BatchRequestPreparer{
			aliases.NewBatchPreparer(provider, app.aliases.Service),
		}, batchRequestPreparers...)
	}
	if app.modelOverrides != nil && app.modelOverrides.Service != nil {
		batchRequestPreparers = append(batchRequestPreparers, modeloverrides.NewBatchPreparer(provider, app.modelOverrides.Service))
	}
	batchRequestPreparer := server.ComposeBatchRequestPreparers(providerAsNativeFileRouter(provider), batchRequestPreparers...)

	// Create server
	allowPassthroughV1Alias := appCfg.Server.AllowPassthroughV1Alias
	swaggerEnabled := appCfg.Server.SwaggerEnabled && server.SwaggerAvailable()
	if appCfg.Server.SwaggerEnabled && !server.SwaggerAvailable() {
		slog.Warn("swagger UI requested but not available in this build",
			"recommendation", "rebuild with -tags=swagger")
	}

	modelAuthorizer := server.ComposeModelAuthorizers(server.AuthKeyModelAuthorizer{}, app.modelOverrides.Service)

	// Build the shared auth-key rate limiter so the admin /auth-keys/:id/stats
	// endpoint can read live consumption from the same counters used to enforce
	// limits in the request path.
	type authKeyRateLimiterBundle interface {
		server.AuthKeyRateLimiter
		server.AuthKeyRateLimitInspector
	}
	var authKeyRateLimiter authKeyRateLimiterBundle = server.NewInMemoryAuthKeyRateLimiter()
	if appCfg.Cache.Model.Redis != nil && appCfg.Cache.Model.Redis.URL != "" {
		rl, err := server.NewRedisAuthKeyRateLimiterFromURL(appCfg.Cache.Model.Redis.URL)
		if err != nil {
			slog.Warn("failed to create Redis rate limiter, falling back to in-memory", "error", err)
		} else {
			authKeyRateLimiter = rl
			app.authKeyRateLimiterCloser = rl
			slog.Info("rate limiter backend: redis")
		}
	} else {
		slog.Debug("rate limiter backend: in-memory")
	}

	requestUsageLogger := usageResult.Logger
	disableRequestBodySnapshot := appCfg.Server.DisableRequestBodySnapshot
	disablePassthroughSemanticEnrichment := appCfg.Server.DisablePassthroughSemanticEnrichment

	serverCfg := &server.Config{
		BasePath:        appCfg.Server.BasePath,
		MasterKey:       appCfg.Server.MasterKey,
		Authenticator:   authKeyResult.Service,
		MetricsEnabled:  appCfg.Metrics.Enabled,
		MetricsEndpoint: appCfg.Metrics.Endpoint,
		BodySizeLimit:   appCfg.Server.BodySizeLimit,
		PprofEnabled:    appCfg.Server.PprofEnabled,
		AuditLogger:     auditResult.Logger,
		UsageLogger:     requestUsageLogger,

		AuthKeyRateLimiter:                   authKeyRateLimiter,
		PricingResolver:                      providerResult.Registry,
		ModelResolver:                        requestModelResolver(app.aliases.Service, app.combos.Service),
		ModelAuthorizer:                      modelAuthorizer,
		FallbackResolver:                     newComboFallbackResolver(app.combos, failover.NewResolver(appCfg.Fallback, providerResult.Registry)),
		WorkflowPolicyResolver:               workflowResult.Service,
		TranslatedRequestPatcher:             translatedRequestPatcher,
		BatchRequestPreparer:                 batchRequestPreparer,
		ExposedModelLister:                   exposedModelLister(app.aliases.Service, app.combos.Service),
		KeepOnlyAliasesAtModelsEndpoint:      appCfg.Models.KeepOnlyAliasesAtModelsEndpoint,
		PassthroughSemanticEnrichers:         cfg.Factory.PassthroughSemanticEnrichers(),
		BatchStore:                           batchResult.Store,
		LogOnlyModelInteractions:             appCfg.Logging.OnlyModelInteractions,
		DisableRequestLogging:                appCfg.Server.DisableRequestLogging || appCfg.Server.MinimalBenchMode,
		DisableRequestBodySnapshot:           disableRequestBodySnapshot,
		DisablePassthroughSemanticEnrichment: disablePassthroughSemanticEnrichment,
		DisablePassthroughRoutes:             !appCfg.Server.EnablePassthroughRoutes,
		EnabledPassthroughProviders:          appCfg.Server.EnabledPassthroughProviders,
		AllowPassthroughV1Alias:              &allowPassthroughV1Alias,
		SwaggerEnabled:                       swaggerEnabled,
		EnableAnthropicIngress:               appCfg.Server.EnableAnthropicIngress,
		TokenSaver:                           appCfg.TokenSaver,
		PromptCacheConfig:                    appCfg.Cache.Prompt,
		Capabilities:                         config.ResolveCapabilities(appCfg.Edition),
	}

	// Initialize admin API and dashboard (behind separate feature flags)
	adminCfg := appCfg.Admin
	if !adminCfg.EndpointsEnabled && adminCfg.UIEnabled {
		slog.Warn("ADMIN_UI_ENABLED=true requires ADMIN_ENDPOINTS_ENABLED=true â€” forcing UI to disabled")
		adminCfg.UIEnabled = false
	}
	usageEnabledForDashboard := usageResult.Logger.Config().Enabled
	var adminHandler *admin.Handler
	if adminCfg.EndpointsEnabled {
		var dashHandler *dashboard.Handler
		var adminErr error
		adminHandler, dashHandler, adminErr = initAdmin(
			auditResult.Logger,
			auditResult.Storage,
			usageResult.Storage,
			providerResult.Registry,
			appCfg.Server.MasterKey,
			providerResult.ConfiguredProviders,
			providerResult.Pools,
			authKeyResult.Service,
			authKeyRateLimiter,
			app.aliases.Service,
			app.combos.Service,
			cliToolsService(appCfg),
			app.modelOverrides.Service,
			workflowResult.Service,
			app.guardrails.Service,
			app.providerOverrides,
			app.poolOverrides,
			app,
			app,
			dashboardRuntimeConfig(appCfg, usageEnabledForDashboard),
			usagePricingRecalculationConfigured(appCfg),
			appCfg.Server.BasePath,
			adminCfg.UIEnabled,
		)
		if adminErr != nil {
			slog.Warn("failed to initialize admin", "error", adminErr)
		} else {
			serverCfg.AdminEndpointsEnabled = true
			serverCfg.AdminHandler = adminHandler
			slog.Info("admin API enabled", "api", config.JoinBasePath(appCfg.Server.BasePath, "/admin/api/v1"))
			if adminCfg.UIEnabled {
				serverCfg.AdminUIEnabled = true
				serverCfg.DashboardHandler = dashHandler
				slog.Info("admin UI enabled",
					"url", fmt.Sprintf("http://localhost:%s%s", appCfg.Server.Port, config.JoinBasePath(appCfg.Server.BasePath, "/admin/dashboard")))
			}
		}
	} else {
		slog.Info("admin API disabled")
	}

	if swaggerEnabled {
		slog.Info("swagger UI enabled", "path", config.JoinBasePath(appCfg.Server.BasePath, "/swagger/index.html"))
	}
	if appCfg.Server.PprofEnabled {
		slog.Debug("pprof enabled", "path", config.JoinBasePath(appCfg.Server.BasePath, "/debug/pprof/"))
	}
	if appCfg.Server.EnablePassthroughRoutes {
		slog.Debug("provider passthrough enabled", "path", config.JoinBasePath(appCfg.Server.BasePath, "/p/{provider}/{endpoint}"))
	} else {
		slog.Debug("provider passthrough disabled")
	}

	rcm, err := responsecache.NewResponseCacheMiddleware(appCfg.Cache.Response, providerResult.CredentialResolvedProviders, usageResult.Logger, providerResult.Registry)
	if err != nil {
		var (
			workflowsCloseErr      error
			guardrailsCloseErr     error
			authKeysCloseErr       error
			comboCloseErr          error
			aliasCloseErr          error
			modelOverridesCloseErr error
			batchCloseErr          error
		)
		if app.workflows != nil {
			workflowsCloseErr = app.workflows.Close()
		}
		if app.guardrails != nil {
			guardrailsCloseErr = app.guardrails.Close()
		}
		if app.authKeys != nil {
			authKeysCloseErr = app.authKeys.Close()
		}
		if app.combos != nil {
			comboCloseErr = app.combos.Close()
		}
		if app.aliases != nil {
			aliasCloseErr = app.aliases.Close()
		}
		if app.modelOverrides != nil {
			modelOverridesCloseErr = app.modelOverrides.Close()
		}
		if app.batch != nil {
			batchCloseErr = app.batch.Close()
		}
		closeErr := errors.Join(workflowsCloseErr, guardrailsCloseErr, authKeysCloseErr, comboCloseErr, aliasCloseErr, modelOverridesCloseErr, batchCloseErr, app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize response cache: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize response cache: %w", err)
	}
	serverCfg.ResponseCacheMiddleware = rcm
	if adminHandler != nil {
		adminHandler.SetResponseCache(rcm)
	}

	internalGuardrailExecutor := server.NewInternalChatCompletionExecutor(provider, server.InternalChatCompletionExecutorConfig{
		ModelResolver:          app.aliases.Service,
		ModelAuthorizer:        modelAuthorizer,
		WorkflowPolicyResolver: workflowResult.Service,
		FallbackResolver:       serverCfg.FallbackResolver,
		AuditLogger:            auditResult.Logger,
		UsageLogger:            usageResult.Logger,
		PricingResolver:        providerResult.Registry,
		ResponseCache:          rcm,
		PromptCacheConfig: &core.PromptCacheConfig{
			Mode:                 core.PromptCacheMode(appCfg.Cache.Prompt.Mode),
			SystemPromptCache:    appCfg.Cache.Prompt.SystemPromptCache,
			FirstMessageCache:    appCfg.Cache.Prompt.FirstMessageCache,
			ToolsCache:           appCfg.Cache.Prompt.ToolsCache,
			MinTokensBeforeCache: appCfg.Cache.Prompt.MinTokensBeforeCache,
		},
	})
	if err := guardrailResult.Service.SetExecutor(ctx, internalGuardrailExecutor); err != nil {
		closeErr := errors.Join(rcm.Close(), app.workflows.Close(), app.guardrails.Close(), app.authKeys.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to wire internal guardrail executor: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to wire internal guardrail executor: %w", err)
	}
	if err := workflowResult.Service.Refresh(ctx); err != nil {
		closeErr := errors.Join(rcm.Close(), app.workflows.Close(), app.guardrails.Close(), app.authKeys.Close(), app.modelOverrides.Close(), app.combos.Close(), app.aliases.Close(), app.batch.Close(), app.usage.Close(), app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to refresh workflows after wiring internal guardrail executor: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to refresh workflows after wiring internal guardrail executor: %w", err)
	}

	app.server = server.New(provider, serverCfg)

	return app, nil
}

// Router returns the core.RoutableProvider for request routing.
func (a *App) Router() core.RoutableProvider {
	if a.providers == nil {
		return nil
	}
	return a.providers.Router
}

// AuditLogger returns the audit logger interface.
func (a *App) AuditLogger() auditlog.LoggerInterface {
	if a.audit == nil {
		return nil
	}
	return a.audit.Logger
}

// UsageLogger returns the usage logger interface.
func (a *App) UsageLogger() usage.LoggerInterface {
	if a.usage == nil {
		return nil
	}
	return a.usage.Logger
}

func providerAsNativeFileRouter(provider core.RoutableProvider) core.NativeFileRoutableProvider {
	if fileRouter, ok := provider.(core.NativeFileRoutableProvider); ok {
		return fileRouter
	}
	return nil
}

// Start starts the HTTP server on the given address.
// This is a blocking call that returns when the server stops.
func (a *App) Start(ctx context.Context, addr string) error {
	return a.startServer(ctx, addr, func(serverCtx context.Context) error {
		return a.server.Start(serverCtx, addr)
	})
}

// StartWithListener starts the HTTP server on a pre-bound listener.
// This is primarily useful for tests that need to reserve a loopback port
// before handing control to the server.
func (a *App) StartWithListener(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return fmt.Errorf("listener is required")
	}
	return a.startServer(ctx, listener.Addr().String(), func(serverCtx context.Context) error {
		return a.server.StartWithListener(serverCtx, listener)
	})
}

func (a *App) startServer(ctx context.Context, address string, start func(context.Context) error) error {
	if a.server == nil {
		return fmt.Errorf("server is not initialized")
	}

	a.serverMu.Lock()
	if a.serverDone != nil {
		a.serverMu.Unlock()
		return fmt.Errorf("server is already running")
	}
	serverCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	a.serverStop = cancel
	a.serverDone = done
	a.serverMu.Unlock()

	slog.Info("starting server", "address", address)
	telemetry.InitHeartbeat(serverCtx, version.Version)
	err := start(serverCtx)

	a.serverMu.Lock()
	if a.serverDone == done {
		done <- err
		close(done)
		a.serverDone = nil
		a.serverStop = nil
	}
	a.serverMu.Unlock()

	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("server stopped gracefully")
			return nil
		}
		return fmt.Errorf("server failed to start: %w", err)
	}
	return nil
}

// Shutdown gracefully tears down app components in dependency order.
// Order:
//  1. Cancel HTTP server context and wait for the server to stop.
//  2. Provider subsystem close (stops model refresh loop and cache resources).
//  3. Auth-key rate-limiter close (graceful Redis connection teardown, if used).
//  4. Combo subsystem close.
//  5. Alias subsystem close.
//  6. Workflow subsystem close.
//  6. Model override subsystem close.
//  7. Guardrail subsystem close.
//  8. Managed auth keys subsystem close.
//  9. Reserved for external auth module close.
//
// 10. Batch store close (flushes pending entries).
// 11. Reserved for external limit module close.
// 12. Usage logger close (flushes pending usage records).
// 13. Audit logger close (flushes pending audit records).
//
// Shutdown is idempotent and safe for repeated calls; after the first call, subsequent calls are no-ops.
// It attempts every close step, aggregates failures, and returns a joined error if any step fails.
func (a *App) Shutdown(ctx context.Context) error {
	a.shutdownMu.Lock()
	if a.shutdown {
		a.shutdownMu.Unlock()
		return nil
	}
	a.shutdown = true
	a.shutdownMu.Unlock()

	slog.Info("shutting down application...")

	var errs []error

	// 1. Stop HTTP server first (stop accepting new requests)
	a.serverMu.Lock()
	serverStop := a.serverStop
	serverDone := a.serverDone
	a.serverMu.Unlock()
	if serverStop != nil {
		serverStop()
	}
	if serverDone != nil {
		select {
		case err := <-serverDone:
			a.serverMu.Lock()
			a.serverDone = nil
			a.serverStop = nil
			a.serverMu.Unlock()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("server shutdown error", "error", err)
				errs = append(errs, fmt.Errorf("server shutdown: %w", err))
			}
		case <-ctx.Done():
			slog.Error("server shutdown timed out", "error", ctx.Err())
			errs = append(errs, fmt.Errorf("server shutdown: %w", ctx.Err()))
		}
	}

	// 2. Close providers (stops model refresh and provider-owned resources)
	if a.providers != nil {
		if err := a.providers.Close(); err != nil {
			slog.Error("providers close error", "error", err)
			errs = append(errs, fmt.Errorf("providers close: %w", err))
		}
	}

	// 3. Close rate limiter (graceful Redis connection teardown)
	if a.authKeyRateLimiterCloser != nil {
		if err := a.authKeyRateLimiterCloser.Close(); err != nil {
			slog.Error("rate limiter close error", "error", err)
			errs = append(errs, fmt.Errorf("rate limiter close: %w", err))
		}
	}

	// 4. Close combos subsystem.
	if a.combos != nil {
		if err := a.combos.Close(); err != nil {
			slog.Error("combos close error", "error", err)
			errs = append(errs, fmt.Errorf("combos close: %w", err))
		}
	}

	// 5. Close aliases subsystem.
	if a.aliases != nil {
		if err := a.aliases.Close(); err != nil {
			slog.Error("aliases close error", "error", err)
			errs = append(errs, fmt.Errorf("aliases close: %w", err))
		}
	}

	// 6. Close workflows subsystem.
	if a.workflows != nil {
		if err := a.workflows.Close(); err != nil {
			slog.Error("workflows close error", "error", err)
			errs = append(errs, fmt.Errorf("workflows close: %w", err))
		}
	}

	// 6. Close model overrides subsystem.
	if a.modelOverrides != nil {
		if err := a.modelOverrides.Close(); err != nil {
			slog.Error("model overrides close error", "error", err)
			errs = append(errs, fmt.Errorf("model overrides close: %w", err))
		}
	}

	// 7. Close reusable guardrails subsystem.
	if a.guardrails != nil {
		if err := a.guardrails.Close(); err != nil {
			slog.Error("guardrails close error", "error", err)
			errs = append(errs, fmt.Errorf("guardrails close: %w", err))
		}
	}

	// 8. Close managed auth keys subsystem.
	if a.authKeys != nil {
		if err := a.authKeys.Close(); err != nil {
			slog.Error("auth keys close error", "error", err)
			errs = append(errs, fmt.Errorf("auth keys close: %w", err))
		}
	}
	// 9. Close batch store (flushes pending entries)
	if a.batch != nil {
		if err := a.batch.Close(); err != nil {
			slog.Error("batch store close error", "error", err)
			errs = append(errs, fmt.Errorf("batch close: %w", err))
		}
	}
	// 10. Close usage tracking (flushes pending entries)
	if a.usage != nil {
		if err := a.usage.Close(); err != nil {
			slog.Error("usage logger close error", "error", err)
			errs = append(errs, fmt.Errorf("usage close: %w", err))
		}
	}

	// 13. Close audit logging (flushes pending logs)
	if a.audit != nil {
		if err := a.audit.Close(); err != nil {
			slog.Error("audit logger close error", "error", err)
			errs = append(errs, fmt.Errorf("audit close: %w", err))
		}
	}

	// 14. Save pool member counters for restart recovery.
	if a.providers != nil && a.providers.Pools != nil && a.poolCountersPath != "" {
		if err := a.providers.Pools.SaveCounters(a.poolCountersPath); err != nil {
			slog.Error("pool counters save error", "error", err)
			errs = append(errs, fmt.Errorf("pool counters save: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %w", errors.Join(errs...))
	}

	slog.Info("application shutdown complete")
	return nil
}

// logStartupInfo logs the application configuration on startup.
func (a *App) logStartupInfo() {
	cfg := a.config

	authMode := "none"
	authSummary := ""
	managedKeysConfigured := a.authKeys != nil && a.authKeys.Service != nil && a.authKeys.Service.Enabled()
	switch {
	case cfg.Server.MasterKey != "" && managedKeysConfigured:
		authMode = "master_key+managed_keys"
		authSummary = fmt.Sprintf("mode=%s total=%d active=%d", authMode, a.authKeys.Service.Total(), a.authKeys.Service.ActiveCount())
	case managedKeysConfigured:
		authMode = "managed_keys"
		authSummary = fmt.Sprintf("mode=%s total=%d active=%d", authMode, a.authKeys.Service.Total(), a.authKeys.Service.ActiveCount())
	case cfg.Server.MasterKey == "":
		slog.Warn("SECURITY WARNING: AURORA_MASTER_KEY not set - server running in UNSAFE MODE",
			"security_risk", "unauthenticated access allowed",
			"recommendation", "set AURORA_MASTER_KEY environment variable to secure this gateway")
		authSummary = "UNSAFE — no master key set"
	default:
		authMode = "master_key"
		authSummary = "mode=master_key"
	}

	metricsSummary := "disabled"
	if cfg.Metrics.Enabled {
		metricsSummary = fmt.Sprintf("enabled endpoint=%s", cfg.Metrics.Endpoint)
	}

	auditSummary := "disabled"
	if cfg.Logging.Enabled {
		auditSummary = fmt.Sprintf("enabled bodies=%v headers=%v ret=%dd", cfg.Logging.LogBodies, cfg.Logging.LogHeaders, cfg.Logging.RetentionDays)
	}

	usageSummary := "disabled"
	if cfg.Usage.Enabled {
		usageSummary = fmt.Sprintf("enabled buffer=%d flush=%ds ret=%dd", cfg.Usage.BufferSize, cfg.Usage.FlushInterval, cfg.Usage.RetentionDays)
	}

	onOff := func(b bool) string {
		if b {
			return "on"
		}
		return "off"
	}
	slog.Info("services configured",
		"auth", authMode,
		"storage", cfg.Storage.Type,
		"metrics", onOff(cfg.Metrics.Enabled),
		"audit", onOff(cfg.Logging.Enabled),
		"usage", onOff(cfg.Usage.Enabled),
	)
	slog.Debug("services detail",
		"auth", authSummary,
		"metrics", metricsSummary,
		"storage", cfg.Storage.Type,
		"audit", auditSummary,
		"usage", usageSummary,
	)
}

// initAdmin creates the admin API handler and optionally the dashboard handler.
// Returns nil dashboard handler if uiEnabled is false.
func initAdmin(
	auditLogger auditlog.LoggerInterface,
	auditStorage, usageStorage storage.Storage,
	registry *providers.ModelRegistry,
	masterKey string,
	configuredProviders []providers.SanitizedProviderConfig,
	pools *pool.Registry,
	authKeyService *authkeys.Service,
	authKeyRateInspector admin.AuthKeyRateLimitInspector,
	aliasService *aliases.Service,
	comboService *combos.Service,
	cliToolService *clitools.Service,
	modelOverrideService *modeloverrides.Service,
	workflowService *workflow.Service,
	guardrailService *guardrails.Service,
	providerOverrides *admin.ProviderOverrideStore,
	poolOverrides *admin.PoolOverrideStore,
	runtimeRefresher admin.RuntimeRefresher,
	settingsManager admin.DashboardSettingsManager,
	runtimeConfig admin.DashboardConfigResponse,
	usagePricingRecalculationEnabled bool,
	basePath string,
	uiEnabled bool,
) (*admin.Handler, *dashboard.Handler, error) {
	// Find a storage connection for reading usage data
	var store storage.Storage
	if auditStorage != nil {
		store = auditStorage
	} else if usageStorage != nil {
		store = usageStorage
	}

	// Create usage reader (may be nil if no storage)
	var reader usage.UsageReader
	var pricingRecalculator usage.PricingRecalculator
	if store != nil {
		var err error
		reader, err = usage.NewReader(store)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create usage reader: %w", err)
		}
		if usagePricingRecalculationEnabled {
			pricingRecalculator, err = usage.NewPricingRecalculator(store)
			if err != nil {
				slog.Warn("usage pricing recalculation unavailable", "error", err)
				pricingRecalculator = nil
			}
		}
	}
	runtimeConfig.PricingRecalculation = dashboardEnabledValue(usagePricingRecalculationEnabled && pricingRecalculator != nil)

	// Create audit reader (only from audit storage, because the usage-only storage
	// schema may not include the audit_logs table/collection).
	var auditReader auditlog.Reader
	if auditStorage != nil {
		var err error
		auditReader, err = auditlog.NewReader(auditStorage)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create audit reader: %w", err)
		}
	}

	adminHandler := admin.NewHandler(
		reader,
		registry,
		admin.WithMasterKey(masterKey),
		admin.WithConfiguredProviders(configuredProviders),
		admin.WithPools(pools),
		admin.WithUsagePricingRecalculator(pricingRecalculator),
		admin.WithAuditReader(auditReader),
		admin.WithAuditLogger(auditLogger),
		admin.WithConsole(console.NewService(auditLogger, 500)),
		admin.WithAuthKeys(authKeyService),
		admin.WithAuthKeyRateInspector(authKeyRateInspector),
		admin.WithAliases(aliasService),
		admin.WithCombos(comboService),
		admin.WithCLITools(cliToolService),
		admin.WithModelOverrides(modelOverrideService),
		admin.WithWorkflows(workflowService),
		admin.WithGuardrailService(guardrailService),
		admin.WithRuntimeRefresher(runtimeRefresher),
		admin.WithDashboardSettingsManager(settingsManager),
		admin.WithProviderOverrides(providerOverrides),
		admin.WithPoolWeights(poolOverrides),
		admin.WithDashboardRuntimeConfig(runtimeConfig),
	)

	var dashHandler *dashboard.Handler
	if uiEnabled {
		var err error
		dashHandler, err = dashboard.NewWithBasePath(basePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize dashboard: %w", err)
		}
	}

	return adminHandler, dashHandler, nil
}

func configGuardrailDefinitions(cfg config.GuardrailsConfig) ([]guardrails.Definition, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	definitions := make([]guardrails.Definition, 0, len(cfg.Rules))
	for i, rule := range cfg.Rules {
		name := strings.TrimSpace(rule.Name)
		ruleType := strings.ToLower(strings.TrimSpace(rule.Type))
		switch ruleType {
		case "llm-based-altering":
			ruleType = "llm_based_altering"
		}
		if name == "" {
			return nil, fmt.Errorf("guardrail rule #%d: name is required", i)
		}
		if ruleType == "" {
			return nil, fmt.Errorf("guardrail rule #%d (%q): type is required", i, name)
		}

		var rawConfig []byte
		var err error
		switch ruleType {
		case "system_prompt":
			rawConfig, err = json.Marshal(map[string]any{
				"mode":    rule.SystemPrompt.Mode,
				"content": rule.SystemPrompt.Content,
			})
		case "llm_based_altering":
			rawConfig, err = json.Marshal(map[string]any{
				"model":               rule.LLMBasedAltering.Model,
				"provider":            rule.LLMBasedAltering.Provider,
				"prompt":              rule.LLMBasedAltering.Prompt,
				"roles":               rule.LLMBasedAltering.Roles,
				"skip_content_prefix": rule.LLMBasedAltering.SkipContentPrefix,
				"max_tokens":          rule.LLMBasedAltering.MaxTokens,
			})
		case "regex_block":
			rawConfig, err = json.Marshal(map[string]any{
				"action":      rule.RegexBlock.Action,
				"patterns":    rule.RegexBlock.Patterns,
				"replacement": rule.RegexBlock.Replacement,
				"roles":       rule.RegexBlock.Roles,
			})
		case "pii_redact":
			rawConfig, err = json.Marshal(map[string]any{
				"kinds": rule.PIIRedact.Kinds,
				"roles": rule.PIIRedact.Roles,
			})
		case "length_limit":
			rawConfig, err = json.Marshal(map[string]any{
				"max_chars":            rule.LengthLimit.MaxChars,
				"max_estimated_tokens": rule.LengthLimit.MaxEstimatedTokens,
			})
		default:
			return nil, fmt.Errorf("guardrail rule #%d (%q): unsupported type %q", i, name, ruleType)
		}
		if err != nil {
			return nil, fmt.Errorf("guardrail rule #%d (%q): marshal config: %w", i, name, err)
		}
		definitions = append(definitions, guardrails.Definition{
			Name:      name,
			Type:      ruleType,
			Direction: strings.TrimSpace(rule.Direction),
			UserPath:  strings.TrimSpace(rule.UserPath),
			Config:    rawConfig,
		})
	}
	return definitions, nil
}

func defaultWorkflowInput(cfg *config.Config, availableGuardrails []string, configuredGuardrails []guardrails.Definition) workflow.CreateInput {
	fallbackEnabled := fallbackFeatureEnabledGlobally(cfg)
	payload := workflow.Payload{
		SchemaVersion: 1,
		Features: workflow.FeatureFlags{
			Cache:    responseCacheConfigured(cfg.Cache.Response),
			Audit:    cfg.Logging.Enabled,
			Usage:    cfg.Usage.Enabled,
			Fallback: &fallbackEnabled,
		},
	}
	available := make(map[string]struct{}, len(availableGuardrails))
	for _, name := range availableGuardrails {
		available[strings.TrimSpace(name)] = struct{}{}
	}
	for _, definition := range configuredGuardrails {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}
		available[name] = struct{}{}
	}
	if cfg.Guardrails.Enabled && len(cfg.Guardrails.Rules) > 0 {
		payload.Guardrails = make([]workflow.GuardrailStep, 0, len(cfg.Guardrails.Rules))
		for _, rule := range cfg.Guardrails.Rules {
			name := strings.TrimSpace(rule.Name)
			if name == "" {
				continue
			}
			if len(available) > 0 {
				if _, ok := available[name]; !ok {
					continue
				}
			}
			payload.Guardrails = append(payload.Guardrails, workflow.GuardrailStep{
				Ref:  name,
				Step: rule.Order,
			})
		}
	}
	payload.Features.Guardrails = cfg.Guardrails.Enabled

	return workflow.CreateInput{
		Scope:       workflow.Scope{},
		Activate:    true,
		Name:        workflow.ManagedDefaultGlobalName,
		Description: workflow.ManagedDefaultGlobalDescription,
		Payload:     payload,
	}
}

// RuntimeSummary holds live state captured after startup, used by main.go for
// the startup summary display.
type RuntimeSummary struct {
	AuthMode         string
	ManagedKeyTotal  int
	ManagedKeyActive int

	ProviderCount  int
	ModelCount     int
	PoolCount      int
	ComboCount     int
	WorkflowCount  int
	GuardrailCount int
	AuthKeyCount   int

	AuditEnabled       bool
	UsageEnabled       bool
	CacheEnabled       string
	SemanticCache      string
	PlaygroundEnabled  bool
	LiveConsoleEnabled bool
	PprofEnabled       bool
	SwaggerEnabled     bool
	PassthroughEnabled bool
}

// StartupSummary returns runtime state from the initialized application.
func (a *App) StartupSummary() RuntimeSummary {
	s := RuntimeSummary{
		ProviderCount: len(a.providers.ConfiguredProviders),
	}

	if a.providers.Registry != nil {
		s.ModelCount = a.providers.Registry.ModelCount()
	}
	if a.providers.Pools != nil {
		s.PoolCount = a.providers.Pools.Count()
	}

	switch {
	case a.config.Server.MasterKey != "" && a.authKeys != nil && a.authKeys.Service != nil && a.authKeys.Service.Enabled():
		s.AuthMode = "master_key+managed_keys"
		s.ManagedKeyTotal = a.authKeys.Service.Total()
		s.ManagedKeyActive = a.authKeys.Service.ActiveCount()
		s.AuthKeyCount = s.ManagedKeyTotal
	case a.authKeys != nil && a.authKeys.Service != nil && a.authKeys.Service.Enabled():
		s.AuthMode = "managed_keys"
		s.ManagedKeyTotal = a.authKeys.Service.Total()
		s.ManagedKeyActive = a.authKeys.Service.ActiveCount()
		s.AuthKeyCount = s.ManagedKeyTotal
	case a.config.Server.MasterKey != "":
		s.AuthMode = "master_key"
	default:
		s.AuthMode = "none"
	}

	if a.combos != nil && a.combos.Service != nil {
		s.ComboCount = len(a.combos.Service.List())
	}
	if a.workflows != nil && a.workflows.Service != nil {
		views, _ := a.workflows.Service.ListViews(context.Background())
		s.WorkflowCount = len(views)
	}
	if a.guardrails != nil && a.guardrails.Service != nil {
		s.GuardrailCount = a.guardrails.Service.Len()
	}
	if a.audit != nil && a.audit.Logger != nil {
		s.AuditEnabled = a.audit.Logger.Config().Enabled
	}
	if a.usage != nil && a.usage.Logger != nil {
		s.UsageEnabled = a.usage.Logger.Config().Enabled
	}

	s.PprofEnabled = a.config.Server.PprofEnabled
	s.SwaggerEnabled = a.config.Server.SwaggerEnabled
	s.PassthroughEnabled = a.config.Server.EnablePassthroughRoutes

	cache := a.config.Cache.Response
	if cache.Simple != nil && cache.Simple.Enabled != nil && *cache.Simple.Enabled {
		s.CacheEnabled = "simple"
	}
	if cache.Semantic != nil && cache.Semantic.Enabled != nil && *cache.Semantic.Enabled {
		if s.CacheEnabled != "" {
			s.CacheEnabled += "+semantic"
		} else {
			s.CacheEnabled = "semantic"
		}
		s.SemanticCache = fmt.Sprintf("%s (%.2f)", cache.Semantic.VectorStore.Type, cache.Semantic.SimilarityThreshold)
	}
	if s.CacheEnabled == "" {
		s.CacheEnabled = "off"
	}

	return s
}

func dashboardRuntimeConfig(cfg *config.Config, usageEnabled bool) admin.DashboardConfigResponse {
	capabilities := map[string]bool(nil)
	edition := string(config.EditionOSS)
	if cfg != nil {
		edition = string(config.NormalizeEditionName(cfg.Edition.Name))
		capabilities = config.ResolveCapabilities(cfg.Edition)
	}
	return admin.DashboardConfigResponse{
		Edition:              edition,
		Capabilities:         config.CapabilityList(capabilities),
		CapabilityMap:        capabilities,
		FeatureFallbackMode:  dashboardFallbackModeValue(cfg),
		LoggingEnabled:       dashboardEnabledValue(cfg != nil && cfg.Logging.Enabled),
		UsageEnabled:         dashboardEnabledValue(cfg != nil && cfg.Usage.Enabled),
		BudgetsEnabled:       dashboardEnabledValue(false),
		GuardrailsEnabled:    dashboardEnabledValue(cfg != nil && cfg.Guardrails.Enabled),
		CacheEnabled:         dashboardEnabledValue(cacheAnalyticsConfigured(cfg, usageEnabled)),
		RedisURL:             dashboardEnabledValue(simpleResponseCacheConfigured(cfg)),
		SemanticCacheEnabled: dashboardEnabledValue(semanticResponseCacheConfigured(cfg)),
		Fallback:             dashboardFallbackConfigSnapshot(cfg),
		Settings:             dashboardSettingsSnapshot(cfg),
		RuntimeFeatures:      dashboardRuntimeFeatures(cfg, usageEnabled),
	}
}

func dashboardSettingsSnapshot(cfg *config.Config) admin.DashboardSettingsSnapshot {
	if cfg == nil {
		return admin.DashboardSettingsSnapshot{}
	}

	semanticTTL := 0
	semanticMaxConversationMessages := 0
	semanticSimilarityThreshold := 0.0
	semanticPromptSimilarityMin := 0.0
	semanticExcludeSystemPrompt := false
	semanticEmbedderProvider := ""
	semanticEmbedderModel := ""
	semanticVectorStoreType := ""
	semanticVectorStoreHints := []string(nil)
	if sem := cfg.Cache.Response.Semantic; sem != nil {
		if sem.TTL != nil {
			semanticTTL = *sem.TTL
		}
		if sem.MaxConversationMessages != nil {
			semanticMaxConversationMessages = *sem.MaxConversationMessages
		}
		semanticSimilarityThreshold = sem.SimilarityThreshold
		semanticPromptSimilarityMin = sem.PromptSimilarityThreshold
		semanticExcludeSystemPrompt = sem.ExcludeSystemPrompt
		semanticEmbedderProvider = strings.TrimSpace(sem.Embedder.Provider)
		semanticEmbedderModel = strings.TrimSpace(sem.Embedder.Model)
		semanticVectorStoreType = strings.TrimSpace(sem.VectorStore.Type)
		semanticVectorStoreHints = semanticVectorStoreHintsForDashboard(sem.VectorStore)
	}

	exactCacheTTL := 0
	exactCacheRedisKey := ""
	exactCacheRedisURL := ""
	if simple := cfg.Cache.Response.Simple; simple != nil && simple.Redis != nil {
		exactCacheTTL = simple.Redis.TTL
		exactCacheRedisKey = strings.TrimSpace(simple.Redis.Key)
		exactCacheRedisURL = redactDashboardURL(simple.Redis.URL)
	}
	modelCacheLocalDir := ""
	modelCacheRedisURL := ""
	modelCacheRedisKey := ""
	modelCacheRedisTTL := 0
	if local := cfg.Cache.Model.Local; local != nil {
		modelCacheLocalDir = strings.TrimSpace(local.CacheDir)
	}
	if redis := cfg.Cache.Model.Redis; redis != nil {
		modelCacheRedisURL = redactDashboardURL(redis.URL)
		modelCacheRedisKey = strings.TrimSpace(redis.Key)
		modelCacheRedisTTL = redis.TTL
	}
	vectorSnapshot := dashboardVectorStoreSnapshot(cfg)

	return admin.DashboardSettingsSnapshot{
		Client: admin.DashboardClientSettingsSnapshot{
			Port:                  cfg.Server.Port,
			BasePath:              cfg.Server.BasePath,
			BodySizeLimit:         cfg.Server.BodySizeLimit,
			SwaggerEnabled:        cfg.Server.SwaggerEnabled,
			PprofEnabled:          cfg.Server.PprofEnabled,
			AdminEndpointsEnabled: cfg.Admin.EndpointsEnabled,
			AdminUIEnabled:        cfg.Admin.UIEnabled,

			EnableAnthropicIngress:          cfg.Server.EnableAnthropicIngress,
			EnablePassthroughRoutes:         cfg.Server.EnablePassthroughRoutes,
			AllowPassthroughV1Alias:         cfg.Server.AllowPassthroughV1Alias,
			EnabledPassthroughProviders:     append([]string(nil), cfg.Server.EnabledPassthroughProviders...),
			ModelsEnabledByDefault:          cfg.Models.EnabledByDefault,
			ModelOverridesEnabled:           cfg.Models.OverridesEnabled,
			KeepOnlyAliasesAtModelsEndpoint: cfg.Models.KeepOnlyAliasesAtModelsEndpoint,
			ConfiguredProviderModelsMode:    string(cfg.Models.ConfiguredProviderModelsMode),
		},
		Caching: admin.DashboardCachingSettingsSnapshot{
			ModelCacheBackend:               modelCacheBackendForDashboard(cfg),
			ModelCacheLocalDir:              modelCacheLocalDir,
			ModelCacheRedisURL:              modelCacheRedisURL,
			ModelCacheRedisKey:              modelCacheRedisKey,
			ModelCacheRedisTTLSeconds:       modelCacheRedisTTL,
			ModelRefreshIntervalSeconds:     cfg.Cache.Model.RefreshInterval,
			ModelListURL:                    cfg.Cache.Model.ModelList.URL,
			ModelListLocalPath:              cfg.Cache.Model.ModelList.LocalPath,
			ModelListUserOverridesPath:      cfg.Cache.Model.ModelList.UserOverridesPath,
			ExactCacheEnabled:               simpleResponseCacheConfigured(cfg),
			ExactCacheRedisURL:              exactCacheRedisURL,
			ExactCacheRedisKey:              exactCacheRedisKey,
			ExactCacheTTLSeconds:            exactCacheTTL,
			SemanticCacheEnabled:            semanticResponseCacheConfigured(cfg),
			SemanticSimilarityThreshold:     semanticSimilarityThreshold,
			SemanticPromptSimilarityMin:     semanticPromptSimilarityMin,
			SemanticTTLSeconds:              semanticTTL,
			SemanticMaxConversationMessages: semanticMaxConversationMessages,
			SemanticExcludeSystemPrompt:     semanticExcludeSystemPrompt,
			SemanticEmbedderProvider:        semanticEmbedderProvider,
			SemanticEmbedderModel:           semanticEmbedderModel,
			SemanticVectorStoreType:         semanticVectorStoreType,
			SemanticVectorStoreHints:        semanticVectorStoreHints,
			SemanticVectorStoreURL:          vectorSnapshot.url,
			SemanticVectorStoreCollection:   vectorSnapshot.collection,
			SemanticVectorStoreTable:        vectorSnapshot.table,
			SemanticVectorStoreNamespace:    vectorSnapshot.namespace,
			SemanticVectorStoreClass:        vectorSnapshot.class,
			SemanticVectorStoreDimension:    vectorSnapshot.dimension,
			SemanticVectorStoreAPIKeySet:    vectorSnapshot.apiKeySet,
			PromptCacheMode:                 cfg.Cache.Prompt.Mode,
			PromptCacheSystemPrompt:         cfg.Cache.Prompt.SystemPromptCache,
			PromptCacheFirstMessage:         cfg.Cache.Prompt.FirstMessageCache,
			PromptCacheTools:                cfg.Cache.Prompt.ToolsCache,
			PromptCacheMinTokens:            cfg.Cache.Prompt.MinTokensBeforeCache,
		},
		Logging: admin.DashboardLoggingSettingsSnapshot{
			Enabled:               cfg.Logging.Enabled,
			LogBodies:             cfg.Logging.LogBodies,
			LogHeaders:            cfg.Logging.LogHeaders,
			BufferSize:            cfg.Logging.BufferSize,
			FlushIntervalSeconds:  cfg.Logging.FlushInterval,
			RetentionDays:         cfg.Logging.RetentionDays,
			OnlyModelInteractions: cfg.Logging.OnlyModelInteractions,
		},
		Observability: admin.DashboardObservabilitySettingsSnapshot{
			MetricsEnabled:  cfg.Metrics.Enabled,
			MetricsEndpoint: cfg.Metrics.Endpoint,
			StorageType:     storageDependency(cfg),
		},

		Storage: admin.DashboardStorageSettingsSnapshot{
			Type:               storageDependency(cfg),
			SQLitePath:         strings.TrimSpace(cfg.Storage.SQLite.Path),
			PostgreSQLURL:      redactDashboardURL(cfg.Storage.PostgreSQL.URL),
			PostgreSQLMaxConns: cfg.Storage.PostgreSQL.MaxConns,
			MongoDBURL:         redactDashboardURL(cfg.Storage.MongoDB.URL),
			MongoDBDatabase:    strings.TrimSpace(cfg.Storage.MongoDB.Database),
		},
		Performance: admin.DashboardPerformanceSettingsSnapshot{
			HTTPTimeoutSeconds:                cfg.HTTP.Timeout,
			HTTPResponseHeaderTimeoutSeconds:  cfg.HTTP.ResponseHeaderTimeout,
			WorkflowRefreshIntervalSeconds:    int64(workflowRefreshInterval(cfg) / time.Second),
			RetryMaxRetries:                   cfg.Resilience.Retry.MaxRetries,
			RetryInitialBackoffMilliseconds:   cfg.Resilience.Retry.InitialBackoff.Milliseconds(),
			RetryMaxBackoffMilliseconds:       cfg.Resilience.Retry.MaxBackoff.Milliseconds(),
			RetryBackoffFactor:                cfg.Resilience.Retry.BackoffFactor,
			RetryJitterFactor:                 cfg.Resilience.Retry.JitterFactor,
			CircuitBreakerFailureThreshold:    cfg.Resilience.CircuitBreaker.FailureThreshold,
			CircuitBreakerSuccessThreshold:    cfg.Resilience.CircuitBreaker.SuccessThreshold,
			CircuitBreakerTimeoutMilliseconds: cfg.Resilience.CircuitBreaker.Timeout.Milliseconds(),
		},
		Proxy: admin.DashboardProxySettingsSnapshot{
			HTTPProxy:        strings.TrimSpace(cfg.HTTP.Proxy.HTTPProxy),
			HTTPSProxy:       strings.TrimSpace(cfg.HTTP.Proxy.HTTPSProxy),
			NoProxy:          strings.TrimSpace(cfg.HTTP.Proxy.NoProxy),
			ProxyAuthEnabled: cfg.HTTP.Proxy.ProxyAuthEnabled,
			CACertPEM:        strings.TrimSpace(cfg.HTTP.Proxy.CACertPEM),
		},
		Security: admin.DashboardSecuritySettingsSnapshot{
			MasterKeyConfigured: strings.TrimSpace(cfg.Server.MasterKey) != "",
			GuardrailsEnabled:   cfg.Guardrails.Enabled,
			BatchGuardrails:     cfg.Guardrails.EnableForBatchProcessing,
		},
		Pricing: admin.DashboardPricingSettingsSnapshot{
			UsageEnabled:                  cfg.Usage.Enabled,
			EnforceReturningUsageData:     cfg.Usage.EnforceReturningUsageData,
			PricingRecalculationEnabled:   cfg.Usage.PricingRecalculationEnabled,
			UsageBufferSize:               cfg.Usage.BufferSize,
			UsageFlushIntervalSeconds:     cfg.Usage.FlushInterval,
			UsageRetentionDays:            cfg.Usage.RetentionDays,
			BudgetsEnabled:                false,
			ConfiguredBudgetUserPathCount: 0,
		},
		TokenSaver: admin.DashboardTokenSaverSettingsSnapshot{
			Enabled:         cfg.TokenSaver.Enabled,
			ApplyStreaming:  cfg.TokenSaver.ApplyStreaming,
			Endpoints:       append([]string(nil), cfg.TokenSaver.Endpoints...),
			OutputEnabled:   cfg.TokenSaver.Output.Enabled,
			OutputProfile:   cfg.TokenSaver.Output.Profile,
			OutputLevel:     cfg.TokenSaver.Output.Level,
			EmitHeaders:     cfg.TokenSaver.EmitHeaders,
			OnError:         cfg.TokenSaver.OnError,
			ModelInclude:    append([]string(nil), cfg.TokenSaver.Models.Include...),
			ModelExclude:    append([]string(nil), cfg.TokenSaver.Models.Exclude...),
			ProviderInclude: append([]string(nil), cfg.TokenSaver.Providers.Include...),
			ProviderExclude: append([]string(nil), cfg.TokenSaver.Providers.Exclude...),
			AuditEnabled:    cfg.TokenSaver.Audit.Enabled,
		},
	}
}

func applyHTTPClientConfig(cfg config.HTTPConfig) {
	base := httpclient.DefaultConfig()
	if cfg.Timeout > 0 {
		base.Timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.ResponseHeaderTimeout > 0 {
		base.ResponseHeaderTimeout = time.Duration(cfg.ResponseHeaderTimeout) * time.Second
	}
	if value := strings.TrimSpace(cfg.Proxy.HTTPProxy); value != "" {
		base.HTTPProxy = value
	}
	if value := strings.TrimSpace(cfg.Proxy.HTTPSProxy); value != "" {
		base.HTTPSProxy = value
	}
	if value := strings.TrimSpace(cfg.Proxy.NoProxy); value != "" {
		base.NoProxy = value
	}
	base.CACertPEM = strings.TrimSpace(cfg.Proxy.CACertPEM)
	httpclient.SetDefaultConfigOverride(&base)
}

type dashboardVectorStoreDetails struct {
	url        string
	collection string
	table      string
	namespace  string
	class      string
	dimension  int
	apiKeySet  bool
}

func dashboardVectorStoreSnapshot(cfg *config.Config) dashboardVectorStoreDetails {
	if cfg == nil || cfg.Cache.Response.Semantic == nil {
		return dashboardVectorStoreDetails{}
	}
	store := cfg.Cache.Response.Semantic.VectorStore
	switch strings.ToLower(strings.TrimSpace(store.Type)) {
	case "qdrant":
		return dashboardVectorStoreDetails{
			url:        redactDashboardURL(store.Qdrant.URL),
			collection: strings.TrimSpace(store.Qdrant.Collection),
			apiKeySet:  strings.TrimSpace(store.Qdrant.APIKey) != "",
		}
	case "pgvector":
		return dashboardVectorStoreDetails{
			url:       redactDashboardURL(store.PGVector.URL),
			table:     strings.TrimSpace(store.PGVector.Table),
			dimension: store.PGVector.Dimension,
		}
	case "pinecone":
		return dashboardVectorStoreDetails{
			url:       strings.TrimSpace(store.Pinecone.Host),
			namespace: strings.TrimSpace(store.Pinecone.Namespace),
			dimension: store.Pinecone.Dimension,
			apiKeySet: strings.TrimSpace(store.Pinecone.APIKey) != "",
		}
	case "weaviate":
		return dashboardVectorStoreDetails{
			url:       redactDashboardURL(store.Weaviate.URL),
			class:     strings.TrimSpace(store.Weaviate.Class),
			apiKeySet: strings.TrimSpace(store.Weaviate.APIKey) != "",
		}
	default:
		return dashboardVectorStoreDetails{}
	}
}

func redactDashboardURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User != nil {
		if username := parsed.User.Username(); username != "" {
			parsed.User = url.UserPassword(username, "***")
		} else {
			parsed.User = nil
		}
	}
	return strings.TrimRight(parsed.String(), "/")
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func modelCacheBackendForDashboard(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.Cache.Model.Redis != nil && strings.TrimSpace(cfg.Cache.Model.Redis.URL) != "" {
		return "redis"
	}
	if cfg.Cache.Model.Local != nil {
		return "local"
	}
	return ""
}

func semanticVectorStoreHintsForDashboard(store config.VectorStoreConfig) []string {
	storeType := strings.TrimSpace(store.Type)
	if storeType == "" {
		return nil
	}
	parts := make([]string, 0, 2)
	switch storeType {
	case "qdrant":
		if url := strings.TrimSpace(store.Qdrant.URL); url != "" {
			parts = append(parts, url)
		}
		if collection := strings.TrimSpace(store.Qdrant.Collection); collection != "" {
			parts = append(parts, "collection: "+collection)
		}
	case "pgvector":
		if table := strings.TrimSpace(store.PGVector.Table); table != "" {
			parts = append(parts, "table: "+table)
		}
		if store.PGVector.Dimension > 0 {
			parts = append(parts, fmt.Sprintf("dimension: %d", store.PGVector.Dimension))
		}
	case "pinecone":
		if host := strings.TrimSpace(store.Pinecone.Host); host != "" {
			parts = append(parts, host)
		}
		if namespace := strings.TrimSpace(store.Pinecone.Namespace); namespace != "" {
			parts = append(parts, "namespace: "+namespace)
		}
		if store.Pinecone.Dimension > 0 {
			parts = append(parts, fmt.Sprintf("dimension: %d", store.Pinecone.Dimension))
		}
	case "weaviate":
		if url := strings.TrimSpace(store.Weaviate.URL); url != "" {
			parts = append(parts, url)
		}
		if className := strings.TrimSpace(store.Weaviate.Class); className != "" {
			parts = append(parts, "class: "+className)
		}
	}
	return parts
}

func dashboardRuntimeFeatures(cfg *config.Config, usageEnabled bool) []admin.RuntimeFeatureSnapshot {
	if cfg == nil {
		return nil
	}
	features := []admin.RuntimeFeatureSnapshot{
		dashboardRuntimeFeature("model_cache", "Redis Model Cache", cfg.Cache.Model.Redis != nil && strings.TrimSpace(cfg.Cache.Model.Redis.URL) != "", "Caches model registry metadata so startup and model list refreshes avoid repeated provider discovery.", "You do not call this directly; aurora uses it during startup, Runtime Refresh, and model inventory loading.", "Redis", "Runtime Refresh / Models"),
		dashboardRuntimeFeature("exact_cache", "Exact Response Cache", simpleResponseCacheConfigured(cfg), "Reuses responses for identical cacheable requests.", "Send normal /v1/chat/completions or /v1/responses traffic; cache hits happen automatically when the active workflow allows cache. View totals in Usage or Cache Overview.", "Redis", "/admin/api/v1/cache/overview"),
		dashboardRuntimeFeature("semantic_cache", "Semantic Response Cache", semanticResponseCacheConfigured(cfg), "Reuses responses for semantically similar prompts using embeddings and vector search.", "Send normal model traffic; aurora embeds eligible requests, searches Qdrant, and serves a hit when the prompt is close enough. View totals in Usage or Cache Overview.", semanticCacheDependency(cfg), "/admin/api/v1/cache/overview"),
		dashboardRuntimeFeature("audit_logs", "Audit Logs", cfg.Logging.Enabled, "Stores request metadata and optional redacted headers for operator review.", "Open the Audit Logs page to inspect requests, provider routing, errors, and conversation traces.", storageDependency(cfg), "/admin/api/v1/audit/log"),
		dashboardRuntimeFeature("usage", "Usage Tracking", cfg.Usage.Enabled, "Records request counts, token usage, and estimated costs.", "Open the Usage page for charts and per-model/user-path breakdowns; pricing recalculation also reads this data.", storageDependency(cfg), "/admin/api/v1/usage/summary"),
		dashboardRuntimeFeature("guardrails", "Guardrails", cfg.Guardrails.Enabled, "Applies configured request safety and transformation rules.", "Open Settings -> Guardrails or Workflows to manage rules; enabled workflows run them before provider dispatch.", "Configured guardrail rules", "/admin/api/v1/guardrails"),
		dashboardRuntimeFeature("token_saver", "Aurora Token Saver", cfg.TokenSaver.Enabled, "Compresses eligible prompt and tool-output content before provider dispatch and can request concise responses.", "Enable and tune it in Settings; eligible /v1/chat/completions requests are transformed according to model, provider, endpoint, and streaming policy.", "Token Saver policy", "Settings / General"),
		dashboardRuntimeFeature("batch_guardrails", "Batch Guardrails", cfg.Guardrails.Enabled && cfg.Guardrails.EnableForBatchProcessing, "Applies guardrails to inline batch request items.", "Submit /v1/batches payloads; inline requests pass through the same guardrail pipeline as live traffic.", "Guardrails", "/v1/batches"),
		dashboardRuntimeFeature("fallback", "Failover", fallbackFeatureEnabledGlobally(cfg), "Routes failed or configured-primary model calls to fallback targets.", "Use normal model selectors; workflows and fallback rules decide when to retry another configured target.", "Fallback rules", "Workflows / fallback config"),
		dashboardRuntimeFeature("metrics", "Prometheus Metrics", cfg.Metrics.Enabled, "Exposes Prometheus-format gateway metrics for scraping.", "Point Prometheus or a local metrics scraper at /metrics on the aurora HTTP port, for example http://localhost:8080/metrics.", "HTTP metrics endpoint", cfg.Metrics.Endpoint),
		dashboardRuntimeFeature("passthrough", "Provider Passthrough", cfg.Server.EnablePassthroughRoutes, "Exposes provider-native API routes through the gateway.", "Call /p/{provider}/{endpoint} when you need a provider-native route instead of the OpenAI-compatible /v1 surface.", strings.Join(cfg.Server.EnabledPassthroughProviders, ", "), "/p/{provider}/{endpoint}"),
		dashboardRuntimeFeature("swagger", "Swagger API Docs", cfg.Server.SwaggerEnabled, "Serves interactive API documentation when the binary is built with Swagger support.", "Open /swagger/index.html in the browser to explore aurora API routes and schemas.", "swagger build tag", "/swagger/index.html"),
		dashboardRuntimeFeature("pprof", "Go Profiling", cfg.Server.PprofEnabled, "Exposes Go runtime profiling handlers.", "Use /debug/pprof/ locally when diagnosing CPU, heap, goroutine, or trace behavior.", "debug handlers", "/debug/pprof/"),
		dashboardRuntimeFeature("pricing_recalculation", "Usage Pricing Recalculation", usagePricingRecalculationConfigured(cfg), "Recomputes stored usage costs from current model pricing metadata.", "Use the Usage Pricing Recalculation form below in Settings to backfill or refresh historical cost estimates.", storageDependency(cfg), "/admin/api/v1/usage/recalculate-pricing"),
	}
	return features
}

func dashboardRuntimeFeature(key, label string, configured bool, description, usage, dependency, endpoint string) admin.RuntimeFeatureSnapshot {
	return admin.RuntimeFeatureSnapshot{
		Key:         key,
		Label:       label,
		Status:      dashboardFeatureStatus(configured),
		Configured:  configured,
		Description: description,
		Usage:       usage,
		Dependency:  dependency,
		Endpoint:    endpoint,
	}
}

func dashboardFeatureStatus(configured bool) string {
	if configured {
		return "enabled"
	}
	return "disabled"
}

func semanticCacheDependency(cfg *config.Config) string {
	if cfg == nil || cfg.Cache.Response.Semantic == nil {
		return "Embedding provider + vector store"
	}
	provider := strings.TrimSpace(cfg.Cache.Response.Semantic.Embedder.Provider)
	store := strings.TrimSpace(cfg.Cache.Response.Semantic.VectorStore.Type)
	parts := make([]string, 0, 2)
	if provider != "" {
		parts = append(parts, "embedder: "+provider)
	}
	if store != "" {
		parts = append(parts, "vector store: "+store)
	}
	if len(parts) == 0 {
		return "Embedding provider + vector store"
	}
	return strings.Join(parts, " + ")
}

func storageDependency(cfg *config.Config) string {
	if cfg == nil {
		return "storage"
	}
	storageType := strings.TrimSpace(cfg.Storage.Type)
	if storageType == "" {
		return "sqlite"
	}
	return storageType
}

func dashboardFallbackConfigSnapshot(cfg *config.Config) admin.FallbackConfigSnapshot {
	mode := dashboardFallbackModeValue(cfg)
	if cfg == nil {
		return admin.FallbackConfigSnapshot{Mode: mode}
	}

	rules := make([]admin.FallbackRuleSnapshot, 0, len(cfg.Fallback.Manual))
	for source, targets := range cfg.Fallback.Manual {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		safeTargets := make([]string, 0, len(targets))
		for _, target := range targets {
			if target = strings.TrimSpace(target); target != "" {
				safeTargets = append(safeTargets, target)
			}
		}
		rules = append(rules, admin.FallbackRuleSnapshot{Source: source, Targets: safeTargets})
	}

	overrides := make([]admin.FallbackOverrideSnapshot, 0, len(cfg.Fallback.Overrides))
	for model, override := range cfg.Fallback.Overrides {
		model = strings.TrimSpace(model)
		overrideMode := strings.TrimSpace(string(override.Mode))
		if model == "" || overrideMode == "" {
			continue
		}
		overrides = append(overrides, admin.FallbackOverrideSnapshot{Model: model, Mode: overrideMode})
	}

	return admin.FallbackConfigSnapshot{
		Mode:                  mode,
		ManualRulesConfigured: len(rules) > 0,
		ManualRuleCount:       len(rules),
		ManualRules:           rules,
		Overrides:             overrides,
	}
}

func usagePricingRecalculationConfigured(cfg *config.Config) bool {
	return cfg != nil && cfg.Usage.Enabled && cfg.Usage.PricingRecalculationEnabled
}

func cacheAnalyticsConfigured(cfg *config.Config, usageEnabled bool) bool {
	return cfg != nil && usageEnabled && responseCacheConfigured(cfg.Cache.Response)
}

func dashboardEnabledValue(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func dashboardFallbackModeValue(cfg *config.Config) string {
	if cfg == nil || !fallbackFeatureEnabledGlobally(cfg) {
		return string(config.FallbackModeOff)
	}

	switch mode := strings.ToLower(strings.TrimSpace(string(cfg.Fallback.DefaultMode))); mode {
	case string(config.FallbackModeAuto):
		return string(config.FallbackModeAuto)
	case string(config.FallbackModeManual):
		return string(config.FallbackModeManual)
	}

	for _, override := range cfg.Fallback.Overrides {
		if strings.EqualFold(strings.TrimSpace(string(override.Mode)), string(config.FallbackModeAuto)) {
			return string(config.FallbackModeAuto)
		}
	}
	for _, override := range cfg.Fallback.Overrides {
		if strings.EqualFold(strings.TrimSpace(string(override.Mode)), string(config.FallbackModeManual)) {
			return string(config.FallbackModeManual)
		}
	}

	return string(config.FallbackModeOff)
}

func runtimeWorkflowFeatureCaps(cfg *config.Config) core.WorkflowFeatures {
	if cfg == nil {
		return core.WorkflowFeatures{}
	}
	return core.WorkflowFeatures{
		Cache:      responseCacheConfigured(cfg.Cache.Response),
		Audit:      cfg.Logging.Enabled,
		Usage:      cfg.Usage.Enabled,
		Budget:     false,
		Guardrails: cfg.Guardrails.Enabled,
		Fallback:   fallbackFeatureEnabledGlobally(cfg),
	}
}

func workflowRefreshInterval(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.Workflows.RefreshInterval <= 0 {
		return time.Minute
	}
	return cfg.Workflows.RefreshInterval
}

func responseCacheConfigured(cfg config.ResponseCacheConfig) bool {
	return simpleResponseCacheConfiguredFromResponse(cfg) || semanticResponseCacheConfiguredFromResponse(cfg)
}

func simpleResponseCacheConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return simpleResponseCacheConfiguredFromResponse(cfg.Cache.Response)
}

func simpleResponseCacheConfiguredFromResponse(cfg config.ResponseCacheConfig) bool {
	return cfg.Simple != nil && config.SimpleCacheEnabled(cfg.Simple) &&
		cfg.Simple.Redis != nil && strings.TrimSpace(cfg.Simple.Redis.URL) != ""
}

func semanticResponseCacheConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return semanticResponseCacheConfiguredFromResponse(cfg.Cache.Response)
}

func semanticResponseCacheConfiguredFromResponse(cfg config.ResponseCacheConfig) bool {
	return cfg.Semantic != nil && config.SemanticCacheActive(cfg.Semantic)
}

func fallbackFeatureEnabledGlobally(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if fallbackModeEnabled(cfg.Fallback.DefaultMode) {
		return true
	}
	for _, override := range cfg.Fallback.Overrides {
		if fallbackModeEnabled(override.Mode) {
			return true
		}
	}
	return false
}

func fallbackModeEnabled(mode config.FallbackMode) bool {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(config.FallbackModeAuto), string(config.FallbackModeManual):
		return true
	default:
		return false
	}
}

func firstSharedStorage(candidates ...storage.Storage) storage.Storage {
	for _, candidate := range candidates {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func cloneRawProviderConfigs(configs map[string]config.RawProviderConfig) map[string]config.RawProviderConfig {
	if len(configs) == 0 {
		return map[string]config.RawProviderConfig{}
	}
	out := make(map[string]config.RawProviderConfig, len(configs))
	for name, provider := range configs {
		provider.Models = append([]config.RawProviderModel(nil), provider.Models...)
		out[name] = provider
	}
	return out
}

func cloneRawPoolConfigs(configs map[string]config.RawPoolConfig) map[string]config.RawPoolConfig {
	if len(configs) == 0 {
		return map[string]config.RawPoolConfig{}
	}
	out := make(map[string]config.RawPoolConfig, len(configs))
	for name, poolConfig := range configs {
		poolConfig.Members = append([]string(nil), poolConfig.Members...)
		if len(poolConfig.Weights) > 0 {
			weights := make(map[string]int, len(poolConfig.Weights))
			for member, weight := range poolConfig.Weights {
				weights[member] = weight
			}
			poolConfig.Weights = weights
		}
		out[name] = poolConfig
	}
	return out
}

// poolCountersFilePath returns the file path for persisting pool member
// counters. It places the file next to the SQLite database by default.
func poolCountersFilePath(cfg config.StorageConfig) string {
	dbPath := strings.TrimSpace(cfg.SQLite.Path)
	if dbPath == "" {
		dbPath = "data/aurora.db"
	}
	dir := filepath.Dir(dbPath)
	return filepath.Join(dir, "aurora-pool-counters.json")
}

func newComboFallbackResolver(result *combos.Result, next gateway.FallbackResolver) gateway.FallbackResolver {
	if result == nil || result.Service == nil {
		return next
	}
	return gateway.NewComboFallbackResolver(result.Service, next)
}
