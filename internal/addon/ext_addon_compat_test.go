package addon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestLoadExtensionAddonSources exercises the exact addon sources shipped by
// the store. It is skipped unless AURORA_EXT_ADDON_DIR points at them, so the
// gateway's own test run stays self-contained while the store can still verify
// its addons compile under Yaegi.
func TestLoadExtensionAddonSources(t *testing.T) {
	dir := os.Getenv("AURORA_EXT_ADDON_DIR")
	if dir == "" {
		t.Skip("AURORA_EXT_ADDON_DIR not set")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}

	s := NewStore(abs)
	loaded := s.Load()
	if len(loaded) == 0 {
		t.Fatalf("no addons loaded from %s: %+v", abs, s.Status())
	}
	if errs := s.Status().Errors; len(errs) > 0 {
		t.Fatalf("Yaegi failed to compile the addon: %+v", errs)
	}

	a := s.Get("vpn-runtime")
	if a == nil {
		t.Fatalf("vpn-runtime not loaded: %v", loaded)
	}
	if a.Kind != KindRuntime {
		t.Fatalf("kind = %q, want runtime", a.Kind)
	}

	// Report-only mode must contribute no exits, and must stay valid JSON.
	out, err := a.CallString("EgressCandidates", `{"provider":"zen","core_kind":"none"}`)
	if err != nil {
		t.Fatalf("EgressCandidates: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("EgressCandidates returned non-JSON: %v (%s)", err, out)
	}

	// Data is what the dashboard polls; it must always be well-formed.
	data, err := a.CallString("Data", `{"key":"status","dir":`+strconv.Quote(filepath.Join(abs, "state"))+`}`)
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if !json.Valid([]byte(data)) {
		t.Fatalf("Data returned non-JSON: %s", data)
	}

	if _, err := a.CallString("OnInit", `{"dir":`+strconv.Quote(abs)+`}`); err != nil {
		t.Fatalf("OnInit: %v", err)
	}
}
