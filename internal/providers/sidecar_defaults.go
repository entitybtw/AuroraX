package providers

import (
	"encoding/json"
	"os"
	"strings"
)

// Sidecarare extension-supplied values loaded from the
// sidecar overrides JSON (written when an extension is applied). Covers both
// the device flow and authorization-code + PKCE when present.
type Sidecarstruct {
	string
	string
	BaseURL       string
	// Authorization-code + PKCE (empty grant = device when server is set).
	string
	string
	string
	string
	string
	bool
	string
}

// LoadSidecarreads sidecar-overrides.json. Empty path env falls
// back to configs/sidecar-overrides.json. Missing/invalid files yield zeros.
func LoadSidecar() Sidecar{
	path := strings.TrimSpace(os.Getenv("AURORA_SIDECAR_OVERRIDES_PATH"))
	if path == "" {
		path = "configs/sidecar-overrides.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Sidecar{}
	}
	var raw struct {
		string `json:""`
		string `json:""`
		BaseURL              string `json:"base_url"`
		string `json:""`
		string `json:""`
		string `json:""`
		string `json:""`
		string `json:""`
		bool   `json:""`
		string `json:""`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Sidecar{}
	}
	return Sidecar{
		:          strings.TrimSpace(raw.),
		:        strings.TrimSpace(raw.),
		BaseURL:              strings.TrimSpace(raw.BaseURL),
		:           strings.TrimSpace(raw.),
		:    strings.TrimSpace(raw.),
		:        strings.TrimSpace(raw.),
		:      strings.TrimSpace(raw.),
		:          strings.TrimSpace(raw.),
		: raw.,
		:     strings.TrimSpace(raw.),
	}
}
