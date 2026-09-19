package cartridge

import (
	"log/slog"
	"testing"
)

func TestNoMBCWithoutRAMReturnsFFForExternalRAM(t *testing.T) {
	rom := make([]uint8, bytesPerROMBank*2)
	rom[0x0147] = uint8(TypeROMOnly)
	rom[0x0148] = 0x00
	rom[0x0149] = 0x00

	cart := NewNoMBC(rom, nil, slog.Default())

	cart.Write(0xA000, 0x12)
	if got, want := cart.Read(0xA000), uint8(0xFF); got != want {
		t.Fatalf("external RAM read without RAM: got %02X, want %02X", got, want)
	}
}
