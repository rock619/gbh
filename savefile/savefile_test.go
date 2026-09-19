package savefile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	tests := []struct {
		name    string
		romPath string
		want    string
	}{
		{name: "GB ROM", romPath: "/games/pokemon.gb", want: "/games/pokemon.sav"},
		{name: "GBC ROM", romPath: "/games/zelda.gbc", want: "/games/zelda.sav"},
		{name: "ROM without extension", romPath: "/games/tetris", want: "/games/tetris.sav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultPath(tt.romPath); got != tt.want {
				t.Fatalf("DefaultPath(): got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.sav")

	ram, err := Read(path)
	if err != nil {
		t.Fatalf("Read() for missing file: %v", err)
	}
	if ram != nil {
		t.Fatalf("Read() for missing file: got %d bytes, want nil", len(ram))
	}

	want := []byte{0x12, 0x34, 0x56}
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("write test save: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read(): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Read(): got %v, want %v", got, want)
	}
}

func TestReadReturnsOtherErrors(t *testing.T) {
	path := t.TempDir()
	_, err := Read(path)
	if err == nil {
		t.Fatal("Read() for directory: got nil error, want error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("Read() error %q does not contain path %q", err, path)
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.sav")

	first := []byte{0x12, 0x34}
	if err := Write(path, first); err != nil {
		t.Fatalf("Write() new file: %v", err)
	}
	assertFileContents(t, path, first)

	second := []byte{0x56, 0x78, 0x9A}
	if err := Write(path, second); err != nil {
		t.Fatalf("Write() replacement: %v", err)
	}
	assertFileContents(t, path, second)
}

func TestWriteCleansUpTemporaryFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.sav")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create destination directory: %v", err)
	}

	if err := Write(path, []byte{0x12}); err == nil {
		t.Fatal("Write() over directory: got nil error, want error")
	}

	temporaryFiles, err := filepath.Glob(filepath.Join(dir, ".game.sav.tmp-*"))
	if err != nil {
		t.Fatalf("find temporary save files: %v", err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("Write() left temporary files: %v", temporaryFiles)
	}
}

func assertFileContents(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("contents of %q: got %v, want %v", path, got, want)
	}
}
