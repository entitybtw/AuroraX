package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sidecar-overrides.json")
	body := `{"":"https://auth.example.test","":"cli-1","base_url":"https://up.example/v1"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", path)

	got := LoadSidecar()
	if got.!= "https://auth.example.test" ||
		got.!= "cli-1" ||
		got.BaseURL != "https://up.example/v1" {
		t.Fatalf("defaults = %+v", got)
	}

	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "missing.json"))
	if zeros := LoadSidecar(); zeros.!= "" || zeros.BaseURL != "" {
		t.Fatalf("missing file should be zero: %+v", zeros)
	}
}
