package sessionhub

import (
	"encoding/json"
	"os"
)

// atomicWrite writes data to path via a temp file + rename so concurrent
// readers never observe a partially-written rules file.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

// writeSessionsJSON persists a slice of session entries atomically to disk.
// A nil/empty slice produces an empty JSON array, not an absent file.
func writeSessionsJSON(path string, entries []*SessionEntry) error {
	if entries == nil {
		entries = []*SessionEntry{}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return atomicWrite(path, raw)
}

// readSessionsJSON loads session entries previously written by
// writeSessionsJSON. A missing file yields an empty slice with nil error.
func readSessionsJSON(path string) ([]*SessionEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []*SessionEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
