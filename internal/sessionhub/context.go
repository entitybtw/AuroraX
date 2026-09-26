package sessionhub

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
)

type inboundCtxKey struct{}

// identityHeaderNames holds exact header names treated as session/identity
// scoped. They are supplied by configuration (an applied extension's
// forward_headers) so the core never hardcodes a specific client's header
// names. Names are matched case-insensitively.
var (
	identityMu       sync.RWMutex
	identityNames    = map[string]bool{}
	identityPrefixes []string
)

// SetIdentityHeaders replaces the configured identity header set. Exact names
// are matched verbatim; prefixes match the lower-cased name prefix.
func SetIdentityHeaders(names []string, prefixes []string) {
	identityMu.Lock()
	defer identityMu.Unlock()
	set := make(map[string]bool, len(names)+4)
	for _, n := range defaultIdentityHeaders() {
		set[n] = true
	}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" {
			set[n] = true
		}
	}
	identityNames = set
	identityPrefixes = identityPrefixes[:0]
	for _, p := range prefixes {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			identityPrefixes = append(identityPrefixes, p)
		}
	}
}

// configuredIdentityLower reports whether name matches a configured identity
// header (exact name or prefix).
func configuredIdentityLower(lower string) bool {
	identityMu.RLock()
	defer identityMu.RUnlock()
	if identityNames[lower] {
		return true
	}
	for _, p := range identityPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// IsIdentityHeader reports whether an exact header name (case-insensitive)
// belongs to the configured identity set used by the session hub and the
// outbound request wrapper.
func IsIdentityHeader(name string) bool {
	return configuredIdentityLower(strings.ToLower(name))
}

// defaultIdentityHeaders are generic non-branded session/identity names that
// are always treated as identity scoped, independent of any extension.
func defaultIdentityHeaders() []string {
	return []string{"x-session-id", "x-client-id", "x-conversation-id"}
}

func init() {
	SetIdentityHeaders(nil, nil)
}

// isCaptureHeader reports whether an inbound header is session/identity scoped
// and relevant to the session hub. Filtering at capture keeps the snapshot tiny,
// so the per-request hot path stays cheap.
// Captures configured identity headers (extension-supplied), any x-session-*,
// and any x-*-session-* header.
func isCaptureHeader(name string) bool {
	lower := strings.ToLower(name)
	if configuredIdentityLower(lower) {
		return true
	}
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
