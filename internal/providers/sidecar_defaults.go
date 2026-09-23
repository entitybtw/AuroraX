package providers

import (
	"encoding/json"
	"os"
	"strings"
)

// SidecarOAuthDefaults are extension-supplied device-flow values loaded from
// the sidecar overrides JSON (written when an extension is applied).
type SidecarOAuthDefaults struct {
	OAuthServer   string
	OAuthClientID string
	BaseURL       string
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
		OAuthServer   string `json:"oauth_server"`
		OAuthClientID string `json:"oauth_client_id"`
		BaseURL       string `json:"base_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return SidecarOAuthDefaults{}
	}
	return SidecarOAuthDefaults{
		OAuthServer:   strings.TrimSpace(raw.OAuthServer),
		OAuthClientID: strings.TrimSpace(raw.OAuthClientID),
		BaseURL:       strings.TrimSpace(raw.BaseURL),
	}
}
