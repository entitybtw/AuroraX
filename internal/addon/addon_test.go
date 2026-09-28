package addon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAddon(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStoreLoadAndCall(t *testing.T) {
	dir := t.TempDir()
	writeAddon(t, dir, "theme-sample.go", `// addon-kind: theme
package main

func Accent() string {
	return "#ff00aa"
}

func Greet(name string) string {
	return "hello " + name
}
`)

	s := NewStore(dir)
	loaded := s.Load()
	if len(loaded) != 1 || loaded[0] != "theme-sample" {
		t.Fatalf("loaded = %#v", loaded)
	}

	a := s.Get("theme-sample")
	if a == nil {
		t.Fatal("addon not found")
	}
	if a.Kind != KindTheme {
		t.Fatalf("kind = %q", a.Kind)
	}
	if !a.Has("Accent") {
		t.Fatal("missing Accent export")
	}
	out, err := a.CallString("Greet", "aurora")
	if err != nil {
		t.Fatalf("CallString: %v", err)
	}
	if out != "hello aurora" {
		t.Fatalf("out = %q", out)
	}

	st := s.Status()
	if st.Dir != dir || len(st.Loaded) != 1 || st.Kinds["theme-sample"] != "theme" {
		t.Fatalf("status = %+v", st)
	}
}

func TestStoreRejectsEscapingPath(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "evil.go")
	if err := os.WriteFile(outside, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)

	s := NewStore(dir)
	if err := s.LoadFile(outside); err == nil {
		t.Fatal("expected escape error")
	}
	// Explicit .. relative path
	rel := filepath.Join(dir, "..", filepath.Base(outside))
	if err := s.LoadFile(rel); err == nil {
		t.Fatal("expected escape error for relative ..")
	}
}

func TestStoreEvalErrorRecorded(t *testing.T) {
	dir := t.TempDir()
	writeAddon(t, dir, "broken.go", `package main

func (((
`)
	s := NewStore(dir)
	if loaded := s.Load(); len(loaded) != 0 {
		t.Fatalf("loaded = %#v", loaded)
	}
	st := s.Status()
	if st.Errors["broken"] == "" {
		t.Fatalf("expected error for broken, status = %+v", st)
	}
	if len(st.Loaded) != 0 {
		t.Fatalf("loaded = %#v", st.Loaded)
	}
}

func TestKindFromFilename(t *testing.T) {
	if k := kindFromSource("package main\n", "preset-retry"); k != KindPreset {
		t.Fatalf("kind = %q", k)
	}
	if k := kindFromSource("package main\n", "auth-pkce"); k != KindAuth {
		t.Fatalf("kind = %q", k)
	}
}

func TestByKind(t *testing.T) {
	dir := t.TempDir()
	writeAddon(t, dir, "auth-one.go", "package main\nfunc Ok() string { return \"1\" }\n")
	writeAddon(t, dir, "ui-two.go", "package main\nfunc Ok() string { return \"2\" }\n")
	s := NewStore(dir)
	s.Load()
	if got := s.ByKind(KindAuth); len(got) != 1 || got[0].Name != "auth-one" {
		t.Fatalf("auth = %#v", got)
	}
	if got := s.ByKind(KindUI); len(got) != 1 || got[0].Name != "ui-two" {
		t.Fatalf("ui = %#v", got)
	}
}

func TestNonGoFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	writeAddon(t, dir, "notes.txt", "not go")
	writeAddon(t, dir, "ok.go", "package main\nfunc Hi() string { return \"hi\" }\n")
	s := NewStore(dir)
	loaded := s.Load()
	if len(loaded) != 1 || !strings.Contains(loaded[0], "ok") {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestAuthAddonContributesUISettingsTab(t *testing.T) {
	dir := t.TempDir()
	writeAddon(t, dir, "auth-sample.go", `// addon-kind: auth
package main

func UI() string {
	return "{\"settings_tabs\":[{\"id\":\"auth-x\",\"label\":\"Auth X\"}]}"
}
`)

	s := NewStore(dir)
	s.Load()

	byKind := s.ByKind(KindAuth)
	if len(byKind) != 1 {
		t.Fatalf("auth addons = %d, want 1", len(byKind))
	}
	raw, err := byKind[0].CallString("UI")
	if err != nil {
		t.Fatalf("UI() error: %v", err)
	}
	if !strings.Contains(raw, `"id":"auth-x"`) {
		t.Fatalf("UI() = %s, want settings tab auth-x", raw)
	}
}

func TestAddDirLoadsExtensionShippedAddon(t *testing.T) {
	primary := t.TempDir()
	extDir := t.TempDir()
	writeAddon(t, extDir, "auth-ext.go", `// addon-kind: auth
package main

func UI() string { return ` + "`" + `{"settings_tabs":[]}` + "`" + ` }
`)

	s := NewStore(primary)
	s.Load()
	if len(s.List()) != 0 {
		t.Fatalf("primary store not empty: %v", s.List())
	}

	s.AddDir(extDir)
	if s.Get("auth-ext") == nil {
		t.Fatalf("auth-ext not loaded from added dir: %v", s.List())
	}
	if len(s.Dirs()) != 2 {
		t.Fatalf("dirs = %v, want primary + extDir", s.Dirs())
	}

	// Re-adding the same dir is idempotent.
	s.AddDir(extDir)
	if len(s.Dirs()) != 2 {
		t.Fatalf("duplicate dir registered: %v", s.Dirs())
	}
}

// Disabling an extension passes a relative config dir (configs/extensions/<id>)
// while Addon.Path is absolute — RemoveDir must resolve it, otherwise the
// addon stays loaded and the disabled extension keeps its settings tabs.
func TestRemoveDirWithRelativePathUnloadsAddon(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	relDir := filepath.Join("configs", "extensions", "rel-ext")
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAddon(t, relDir, "auth-rel.go", `// addon-kind: auth
package main

func UI() string { return ` + "`" + `{"settings_tabs":[{"id":"tab","label":"Tab"}]}` + "`" + ` }
`)

	s := NewStore(filepath.Join("configs", "addons"))
	s.AddDir(relDir)
	if s.Get("auth-rel") == nil {
		t.Fatalf("auth-rel not loaded: %v", s.List())
	}

	s.RemoveDir(relDir)
	if s.Get("auth-rel") != nil {
		t.Fatalf("auth-rel still loaded after RemoveDir(%q): %v", relDir, s.List())
	}
	for _, d := range s.Dirs() {
		if filepath.Clean(d) == filepath.Clean(relDir) {
			t.Fatalf("dir %q still registered after RemoveDir", d)
		}
	}

	// Addon names must stay unique per store; a second extension dir with the
	// same file name reloads cleanly after the first was removed.
	s.AddDir(relDir)
	if s.Get("auth-rel") == nil {
		t.Fatalf("auth-rel did not reload after re-add: %v", s.List())
	}
}
