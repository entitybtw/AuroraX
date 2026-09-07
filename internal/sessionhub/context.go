package sessionhub

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
)

type inboundCtxKey struct{}

// WithInboundHeaders stores a snapshot of inbound request headers on the context
// so the session hub can map client session identifiers onto stable per-provider
// outbound values even when the translated outbound request drops them.
func WithInboundHeaders(ctx context.Context, h http.Header) context.Context {
	if ctx == nil || h == nil {
		return ctx
	}
	clone := make(http.Header, len(h))
	for k, vv := range h {
		c := make([]string, len(vv))
		copy(c, vv)
		clone[k] = c
	}
	return context.WithValue(ctx, inboundCtxKey{}, clone)
}

// InboundHeadersFrom returns the inbound header snapshot stored on ctx, if any.
func InboundHeadersFrom(ctx context.Context) http.Header {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(inboundCtxKey{}).(http.Header); ok {
		return v
	}
	return nil
}

// CaptureInboundHeadersMiddleware stores a snapshot of the inbound request
// headers (before translation drops them) so the session hub can map
// session-scoped headers per provider.
func CaptureInboundHeadersMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			if req != nil && req.Header != nil {
				ctx := WithInboundHeaders(req.Context(), req.Header)
				c.SetRequest(req.WithContext(ctx))
			}
			return next(c)
		}
	}
}
