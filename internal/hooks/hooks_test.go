package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/addon"
)

func writeAddon(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newBridge wires a store built from src plus a fake extension owner so the
// payload enrichment path is exercised the same way app.go wires it.
func newBridge(t *testing.T, extID string, info ExtensionInfo, srcs map[string]string) *Bridge {
	t.Helper()
	dir := t.TempDir()
	for name, src := range srcs {
		writeAddon(t, dir, name, src)
	}
	store := addon.NewStore(dir)
	store.Load()

	b := NewBridge()
	b.SetStore(store)
	b.SetContext(
		// In these fixtures every addon belongs to the extension under test.
		func(string) string { return extID },
		func(id string) (ExtensionInfo, bool) {
			if id == extID {
				return info, true
			}
			return ExtensionInfo{}, false
		},
	)
	return b
}

const egressAddon = `// addon-kind: runtime
package main

func EgressResolve(payload string) string { return payload }
`

func TestCallEnrichesPayloadWithExtensionSettings(t *testing.T) {
	info := ExtensionInfo{
		ID:       "vpn-egress",
		Dir:      filepath.Join("configs", "extensions", "vpn-egress"),
		Settings: map[string]string{"strategy": "latency"},
		Config:   map[string]string{"refresh_interval_minutes": "30"},
		Applied:  true,
	}
	b := newBridge(t, "vpn-egress", info, map[string]string{"vpn-runtime.go": egressAddon})

	raw, err := b.Call(addon.KindRuntime, "EgressResolve", map[string]any{"provider": "zen"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("payload is not JSON: %v (%s)", err, raw)
	}
	if got["provider"] != "zen" {
		t.Errorf("provider = %v, want zen", got["provider"])
	}
	if got["extension"] != "vpn-egress" {
		t.Errorf("extension = %v, want vpn-egress", got["extension"])
	}
	if got["hook"] != "EgressResolve" {
		t.Errorf("hook = %v", got["hook"])
	}
	settings, _ := got["settings"].(map[string]any)
	if settings["strategy"] != "latency" {
		t.Errorf("settings = %v", got["settings"])
	}
	config, _ := got["config"].(map[string]any)
	if config["refresh_interval_minutes"] != "30" {
		t.Errorf("config = %v", got["config"])
	}
	if got["dir"] != info.Dir {
		t.Errorf("dir = %v, want %s", got["dir"], info.Dir)
	}
}

// An addon that does not export the hook must be skipped silently rather than
// turned into an error — that is what lets unrelated extensions coexist.
func TestCallSkipsAddonsMissingTheExport(t *testing.T) {
	info := ExtensionInfo{ID: "other", Dir: filepath.Join("configs", "extensions", "other"), Applied: true}
	b := newBridge(t, "other", info, map[string]string{
		"runtime-silent.go": "// addon-kind: runtime\npackage main\n\nfunc SomethingElse() string { return \"x\" }\n",
		"runtime-answer.go": "// addon-kind: runtime\npackage main\n\nfunc EgressResolve(payload string) string { return `{\"handled\":true}` }\n",
	})

	raw, err := b.Call(addon.KindRuntime, "EgressResolve", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if raw != `{"handled":true}` {
		t.Fatalf("raw = %s", raw)
	}
}

func TestCallReportsNotOwnedWhenNobodyExports(t *testing.T) {
	info := ExtensionInfo{ID: "x", Dir: filepath.Join("configs", "extensions", "x"), Applied: true}
	b := newBridge(t, "x", info, map[string]string{
		"runtime-none.go": "// addon-kind: runtime\npackage main\n\nfunc Other() string { return \"\" }\n",
	})
	if _, err := b.Call(addon.KindRuntime, "EgressResolve", nil); err == nil {
		t.Fatal("expected an error when no addon exports the hook")
	}
}

// {"skip": true} means "not my provider" — the bridge keeps looking.
func TestCallHonoursSkipReplies(t *testing.T) {
	info := ExtensionInfo{ID: "x", Dir: filepath.Join("configs", "extensions", "x"), Applied: true}
	b := newBridge(t, "x", info, map[string]string{
		"runtime-a.go": "// addon-kind: runtime\npackage main\n\nfunc EgressResolve(payload string) string { return `{\"skip\":true,\"error\":\"not mine\"}` }\n",
		"runtime-b.go": "// addon-kind: runtime\npackage main\n\nfunc EgressResolve(payload string) string { return `{\"proxy\":\"socks5://127.0.0.1:1080\"}` }\n",
	})
	raw, err := b.Call(addon.KindRuntime, "EgressResolve", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if raw != `{"proxy":"socks5://127.0.0.1:1080"}` {
		t.Fatalf("raw = %s", raw)
	}
}

// A disabled extension must stop serving hooks even if its addon file is
// still loaded (crash between unapply and unload).
func TestCallSkipsUnappliedExtension(t *testing.T) {
	info := ExtensionInfo{ID: "off", Dir: filepath.Join("configs", "extensions", "off"), Applied: false}
	b := newBridge(t, "off", info, map[string]string{"runtime-off.go": egressAddon})
	if _, err := b.Call(addon.KindRuntime, "EgressResolve", nil); err == nil {
		t.Fatal("expected the unapplied extension's addon to be skipped")
	}
}

func TestCallAllCollectsEveryAddon(t *testing.T) {
	info := ExtensionInfo{ID: "x", Dir: filepath.Join("configs", "extensions", "x"), Applied: true}
	b := newBridge(t, "x", info, map[string]string{
		"ui-one.go": "// addon-kind: ui\npackage main\n\nfunc Data(payload string) string { return `{\"blocks\":[{\"kind\":\"kv\"}]}` }\n",
		"ui-two.go": "// addon-kind: ui\npackage main\n\nfunc Data(payload string) string { return `{\"blocks\":[{\"kind\":\"list\"}]}` }\n",
	})
	res := b.CallAll(addon.KindUI, "Data", map[string]any{"key": "nodes"})
	if len(res) != 2 {
		t.Fatalf("results = %d, want 2: %+v", len(res), res)
	}
	for _, r := range res {
		if r.Error != "" {
			t.Errorf("%s: %s", r.Addon, r.Error)
		}
	}
}

// Fire is best-effort: a hook whose addon fails to answer must not propagate
// an error. Here the export takes two args while Fire passes one, so the call
// fails on arity.
func TestFireSwallowsAddonErrors(t *testing.T) {
	info := ExtensionInfo{ID: "x", Dir: filepath.Join("configs", "extensions", "x"), Applied: true}
	b := newBridge(t, "x", info, map[string]string{
		"runtime-boom.go": "// addon-kind: runtime\npackage main\n\nfunc OnTick(payload string, extra string) string { return payload + extra }\n",
	})
	b.Fire(addon.KindRuntime, "OnTick", nil)
}

func TestBridgeDisabledWithoutStore(t *testing.T) {
	b := NewBridge()
	if b.Enabled() {
		t.Fatal("empty bridge must report disabled")
	}
	if _, err := b.Call(addon.KindRuntime, "EgressResolve", nil); err != ErrDisabled {
		t.Fatalf("err = %v, want ErrDisabled", err)
	}
}
