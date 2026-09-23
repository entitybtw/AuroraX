package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSidecarOAuthDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sidecar-overrides.json")
	body := `{"oauth_server":"https://auth.example.test","oauth_client_id":"cli-1","base_url":"https://up.example/v1"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", path)

	got := LoadSidecarOAuthDefaults()
	if got.OAuthServer != "https://auth.example.test" ||
		got.OAuthClientID != "cli-1" ||
		got.BaseURL != "https://up.example/v1" {
		t.Fatalf("defaults = %+v", got)
	}

	t.Setenv("AURORA_SIDECAR_OVERRIDES_PATH", filepath.Join(dir, "missing.json"))
	if zeros := LoadSidecarOAuthDefaults(); zeros.OAuthServer != "" || zeros.BaseURL != "" {
		t.Fatalf("missing file should be zero: %+v", zeros)
	}
}
