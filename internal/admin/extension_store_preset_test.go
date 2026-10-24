package admin

import (
	"encoding/json"
	"os"
	"testing"
)

// The store ships the opencode preset, and the gateway has to accept it as
// written. The preset is the reverse engineered description of the real
// upstream client: its identity headers, its trace headers and its tool
// schema. A change in the preset that the validator rejects would break the
// installation of the extension.
func TestOpenCodePresetFromStoreIsValid(t *testing.T) {
	const path = "/root/projects/aurorax-store/extensions/opencode.extension.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("store preset not available: %v", err)
	}
	var ext Extension
	if err := json.Unmarshal(data, &ext); err != nil {
		t.Fatalf("unmarshal preset: %v", err)
	}
	if err := ext.Validate(); err != nil {
		t.Fatalf("store preset is rejected by the gateway: %v", err)
	}
}
