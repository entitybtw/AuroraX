package sessionhub

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

type inboundCtxKey struct{}

// isCaptureHeader reports whether an inbound header is session/identity scoped
// and relevant to the session hub. Filtering at capture keeps the snapshot tiny,
// so the per-request hot path stays cheap.
func isCaptureHeader(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "x-") && strings.Contains(lower, "session")
}

// WithInboundHeaders stores only the session-scoped inbound headers on ctx so
// translated outbound requests can map them onto stable per-provider values.
func WithInboundHeaders(ctx context.Context, h http.Header) context.Context {
	if ctx == nil || h == nil {
		return ctx
	}
	var clone http.Header
	for k, vv := range h {
		if !isCaptureHeader(k) {
			continue
		}
		if clone == nil {
			clone = make(http.Header, 2)
		}
		c := make([]string, len(vv))
		copy(c, vv)
		clone[k] = c
	}
	if clone == nil {
		return ctx
	}
	return context.WithValue(ctx, inboundCtxKey{}, clone)
}

// InboundHeadersFrom returns the inbound session-scoped header snapshot, if any.
func InboundHeadersFrom(ctx context.Context) http.Header {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(inboundCtxKey{}).(http.Header); ok {
		return v
	}
	return nil
}

// CaptureInboundHeadersMiddleware snapshots inbound session-scoped headers
// before translation drops them.
func CaptureInboundHeadersMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			if req != nil && req.Header != nil {
				hd := WithInboundHeaders(req.Context(), req.Header)
				if hd != req.Context() {
					c.SetRequest(req.WithContext(hd))
				}
			}
			return next(c)
		}
	}
}
