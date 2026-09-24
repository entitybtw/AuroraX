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
