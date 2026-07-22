package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"aurora/internal/version"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

var isColoredTerm bool

const (
	envLogFormat    = "LOG_FORMAT"
	envLogLevel     = "LOG_LEVEL"
	envLogAddSource = "LOG_ADD_SOURCE"
	envServiceName  = "SERVICE_NAME"
	envEnvironment  = "ENV"
)

const defaultServiceName = "aurora"

func configureLogging(w io.Writer, isTTY bool) error {
	level, err := parseLogLevel(os.Getenv(envLogLevel))
	if err != nil {
		return err
	}

	handler := newLogHandler(w, isTTY, os.Getenv(envLogFormat), level)

	attrs := addDefaultAttrs()
	logger := slog.New(handler)
	if len(attrs) > 0 {
		logger = logger.With(attrs...)
	}

	slog.SetDefault(logger)
	return nil
}

func addDefaultAttrs() []any {
	var attrs []any

	if name := os.Getenv(envServiceName); name != "" {
		attrs = append(attrs, slog.String("service", name))
	} else {
		attrs = append(attrs, slog.String("service", defaultServiceName))
	}

	if env := os.Getenv(envEnvironment); env != "" {
		attrs = append(attrs, slog.String("env", env))
	} else {
		attrs = append(attrs, slog.String("env", "development"))
	}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		attrs = append(attrs, slog.String("host", hostname))
	}

	return attrs
}

func newLogHandler(w io.Writer, isTTY bool, format string, level slog.Level) slog.Handler {
	format = strings.ToLower(strings.TrimSpace(format))
	if (isTTY && format != "json") || format == "text" {
		return tint.NewHandler(w, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
			NoColor:    !isTTY,
		})
	}

	addSource := false
	if raw := os.Getenv(envLogAddSource); raw != "" {
		addSource, _ = strconv.ParseBool(raw)
	}
	if level == slog.LevelDebug {
		addSource = true
	}

	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	})
}

// ── Startup phase helpers ─────────────────────────────────────────────────

const (
	phaseColor  = "\033[38;5;243m"
	phaseLabel  = "\033[38;5;106m"
	summaryLine = "──────────────────────────────────────────────────────────────"
)

func initTerminal() {
	isColoredTerm = term.IsTerminal(int(os.Stderr.Fd()))
}

func printLogoWithVersion() {
	fmt.Print(AuroraLogo)
	bi := version.Current()
	if isColoredTerm {
		commit := bi.Commit
		if len(commit) > 7 {
			commit = commit[:7]
		}
		fmt.Fprintf(os.Stderr, "  \033[38;5;106mv%s\033[0m  \033[38;5;243m│\033[0m  %s  \033[38;5;243m│\033[0m  commit %s  \033[38;5;243m│\033[0m  %s\n\n",
			bi.Version, bi.GoVersion, commit, bi.Date)
	} else {
		fmt.Fprintf(os.Stderr, "  v%s | %s | commit %s | %s\n\n",
			bi.Version, bi.GoVersion, bi.Commit, bi.Date)
	}
}

func printPhase(label string) {
	if isColoredTerm {
		fmt.Fprintf(os.Stderr, "\n%s─── %s%s %s%s\n", phaseColor, phaseLabel, label, phaseColor, strings.Repeat("─", 50-len(label)))
	} else {
		fmt.Fprintf(os.Stderr, "\n--- %s\n", label)
	}
}

func printStartupSummary(started time.Time, addr string, features map[string]string) {
	elapsed := time.Since(started).Round(time.Millisecond).String()

	if isColoredTerm {
		fmt.Fprintf(os.Stderr, "\n  \033[38;5;106m●\033[0m \033[1mAurora Gateway %s\033[0m  —  \033[38;5;243mready in %s\033[0m\n", version.Version, elapsed)
		fmt.Fprintf(os.Stderr, "  %s\n", summaryLine)
		fmt.Fprintf(os.Stderr, "  \033[38;5;243mListening on\033[0m    \033[38;5;39mhttp://0.0.0.0%s\033[0m\n", addr)
		fmt.Fprintf(os.Stderr, "  \033[38;5;243mDashboard\033[0m      \033[38;5;39mhttp://localhost%s/admin/dashboard\033[0m\n", addr)
		if len(features) > 0 {
			fmt.Fprintf(os.Stderr, "  %s\n", summaryLine)
			for k, v := range features {
				fmt.Fprintf(os.Stderr, "  \033[38;5;243m%-16s\033[0m %s\n", k, v)
			}
		}
		fmt.Fprintf(os.Stderr, "  %s\n\n", summaryLine)
	} else {
		fmt.Fprintf(os.Stderr, "\n  * Aurora Gateway %s -- ready in %s\n", version.Version, elapsed)
		fmt.Fprintf(os.Stderr, "  Listening on  http://0.0.0.0%s\n", addr)
		fmt.Fprintf(os.Stderr, "  Dashboard     http://localhost%s/admin/dashboard\n", addr)
		if len(features) > 0 {
			for k, v := range features {
				fmt.Fprintf(os.Stderr, "  %-16s %s\n", k, v)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
}

// LogError records a structured error with stack trace for operator observability.
// Use for internal server errors where root-cause attribution matters.
func LogError(msg string, err error, extra ...any) {
	if err == nil {
		return
	}
	attrs := make([]any, 0, 4+len(extra))
	attrs = append(attrs, slog.Group("error",
		"message", err.Error(),
		"stack", string(debug.Stack()),
	))
	attrs = append(attrs, extra...)
	slog.Error(msg, attrs...)
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info", "inf":
		return slog.LevelInfo, nil
	case "debug", "dbg":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error", "err":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid %s %q: supported values are debug, info, warn, error", envLogLevel, raw)
	}
}
