// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"runtime/debug"

	"aurora/configuration"
	"aurora/internal/application"
	"aurora/internal/telemetry"
	"aurora/internal/providers"
	"aurora/internal/providers/anthropic"
	"aurora/internal/providers/azure"
	"aurora/internal/providers/deepseek"
	"aurora/internal/providers/gemini"
	"aurora/internal/providers/groq"
	"aurora/internal/providers/minimax"
	"aurora/internal/providers/ollama"
	"aurora/internal/providers/openai"
	"aurora/internal/providers/openrouter"
	"aurora/internal/providers/oracle"
	"aurora/internal/providers/reranker"
	"aurora/internal/providers/vllm"
	"aurora/internal/providers/xai"
	"aurora/internal/providers/zai"
	"aurora/internal/version"

	"github.com/joho/godotenv"
)

func enabledDisabled(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func logFeatureSummary(cfg *config.Config, rt app.RuntimeSummary, started time.Time, addr string) {
	authMode := rt.AuthMode
	if authMode == "" {
		authMode = "none"
	}

	cacheMode := rt.CacheEnabled
	if rt.SemanticCache != "" {
		cacheMode += " (" + rt.SemanticCache + ")"
	}

	redisConfigured := cfg.Cache.Model.Redis != nil && cfg.Cache.Model.Redis.URL != ""
	redisStatus := "not configured"
	if redisConfigured {
		redisStatus = "connected"
	}

	onOff := func(b bool) string {
		if b { return "on" }
		return "off"
	}

	status := func(count int, label string) string {
		if count > 0 {
			return fmt.Sprintf("on (%d %s)", count, label)
		}
		return "on"
	}
	statusOff := func(count int, label string) string {
		if count > 0 {
			return fmt.Sprintf("on (%d %s)", count, label)
		}
		return "off"
	}

	authSummary := authMode
	if rt.ManagedKeyActive > 0 {
		authSummary = fmt.Sprintf("%s (%d active / %d total)", authMode, rt.ManagedKeyActive, rt.ManagedKeyTotal)
	}

	providerSummary := fmt.Sprintf("%d providers", rt.ProviderCount)
	if rt.ModelCount > 0 {
		providerSummary += fmt.Sprintf(", %d models", rt.ModelCount)
	}
	if rt.PoolCount > 0 {
		providerSummary += fmt.Sprintf(", %d pools", rt.PoolCount)
	}

	slog.Info("startup complete",
		"address", addr,
		"startup_time", time.Since(started).String(),
		"edition", config.NormalizeEditionName(cfg.Edition.Name),
		"auth", authMode,
		"storage", cfg.Storage.Type,
		"redis", redisStatus,
		"cache", cacheMode,
		"audit", onOff(rt.AuditEnabled),
		"usage", onOff(rt.UsageEnabled),
		"guardrails", status(rt.GuardrailCount, "rules"),
		"providers", rt.ProviderCount,
		"models", rt.ModelCount,
		"pools", rt.PoolCount,
		"combos", rt.ComboCount,
		"workflows", rt.WorkflowCount,
		"api_keys", rt.AuthKeyCount,
		"admin_api", onOff(cfg.Admin.EndpointsEnabled),
		"admin_ui", onOff(cfg.Admin.UIEnabled),
		"metrics", cfg.Metrics.Endpoint,
		"pprof", onOff(rt.PprofEnabled),
		"swagger", onOff(rt.SwaggerEnabled),
		"passthrough", onOff(rt.PassthroughEnabled),
	)

	features := map[string]string{
		"Auth":       authSummary,
		"API Keys":   statusOff(rt.AuthKeyCount, "keys"),
		"Providers":  providerSummary,
		"Pools":      statusOff(rt.PoolCount, "pools"),
		"Combos":     statusOff(rt.ComboCount, "combos"),
		"Models":     onOff(rt.ModelCount > 0),
		"Workflows":  statusOff(rt.WorkflowCount, "wf"),
		"Guardrails": statusOff(rt.GuardrailCount, "rules"),
		"Cache":      cacheMode,
		"Audit":      onOff(rt.AuditEnabled),
		"Usage":      onOff(rt.UsageEnabled),
		"PassThrough": onOff(rt.PassthroughEnabled),
		"Swagger":    onOff(rt.SwaggerEnabled),
		"Storage":    cfg.Storage.Type,
		"Redis":      redisStatus,
	}

	printStartupSummary(started, addr, features)
}

type lifecycleApp interface {
	Start(ctx context.Context, addr string) error
	Shutdown(ctx context.Context) error
}

var shutdownTimeout = 30 * time.Second

func shutdownApplication(application lifecycleApp, ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		done <- application.Shutdown(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// startApplication calls lifecycleApp.Start and, if Start fails, attempts a
// graceful shutdown via shutdownApplication using shutdownTimeout before
// returning the original start error or a combined start/shutdown error.
func startApplication(application lifecycleApp, addr string) error {
	if err := application.Start(context.Background(), addr); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if shutdownErr := shutdownApplication(application, shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("server failed to start: %w", errors.Join(err, fmt.Errorf("shutdown after start failure: %w", shutdownErr)))
		}
		return err
	}
	return nil
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, AuroraLogo)
	fmt.Fprintln(w, "Aurora AI Gateway — one API for every LLM provider")
	fmt.Fprintln(w, "Self-hosted, open-source. Version:", version.Version)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "USAGE")
	fmt.Fprintln(w, "  aurora [flags]")
	fmt.Fprintln(w, "  aurora init [flags]")
	fmt.Fprintln(w, "  aurora update")
	fmt.Fprintln(w, "  aurora uninstall")
	fmt.Fprintln(w, "  aurora models <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "FLAGS")
	fmt.Fprintln(w, "  -version      Print version information")
	fmt.Fprintln(w, "  -help         Show this configuration reference")
	fmt.Fprintln(w, "  -help-json    Dump environment variable schema as JSON")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "SUBCOMMANDS")
	fmt.Fprintln(w, "  aurora init          Bootstrap config.yaml and .env in the current directory")
	fmt.Fprintln(w, "  aurora update        Self-update to the latest version via npm")
	fmt.Fprintln(w, "  aurora uninstall     Remove aurora from your system via npm")
	fmt.Fprintln(w, "  aurora models sync   Download upstream model registry to local file")
	fmt.Fprintln(w, "  aurora models diff   Show pricing diff between upstream and local snapshot")
	fmt.Fprintln(w, "  aurora models show   Print effective pricing for a model")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "CONFIGURATION")
	fmt.Fprintln(w, "  Aurora loads: code defaults -> YAML config -> env vars (env vars win).")
	fmt.Fprintln(w, "  Run `aurora init` to generate config.yaml and .env automatically.")
	fmt.Fprintln(w, "  Set AURORA_CONFIG_PATH to point to your config YAML path.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "ENVIRONMENT VARIABLES")
	fmt.Fprintln(w, "  Full reference below. Use -help-json for machine-readable output.")
	fmt.Fprintln(w, "")

	schema := config.GetEnvSchema()
	currentSection := ""
	for _, ev := range schema {
		if ev.Section != currentSection {
			currentSection = ev.Section
			fmt.Fprintf(w, "  [%s]\n", currentSection)
		}
		def := ev.Default
		if def == "" {
			def = "—"
		}
		desc := ev.Description
		if desc == "" {
			desc = "type: " + ev.Type
		}
		if def != "—" {
			fmt.Fprintf(w, "    %-40s  (default: %s)  %s\n", ev.Name, def, desc)
		} else {
			fmt.Fprintf(w, "    %-40s  %s\n", ev.Name, desc)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "QUICK START")
	fmt.Fprintln(w, "  1. Create a working directory: mkdir my-gateway && cd my-gateway")
	fmt.Fprintln(w, "  2. Run: aurora init")
	fmt.Fprintln(w, "  3. Edit .env with your API keys")
	fmt.Fprintln(w, "  4. Run: aurora")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "ENDPOINTS")
	fmt.Fprintln(w, "  Dashboard       http://localhost:8080/admin/dashboard")
	fmt.Fprintln(w, "  Health          http://localhost:8080/health")
	fmt.Fprintln(w, "  Swagger         http://localhost:8080/swagger/index.html")
	fmt.Fprintln(w, "  Metrics         http://localhost:8080/metrics")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Full documentation: https://github.com/aurorallm/aurora#readme")
}

