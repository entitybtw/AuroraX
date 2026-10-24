package sessionhub

import (
	"crypto/rand"
	"net/http"
	"strings"
)

// Transform applies provider-specific header rules to an http.Header map.
// It mutates headers in place. Returns the set of headers that were
// generated/mapped so callers can inspect what happened.
func Transform(headers http.Header, provider string, rule ProviderRule, store *Store) map[string]string {
	if !rule.Enabled && len(rule.Headers) > 0 {
		return nil
	}

	result := make(map[string]string)

	// The two tracing headers of the emulated client are derived from one
	// trace context: the same trace id and the same span id in either format.
	// The context is created when the first of them is generated.
	var trace *TraceContext
	// The session headers of the emulated client all carry the same value:
	// the client has one session id and repeats it in every header it sends
	// it in. A gateway that gives each header its own id looks different
	// from the client it emulates, and the upstream can tell.
	var session *sessionValue

	for _, hr := range rule.Headers {
		name := http.CanonicalHeaderKey(hr.Name)
		inbound := headers.Get(name)

		switch hr.Mode {
		case HeaderModeMap:
			if inbound == "" {
				continue
			}
			prefix := hr.Prefix
			if prefix == "" {
				prefix = "ses_"
			}
			length := hr.Length
			if length <= 0 {
				length = 28
			}
			outbound := store.GetOrCreate(provider, inbound, prefix, length, hr.Charset)
			headers.Set(name, outbound)
			result[name] = outbound

		case HeaderModeMapOrGenerate:
			prefix := hr.Prefix
			if prefix == "" {
				prefix = "ses_"
			}
			length := hr.Length
			if length <= 0 {
				length = 28
			}
			var outbound string
			if inbound != "" {
				outbound = store.GetOrCreate(provider, inbound, prefix, length, hr.Charset)
			} else {
				outbound = generateRuleValue(hr, prefix, length, &trace, &session)
			}
			headers.Set(name, outbound)
			result[name] = outbound

		case HeaderModeGenerate:
			prefix := hr.Prefix
			if prefix == "" {
				prefix = "ses_"
			}
			length := hr.Length
			if length <= 0 {
				length = 28
			}
			outbound := generateRuleValue(hr, prefix, length, &trace, &session)
			headers.Set(name, outbound)
			result[name] = outbound

		case HeaderModePassthrough:
			if inbound != "" {
				result[name] = inbound
			}

		case HeaderModeStatic:
			headers.Set(name, hr.Value)
			result[name] = hr.Value

		case HeaderModeRandomFromList:
			if len(hr.Values) > 0 {
				idx := randomIndex(len(hr.Values))
				val := hr.Values[idx]
				headers.Set(name, val)
				result[name] = val
			}

		case HeaderModeRemove:
			headers.Del(name)
		}
	}

	return result
}

func randomIndex(n int) int {
	if n <= 0 {
		return 0
	}
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0]) % n
}

// sessionValue is the session id of one request, shared by every session
// header of that request.
type sessionValue struct {
	value string
}

// generateRuleValue produces the value of a generated header rule. A rule that
// names a template uses that template's generator (trace contexts, project id,
// the shared session id); every other rule falls back to a random value of the
// requested shape.
//
// The trace context and the session id are passed in and created on demand,
// so the tracing headers of one request share their trace id and their span
// id, and the session headers of one request share their value.
func generateRuleValue(hr HeaderRule, prefix string, length int, trace **TraceContext, session **sessionValue) string {
	if hr.Template != "" {
		if strings.EqualFold(strings.TrimSpace(hr.Template), "session") {
			if session != nil {
				if *session == nil {
					v := sessionValue{value: GenerateValue(prefix, length, hr.Charset)}
					*session = &v
				}
				return (*session).value
			}
			return GenerateValue(prefix, length, hr.Charset)
		}
		if v, ok := generateTemplateValue(hr.Template, trace); ok {
			return v
		}
	}
	return GenerateValue(prefix, length, hr.Charset)
}

// ExtractSession extracts session-related values from headers.
func ExtractSession(headers http.Header, captureHeaders []string) map[string]string {
	result := make(map[string]string)
	for _, h := range captureHeaders {
		canonical := http.CanonicalHeaderKey(h)
		if v := headers.Get(canonical); v != "" {
			result[canonical] = v
		}
	}
	return result
}

// IsSessionHeader returns true if the header name looks like a session header.
func IsSessionHeader(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "session") || strings.Contains(lower, "token")
}
