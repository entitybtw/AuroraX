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
			hexLen := hr.Length
			if hexLen <= 0 {
				hexLen = 16
			}
			outbound := store.GetOrCreate(provider, inbound, prefix, hexLen)
			headers.Set(name, outbound)
			result[name] = outbound

		case HeaderModeGenerate:
			prefix := hr.Prefix
			if prefix == "" {
				prefix = "ses_"
			}
			hexLen := hr.Length
			if hexLen <= 0 {
				hexLen = 16
			}
			outbound := GenerateValue(prefix, hexLen)
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