func checkVersionUpdate() {
	time.Sleep(2 * time.Second)
	msg, ok := version.CheckForUpdatePlain()
	if ok {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// @title          Aurora API
// @version        1.0
// @description    High-performance AI gateway routing requests to multiple LLM providers (OpenAI, Anthropic, Gemini, Groq, OpenRouter, DeepSeek, Z.ai, xAI, MiniMax, Oracle, Ollama). Drop-in OpenAI-compatible API.
// @BasePath       /
// @schemes        http
// @securityDefinitions.apikey BearerAuth
// @in             header
// @name           Authorization
func main() {
	// Aggressive GC tuning for throughput — fewer GC cycles = more CPU for requests.
	debug.SetGCPercent(200)

	if handled, code := runUpdateSubcommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	if handled, code := runInitSubcommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	if handled, code := runModelsSubcommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}

	versionFlag := flag.Bool("version", false, "Print version information")
	helpFlag := flag.Bool("help", false, "Show help and configuration reference")
	helpJSONFlag := flag.Bool("help-json", false, "Dump environment variable schema as JSON")
	flag.Usage = func() {
		w := os.Stderr
		printHelp(w)
	}
	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *helpJSONFlag {
		schema := config.GetEnvSchema()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(schema)
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println(version.Info())
		os.Exit(0)
	}

	started := time.Now()

	if p := os.Getenv("AURORA_CONFIG_PATH"); p != "" {
		_ = godotenv.Load(filepath.Join(filepath.Dir(p), ".env"))
	} else {
		_ = godotenv.Load()
	}

	initTerminal()
	if err := configureLogging(os.Stderr, isColoredTerm); err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure logging: %v\n", err)
		os.Exit(1)
	}

	printLogoWithVersion()

	go checkVersionUpdate()

	printPhase("Configuration")
	loadStart := time.Now()
	result, err := config.Load()
	if err != nil {
		LogError("failed to load config", err)
		os.Exit(1)
	}
	slog.Info("configuration loaded",
		"edition", config.NormalizeEditionName(result.Config.Edition.Name),
		"port", result.Config.Server.Port,
		"storage", result.Config.Storage.Type,
		"duration", time.Since(loadStart),
	)
	configureSwaggerDocs(result.Config.Server.BasePath)

	factory := providers.NewProviderFactory()

	if result.Config.Metrics.Enabled {
		factory.SetHooks(telemetry.NewPrometheusHooks())
	}

	factory.Add(openai.Registration)
	factory.Add(openrouter.Registration)
	factory.Add(azure.Registration)
	factory.Add(oracle.Registration)
	factory.Add(anthropic.Registration)
	factory.Add(deepseek.Registration)
	factory.Add(gemini.Registration)
	factory.Add(groq.Registration)
	factory.Add(minimax.Registration)
	factory.Add(ollama.Registration)
	factory.Add(vllm.Registration)
	factory.Add(xai.Registration)
	factory.Add(zai.Registration)

	// Reranker is a specialized provider type for reranking/embedding
	// services (e.g. Jina AI, Cohere). It is not an LLM provider — it
	// has no chat capability. Users configure it in YAML via type: reranker.
	factory.Add(reranker.Registration)

	printPhase("Services")
	application, err := app.New(context.Background(), app.Config{
		AppConfig: result,
		Factory:   factory,
	})
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no providers") {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "ERROR: No AI providers configured.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  The gateway needs at least one provider API key to start.")
			fmt.Fprintln(os.Stderr, "  Set a provider key in your .env file or as an environment variable.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  Examples:")
			fmt.Fprintln(os.Stderr, "    GROQ_API_KEY=gsk_...      (recommended — fast & free)")
			fmt.Fprintln(os.Stderr, "    OPENAI_API_KEY=sk-...     (OpenAI)")
			fmt.Fprintln(os.Stderr, "    ANTHROPIC_API_KEY=sk-ant-... (Anthropic Claude)")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  Also set AURORA_MASTER_KEY to secure the gateway.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  See `aurora --help` for the full configuration reference.")
			fmt.Fprintln(os.Stderr, "  Docs: https://github.com/aurorallm/aurora#readme")
		} else {
			LogError("failed to initialize application", err)
		}
		os.Exit(1)
	}
	addr := ":" + result.Config.Server.Port
	logFeatureSummary(result.Config, application.StartupSummary(), started, addr)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		caught := <-quit
		slog.Warn("signal received, initiating graceful shutdown",
			"signal", caught.String(),
			"timeout", shutdownTimeout.String(),
		)

		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := shutdownApplication(application, ctx); err != nil {
			LogError("graceful shutdown failed", err, "duration", time.Since(shutdownStart))
		} else {
			slog.Info("graceful shutdown complete", "duration", time.Since(shutdownStart))
		}
	}()

	if err := startApplication(application, addr); err != nil {
		LogError("application terminated with error", err, "uptime", time.Since(started))
		os.Exit(1)
	}

	slog.Info("aurora stopped", "uptime", time.Since(started))
}
