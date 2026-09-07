package providers

import "net/http"

// WrapHeaderSetterWithSessionHub wraps an existing headerSetter to also apply
// session hub transformations. The sessionHub transformer is called first,
// then the original setter is called.
func WrapHeaderSetterWithSessionHub(original func(req *http.Request), providerName string, sessionHub SessionHubTransformer) func(req *http.Request) {
	if sessionHub == nil {
		return original
	}
	return func(req *http.Request) {
		// Apply session hub transformations first
		sessionHub(providerName, req.Header)
		// Then apply provider-specific headers
		if original != nil {
			original(req)
		}
	}
}
