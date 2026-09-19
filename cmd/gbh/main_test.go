package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSavePath(t *testing.T) {
	tests := []struct {
		name    string
		romPath string
		args    []string
		want    string
	}{
		{
			name:    "GB ROM",
			romPath: "/games/pokemon.gb",
			args:    []string{"gbh", "/games/pokemon.gb"},
			want:    "/games/pokemon.sav",
		},
		{
			name:    "GBC ROM",
			romPath: "/games/zelda.gbc",
			args:    []string{"gbh", "/games/zelda.gbc"},
			want:    "/games/zelda.sav",
		},
		{
			name:    "ROM without extension",
			romPath: "/games/tetris",
			args:    []string{"gbh", "/games/tetris"},
			want:    "/games/tetris.sav",
		},
		{
			name:    "explicit save path",
			romPath: "/games/pokemon.gb",
			args:    []string{"gbh", "/games/pokemon.gb", "/saves/pokemon.backup"},
			want:    "/saves/pokemon.backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSavePath(tt.romPath, tt.args); got != tt.want {
				t.Fatalf("resolveSavePath(): got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateSaveRAM(t *testing.T) {
	tests := []struct {
		name       string
		ram        []byte
		hasBattery bool
		ramSize    int
		wantError  bool
	}{
		{name: "missing save", ram: nil, hasBattery: true, ramSize: 8 * 1024},
		{name: "matching save", ram: make([]byte, 8*1024), hasBattery: true, ramSize: 8 * 1024},
		{name: "empty existing save", ram: make([]byte, 0), hasBattery: true, ramSize: 8 * 1024, wantError: true},
		{name: "short save", ram: make([]byte, 1), hasBattery: true, ramSize: 8 * 1024, wantError: true},
		{name: "invalid cartridge RAM size", ram: nil, hasBattery: true, ramSize: -1, wantError: true},
		{name: "non-battery cartridge", ram: make([]byte, 1), hasBattery: false, ramSize: 8 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSaveRAM(tt.ram, tt.hasBattery, tt.ramSize)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateSaveRAM() error = %v, wantError = %t", err, tt.wantError)
			}
		})
	}
}

type fakeBatteryRAM struct {
	hasBattery bool
	ramSize    int
	ram        []byte
}

func (r fakeBatteryRAM) HasBattery() bool { return r.hasBattery }
func (r fakeBatteryRAM) RAMSize() int     { return r.ramSize }
func (r fakeBatteryRAM) SaveRAM() []byte  { return r.ram }

func TestSaveBatteryRAM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.sav")
	want := []byte{0x12, 0x34, 0x56}
	ram := fakeBatteryRAM{
		hasBattery: true,
		ramSize:    len(want),
		ram:        want,
	}

	if err := saveBatteryRAM(path, ram); err != nil {
		t.Fatalf("saveBatteryRAM(): %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved RAM: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("saved RAM: got %v, want %v", got, want)
	}
}

func TestSaveBatteryRAMSkipsUnsupportedCartridge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.sav")
	ram := fakeBatteryRAM{ramSize: 1, ram: []byte{0x12}}

	if err := saveBatteryRAM(path, ram); err != nil {
		t.Fatalf("saveBatteryRAM(): %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("save file stat error = %v, want not exist", err)
	}
}

func TestSaveBatteryRAMSkipsCartridgeWithoutRAM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.sav")
	ram := fakeBatteryRAM{hasBattery: true, ramSize: 0}

	if err := saveBatteryRAM(path, ram); err != nil {
		t.Fatalf("saveBatteryRAM(): %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("save file stat error = %v, want not exist", err)
	}
}

func TestSaveBatteryRAMReturnsWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "game.sav")
	ram := fakeBatteryRAM{hasBattery: true, ramSize: 1, ram: []byte{0x12}}

	if err := saveBatteryRAM(path, ram); err == nil {
		t.Fatal("saveBatteryRAM(): got nil error, want write error")
	}
}
