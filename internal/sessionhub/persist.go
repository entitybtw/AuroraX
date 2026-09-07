package sessionhub

import (
	"os"
	"path/filepath"
)

// atomicWrite writes data to path via a temp file + rename so concurrent
// readers never observe a partially-written rules file.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
