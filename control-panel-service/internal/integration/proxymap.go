package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path through a temp file and rename, so a
// concurrent reader (the proxy's path unit) never observes a partial
// write. It skips the write, and reports false, when path already holds
// exactly data. path's directory must already exist.
func WriteAtomic(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mbr-routes-*")
	if err != nil {
		return false, fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return false, fmt.Errorf("rename into %s: %w", path, err)
	}
	return true, nil
}
