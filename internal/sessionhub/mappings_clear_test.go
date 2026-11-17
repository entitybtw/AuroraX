package sessionhub

import (
	"path/filepath"
	"testing"
)

// TestClearPersistedMappings_OnlyTouchesOneProvider keeps the per-provider
// delete honest: clearing opencode-zen must not drop another provider's rows.
func TestClearPersistedMappings_OnlyTouchesOneProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	entries := []*SessionEntry{
		{Provider: "opencode-zen", InboundValue: "a", OutboundValue: "ses_a"},
		{Provider: "opencode-zen", InboundValue: "b", OutboundValue: "ses_b"},
		{Provider: "other", InboundValue: "c", OutboundValue: "ses_c"},
	}
	if err := writeSessionsJSON(path, entries); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := New(&HubConfig{MappingStorage: "disk", MappingsPath: path})
	if err := h.ClearPersistedMappings("opencode-zen"); err != nil {
		t.Fatalf("ClearPersistedMappings: %v", err)
	}

	left, err := readSessionsJSON(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(left) != 1 || left[0].Provider != "other" {
		t.Fatalf("left = %+v, want only the other provider's row", left)
	}
}

// TestClearPersistedMappings_AllProviders covers the "clear everything" path.
func TestClearPersistedMappings_AllProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	entries := []*SessionEntry{
		{Provider: "opencode-zen", InboundValue: "a", OutboundValue: "ses_a"},
		{Provider: "other", InboundValue: "c", OutboundValue: "ses_c"},
	}
	if err := writeSessionsJSON(path, entries); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := New(&HubConfig{MappingStorage: "disk", MappingsPath: path})
	if err := h.ClearPersistedMappings(""); err != nil {
		t.Fatalf("ClearPersistedMappings: %v", err)
	}

	left, err := readSessionsJSON(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("left = %+v, want none", left)
	}
}

// TestClearPersistedMappings_MissingFileIsNotAnError: a hub that never wrote a
// mapping file must still answer the delete request successfully.
func TestClearPersistedMappings_MissingFileIsNotAnError(t *testing.T) {
	h := New(&HubConfig{MappingStorage: "disk", MappingsPath: filepath.Join(t.TempDir(), "absent.json")})
	if err := h.ClearPersistedMappings("opencode-zen"); err != nil {
		t.Fatalf("missing file should be a no-op, got %v", err)
	}
}

// TestClearPersistedMappings_SurvivesReload is the bug this fixes: after
// deleting the mappings, a restarted hub must not bring them back.
func TestClearPersistedMappings_SurvivesReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	entries := []*SessionEntry{{Provider: "opencode-zen", InboundValue: "a", OutboundValue: "ses_a"}}
	if err := writeSessionsJSON(path, entries); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := New(&HubConfig{MappingStorage: "disk", MappingsPath: path})
	if err := h.ClearPersistedMappings("opencode-zen"); err != nil {
		t.Fatalf("ClearPersistedMappings: %v", err)
	}

	reloaded := New(&HubConfig{MappingStorage: "disk", MappingsPath: path})
	if got := len(reloaded.Store().List()); got != 0 {
		t.Fatalf("reloaded hub holds %d mappings, want 0", got)
	}
}
