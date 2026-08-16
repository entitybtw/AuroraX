package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyYAML_OverlayWithoutBaseConfig(t *testing.T) {
	tmp := t.TempDir()
	overlayDir := filepath.Join(tmp, "configs")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	overlayPath := filepath.Join(overlayDir, "dashboard-overrides.yaml")
	content := "response_headers:\n  enabled: true\n  actual_provider_header: true\n  custom_headers:\n    - name: X-Custom\n      value: \"{actual_model}\"\n      enabled: true\n"
	if err := os.WriteFile(overlayPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := &Config{}
	providers, pools, err := applyYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if providers == nil {
		t.Error("expected non-nil providers map")
	}
	if pools == nil {
		t.Error("expected non-nil pools map")
	}

	if !cfg.ResponseHeaders.Enabled {
		t.Error("expected response_headers.enabled to be true from overlay without base config")
	}
	if !cfg.ResponseHeaders.ActualProviderHeader {
		t.Error("expected actual_provider_header to be true from overlay")
	}
	if len(cfg.ResponseHeaders.CustomHeaders) != 1 {
		t.Fatalf("expected 1 custom header, got %d", len(cfg.ResponseHeaders.CustomHeaders))
	}
	if cfg.ResponseHeaders.CustomHeaders[0].Name != "X-Custom" {
		t.Errorf("expected custom header name X-Custom, got %q", cfg.ResponseHeaders.CustomHeaders[0].Name)
	}
}