package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
	configpkg "aurora/configuration"
)

// handleError converts gateway errors to appropriate HTTP responses.
func handleError(c *echo.Context, err error) error {
	if gatewayErr, ok := errors.AsType[*core.GatewayError](err); ok {
		logHandledError(c, gatewayErr)
		auditlog.EnrichEntryWithError(c, string(gatewayErr.Type), gatewayErr.Message, gatewayErrorCode(gatewayErr))
		applyErrorResponseHeaders(c, err)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	gatewayErr := core.NewProviderError("", http.StatusInternalServerError, "an unexpected error occurred", err)
	logHandledError(c, gatewayErr)
	auditlog.EnrichEntryWithError(c, string(gatewayErr.Type), gatewayErr.Message, gatewayErrorCode(gatewayErr))
	return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
}

type responseHeaderError interface {
	ResponseHeaders() http.Header
}

func applyErrorResponseHeaders(c *echo.Context, err error) {
	if c == nil || err == nil {
		return
	}
	var headerErr responseHeaderError
	if !errors.As(err, &headerErr) {
		return
	}
	for key, values := range headerErr.ResponseHeaders() {
		for i, value := range values {
			if i == 0 {
				c.Response().Header().Set(key, value)
				continue
			}
			c.Response().Header().Add(key, value)
		}
	}
}

func gatewayErrorCode(err *core.GatewayError) string {
	if err == nil || err.Code == nil {
		return ""
	}
	return *err.Code
}

// handleErrorWithHeaders handles errors and adds response headers based on config.
func handleErrorWithHeaders(c *echo.Context, err error, cfg configpkg.ResponseHeadersConfig) error {
	if !cfg.Enabled {
		return handleError(c, err)
	}

	// First, call the original handleError to get the error response
	_ = handleError(c, err)

	// If response headers are enabled and mode allows error responses, add headers
	if !cfg.Enabled {
		return nil
	}

	status := 0
	if resp, ok := c.Response().(*echo.Response); ok {
		status = resp.Status
	}

	switch cfg.Mode {
	case "success":
		return nil // Don't add headers to error responses in success mode
	case "error":
		if status < 400 {
			return nil
		}
	case "always":
		// Add headers for all responses
	default:
		return nil
	}

	// Add error response headers
	header := c.Response().Header()
	requestID := requestIDFromContextOrHeader(c.Request())

	// Add request ID if available
	if requestID != "" {
		header.Set("X-Request-ID", requestID)
	}

	// Add custom headers with placeholders
	for _, custom := range cfg.CustomHeaders {
		if !custom.Enabled || strings.TrimSpace(custom.Name) == "" {
			continue
		}
		name := http.CanonicalHeaderKey(strings.TrimSpace(custom.Name))
		value := expandResponseHeaderTemplateForError(custom.Value, requestID)
		if name != "" && value != "" {
			header.Set(name, value)
		}
	}

	return nil
}

func logHandledError(c *echo.Context, gatewayErr *core.GatewayError) {
	if gatewayErr == nil {
		return
	}
	if minimalBenchModeEnabled() {
		return
	}

	errorGroup := []any{
		slog.String("type", string(gatewayErr.Type)),
		slog.Int("status", gatewayErr.HTTPStatusCode()),
		slog.String("message", gatewayErr.Message),
	}
	if gatewayErr.Provider != "" {
		errorGroup = append(errorGroup, slog.String("provider", gatewayErr.Provider))
	}
	if gatewayErr.Param != nil {
		errorGroup = append(errorGroup, slog.String("param", *gatewayErr.Param))
	}
	if gatewayErr.Code != nil {
		errorGroup = append(errorGroup, slog.String("code", *gatewayErr.Code))
	}
	if gatewayErr.Err != nil {
		errorGroup = append(errorGroup, slog.Any("cause", gatewayErr.Err))
	}

	attrs := []any{
		slog.Group("error", errorGroup...),
	}
	if c != nil && c.Request() != nil {
		req := c.Request()
		attrs = append(attrs,
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("request_id", requestIDFromContextOrHeader(req)),
		)
	}

	if gatewayErr.HTTPStatusCode() >= http.StatusInternalServerError {
		slog.Error("request failed", attrs...)
		return
	}
	slog.Warn("request failed", attrs...)
}
