package cartridge

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestCartMetadata(t *testing.T) {
	tests := []struct {
		name        string
		cartType    Type
		ramCode     uint8
		wantRAM     int
		wantBattery bool
	}{
		{name: "ROM only", cartType: TypeROMOnly, ramCode: 0x00, wantRAM: 0},
		{name: "MBC1 RAM", cartType: TypeMBC1RAM, ramCode: 0x02, wantRAM: 8 * Ki},
		{name: "MBC1 RAM battery", cartType: TypeMBC1RAMBattery, ramCode: 0x03, wantRAM: 32 * Ki, wantBattery: true},
		{name: "unsupported MBC3 RAM battery", cartType: TypeMBC3RAMBattery, ramCode: 0x03, wantRAM: 32 * Ki},
		{name: "MBC5 RAM battery", cartType: TypeMBC5RAMBattery, ramCode: 0x04, wantRAM: 128 * Ki, wantBattery: true},
		{name: "MBC5 rumble RAM battery", cartType: TypeMBC5RumbleRAMBattery, ramCode: 0x05, wantRAM: 64 * Ki, wantBattery: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rom := make([]uint8, 2*bytesPerROMBank)
			rom[0x0147] = uint8(tt.cartType)
			rom[0x0148] = 0x00
			rom[0x0149] = tt.ramCode

			cart := newCart(rom, nil, slog.Default())
			if got := cart.Type(); got != tt.cartType {
				t.Fatalf("Type(): got %#02x, want %#02x", got, tt.cartType)
			}
			if got := cart.RAMSize(); got != tt.wantRAM {
				t.Fatalf("RAMSize(): got %d, want %d", got, tt.wantRAM)
			}
			if got := cart.HasBattery(); got != tt.wantBattery {
				t.Fatalf("HasBattery(): got %t, want %t", got, tt.wantBattery)
			}
		})
	}
}

func TestImplementedCartridgesExposeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		cartType Type
	}{
		{name: "no MBC", cartType: TypeROMOnly},
		{name: "MBC1", cartType: TypeMBC1RAMBattery},
		{name: "MBC5", cartType: TypeMBC5RAMBattery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rom := make([]uint8, 2*bytesPerROMBank)
			rom[0x0147] = uint8(tt.cartType)
			rom[0x0148] = 0x00
			var cart Cartridge = New(rom, nil, slog.Default())
			if got := cart.Type(); got != tt.cartType {
				t.Fatalf("Type(): got %#02x, want %#02x", got, tt.cartType)
			}
		})
	}
}

func TestSaveRAMReturnsRAMInBankOrder(t *testing.T) {
	rom := make([]uint8, 2*bytesPerROMBank)
	rom[0x0147] = uint8(TypeMBC1RAMBattery)
	rom[0x0148] = 0x00
	rom[0x0149] = 0x03

	want := make([]uint8, 4*bytesPerRAMBank)
	for i := range want {
		want[i] = uint8(i / bytesPerRAMBank)
	}

	cart := newCart(rom, want, slog.Default())
	got := cart.SaveRAM()
	if !bytes.Equal(got, want) {
		t.Fatal("SaveRAM() did not preserve RAM bank order")
	}
}

func TestSaveRAMReturnsCopy(t *testing.T) {
	rom := make([]uint8, 2*bytesPerROMBank)
	rom[0x0147] = uint8(TypeMBC1RAMBattery)
	rom[0x0148] = 0x00
	rom[0x0149] = 0x02

	cart := newCart(rom, []uint8{0x12}, slog.Default())
	saved := cart.SaveRAM()
	saved[0] = 0x34

	if got := cart.rams[0][0]; got != 0x12 {
		t.Fatalf("SaveRAM() exposed internal RAM: got %#02x, want 0x12", got)
	}
}

func TestSaveRAMWithoutRAMReturnsNil(t *testing.T) {
	rom := make([]uint8, 2*bytesPerROMBank)
	rom[0x0147] = uint8(TypeROMOnly)
	rom[0x0148] = 0x00
	rom[0x0149] = 0x00

	cart := newCart(rom, nil, slog.Default())
	if got := cart.SaveRAM(); got != nil {
		t.Fatalf("SaveRAM(): got %d bytes, want nil", len(got))
	}
}
