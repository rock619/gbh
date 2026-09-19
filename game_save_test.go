package gbh

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/rock619/gbh/cartridge"
)

func TestGameExposesCartridgeRAM(t *testing.T) {
	rom := make([]byte, 32*1024)
	rom[0x0147] = byte(cartridge.TypeMBC1RAMBattery)
	rom[0x0148] = 0x00
	rom[0x0149] = 0x02

	ram := make([]byte, 8*1024)
	for i := range ram {
		ram[i] = byte(i)
	}

	game := NewHeadlessGame(rom, ram, slog.Default())
	if !game.HasBattery() {
		t.Fatal("HasBattery(): got false, want true")
	}
	if got, want := game.RAMSize(), len(ram); got != want {
		t.Fatalf("RAMSize(): got %d, want %d", got, want)
	}
	if got := game.SaveRAM(); !bytes.Equal(got, ram) {
		t.Fatal("SaveRAM(): got data that differs from cartridge RAM")
	}
}
