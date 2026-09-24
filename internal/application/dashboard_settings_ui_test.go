package app

import (
	"testing"

	"aurora/configuration"
	"aurora/internal/admin"
)

func TestNormalizeHiddenFeatures(t *testing.T) {
	got := normalizeHiddenFeatures([]string{" Pools ", "audit_logs", "POOLS", "", "pools"})
	want := []string{"pools", "audit_logs"}
	if len(got) != len(want) {
		t.Fatalf("normalizeHiddenFeatures = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeHiddenFeatures[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if normalizeHiddenFeatures(nil) != nil {
		t.Fatalf("normalizeHiddenFeatures(nil) must be nil")
	}
}

func TestApplyDashboardSettingsUIHiddenFeatures(t *testing.T) {
	cfg := &config.Config{}
	req := admin.DashboardSettingsUpdateRequest{}
	req.UI.HiddenFeatures = []string{"Pools", "audit_logs", "pools"}
	applyDashboardSettingsToConfig(cfg, req)
	if len(cfg.UI.HiddenFeatures) != 2 {
		t.Fatalf("cfg.UI.HiddenFeatures = %#v, want 2 ids", cfg.UI.HiddenFeatures)
	}
	if cfg.UI.HiddenFeatures[0] != "pools" || cfg.UI.HiddenFeatures[1] != "audit_logs" {
		t.Fatalf("cfg.UI.HiddenFeatures = %#v", cfg.UI.HiddenFeatures)
	}

	overlay := dashboardSettingsOverlay{}
	applyDashboardSettingsToOverlay(&overlay, req)
	if overlay.UI == nil {
		t.Fatalf("overlay.UI is nil")
	}
	if len(overlay.UI.HiddenFeatures) != 2 {
		t.Fatalf("overlay.UI.HiddenFeatures = %#v", overlay.UI.HiddenFeatures)
	}

	snap := dashboardSettingsSnapshot(cfg)
	if len(snap.UI.HiddenFeatures) != 2 {
		t.Fatalf("snapshot UI.HiddenFeatures = %#v", snap.UI.HiddenFeatures)
	}
	if snap.UI.HiddenFeatures[0] != "pools" {
		t.Fatalf("snapshot UI.HiddenFeatures[0] = %q", snap.UI.HiddenFeatures[0])
	}
}
