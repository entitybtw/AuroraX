package modeldata

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// LoadFromFile reads and parses a models.json snapshot from disk.
//
// It returns (list, raw, nil) on success, or (nil, nil, nil) if path is empty
// or the file does not exist — making it safe to call before falling back to
// a network fetch. Any other read or parse failure is returned as an error.
func LoadFromFile(path string) (*ModelList, []byte, error) {
	if path == "" {
		return nil, nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading model list file %q: %w", path, err)
	}

	list, err := Parse(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing model list file %q: %w", path, err)
	}

	return list, raw, nil
}

// SaveToFile writes raw model-list JSON bytes to path atomically (temp file +
// rename), creating parent directories as needed. Used by `aurora models
// sync` to refresh the local snapshot from upstream.
func SaveToFile(path string, raw []byte) error {
	if path == "" {
		return fmt.Errorf("model list local path is empty")
	}

	tmp := path + ".tmp"
	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func parentDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			if i == 0 {
				return string(p[0])
			}
			return p[:i]
		}
	}
	return "."
}
