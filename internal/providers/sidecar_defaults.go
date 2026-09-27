package providers

import (
	"encoding/json"
	"os"
	"strings"
)

// SidecarDefaults are extension-supplied sidecar values loaded from the
// sidecar overrides JSON (written when an extension is applied).
type SidecarDefaults struct {
	BaseURL string
}

// LoadSidecarDefaults reads sidecar-overrides.json. Empty path env falls
// back to configs/sidecar-overrides.json. Missing/invalid files yield zeros.
func LoadSidecarDefaults() SidecarDefaults {
	path := strings.TrimSpace(os.Getenv("AURORA_SIDECAR_OVERRIDES_PATH"))
	if path == "" {
		path = "configs/sidecar-overrides.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SidecarDefaults{}
	}
	var raw struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return SidecarDefaults{}
	}
	return SidecarDefaults{
		BaseURL: strings.TrimSpace(raw.BaseURL),
	}
}
