package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSidecarDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sidecar-overrides.json")
	body := `{"base_url":"https://up.example/v1","ignored":"value"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", path)

	got := LoadSidecarDefaults()
	if got.BaseURL != "https://up.example/v1" {
		t.Fatalf("defaults = %+v", got)
	}

	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "missing.json"))
	if zeros := LoadSidecarDefaults(); zeros.BaseURL != "" {
		t.Fatalf("missing file should be zero: %+v", zeros)
	}
}
