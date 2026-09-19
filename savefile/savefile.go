package savefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns the default save path for a ROM path.
func DefaultPath(romPath string) string {
	ext := filepath.Ext(romPath)
	return strings.TrimSuffix(romPath, ext) + ".sav"
}

// Read reads a save file. A missing save file is not an error.
func Read(path string) ([]byte, error) {
	ram, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read save file %q: %w", path, err)
	}
	return ram, nil
}

// Write atomically replaces a save file with data.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary save file for %q: %w", path, err)
	}

	tempPath := file.Name()
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("set save file permissions: %w", err)
	}
	n, err := file.Write(data)
	if err != nil {
		return fmt.Errorf("write save file: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("write save file: %w", io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync save file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close save file: %w", err)
	}
	closed = true

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace save file %q: %w", path, err)
	}
	return nil
}
