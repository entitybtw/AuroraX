package providers

import (
	"encoding/json"
	"os"
	"strings"
)

// SidecarOAuthDefaults are extension-supplied OAuth values loaded from the
// sidecar overrides JSON (written when an extension is applied). Covers both
// the device flow and authorization-code + PKCE when present.
type SidecarOAuthDefaults struct {
	OAuthServer   string
	OAuthClientID string
	BaseURL       string
	// Authorization-code + PKCE (empty grant = device when server is set).
	OAuthGrant           string
	OAuthAuthorizeURL    string
	OAuthTokenURL        string
	OAuthTokenStyle      string
	OAuthScopes          string
	OAuthStateIsVerifier bool
	OAuthRedirectURI     string
}

// LoadSidecarOAuthDefaults reads sidecar-overrides.json. Empty path env falls
// back to configs/sidecar-overrides.json. Missing/invalid files yield zeros.
func LoadSidecarOAuthDefaults() SidecarOAuthDefaults {
	path := strings.TrimSpace(os.Getenv("AURORA_SIDECAR_OVERRIDES_PATH"))
	if path == "" {
		path = "configs/sidecar-overrides.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SidecarOAuthDefaults{}
	}
	var raw struct {
		OAuthServer          string `json:"oauth_server"`
		OAuthClientID        string `json:"oauth_client_id"`
		BaseURL              string `json:"base_url"`
		OAuthGrant           string `json:"oauth_grant"`
		OAuthAuthorizeURL    string `json:"oauth_authorize_url"`
		OAuthTokenURL        string `json:"oauth_token_url"`
		OAuthTokenStyle      string `json:"oauth_token_style"`
		OAuthScopes          string `json:"oauth_scopes"`
		OAuthStateIsVerifier bool   `json:"oauth_state_is_verifier"`
		OAuthRedirectURI     string `json:"oauth_redirect_uri"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return SidecarOAuthDefaults{}
	}
	return SidecarOAuthDefaults{
		OAuthServer:          strings.TrimSpace(raw.OAuthServer),
		OAuthClientID:        strings.TrimSpace(raw.OAuthClientID),
		BaseURL:              strings.TrimSpace(raw.BaseURL),
		OAuthGrant:           strings.TrimSpace(raw.OAuthGrant),
		OAuthAuthorizeURL:    strings.TrimSpace(raw.OAuthAuthorizeURL),
		OAuthTokenURL:        strings.TrimSpace(raw.OAuthTokenURL),
		OAuthTokenStyle:      strings.TrimSpace(raw.OAuthTokenStyle),
		OAuthScopes:          strings.TrimSpace(raw.OAuthScopes),
		OAuthStateIsVerifier: raw.OAuthStateIsVerifier,
		OAuthRedirectURI:     strings.TrimSpace(raw.OAuthRedirectURI),
	}
}
