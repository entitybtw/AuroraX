package providers

import (
	"net/http"
	"strings"

	"aurora/internal/sessionhub"
)

// WrapHeaderSetterWithSessionHub wraps an existing headerSetter so session hub
// transformations are applied to the outbound request. Before transforming,
// any relevant inbound session headers forwarded on the request context are
// copied onto the outbound request so "map" / "map_or_generate" mode can
// resolve a stable per-provider value rather than only generating fresh ones.
func WrapHeaderSetterWithSessionHub(original func(req *http.Request), providerName string, sessionHub SessionHubTransformer) func(req *http.Request) {
	if sessionHub == nil {
		return original
	}
	return func(req *http.Request) {
		if req != nil {
			if inbound := sessionhub.InboundHeadersFrom(req.Context()); inbound != nil {
				for k, vv := range inbound {
					ck := http.CanonicalHeaderKey(k)
					if isSessionScopedHeader(ck) && req.Header.Get(ck) == "" {
						req.Header[ck] = append([]string(nil), vv...)
					}
				}
			}
		}
		sessionHub(providerName, req.Header)
		if original != nil {
			original(req)
		}
	}
}

// isSessionScopedHeader reports whether an inbound header name is safe to copy
// onto the upstream request. Only session/identity-scoped names are forwarded;
// credentials and envelope metadata are excluded to avoid leaking secrets.
func isSessionScopedHeader(name string) bool {
	switch name {
	case "X-Opencode-Session", "X-Session-Id", "X-Session", "X-Session-Token",
		"X-Conversation-Id", "X-Opencode-Client":
		return true
	}
	l := strings.ToLower(name)
	return strings.Contains(l, "session") && strings.HasPrefix(l, "x-")
}
