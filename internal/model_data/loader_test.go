package modeldata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile_EmptyPath(t *testing.T) {
	list, raw, err := LoadFromFile("")
	if list != nil || raw != nil || err != nil {
		t.Errorf("expected all nil for empty path; got list=%v raw=%v err=%v", list, raw, err)
	}
}

func TestLoadFromFile_Missing(t *testing.T) {
	dir := t.TempDir()
	list, raw, err := LoadFromFile(filepath.Join(dir, "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file should not error; got %v", err)
	}
	if list != nil || raw != nil {
		t.Errorf("expected nil list/raw for missing file")
	}
}

func TestLoadFromFile_ParsesValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	body := []byte(`{
		"version": 1,
		"updated_at": "2026-05-08T00:00:00Z",
		"providers": {"openai": {"display_name": "OpenAI"}},
		"models": {"gpt-4o": {"display_name": "GPT-4o", "modes": ["chat"]}},
		"provider_models": {}
	}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	list, raw, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list == nil || raw == nil {
		t.Fatal("expected non-nil list and raw")
	}
	if list.Models["gpt-4o"].DisplayName != "GPT-4o" {
		t.Errorf("model not parsed; got %+v", list.Models)
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadFromFile(path); err == nil {
		t.Error("expected parse error")
	}
}

func TestSaveToFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "models.json")
	body := []byte(`{"version":1,"providers":{},"models":{},"provider_models":{}}`)

	if err := SaveToFile(path, body); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("content mismatch: got %q want %q", got, body)
	}
	// Temp file must be cleaned up.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected temp file to be removed after rename")
	}
}

func TestSaveToFile_EmptyPath(t *testing.T) {
	if err := SaveToFile("", []byte("{}")); err == nil {
		t.Error("expected error for empty path")
	}
}
