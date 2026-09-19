package cartridge

import (
	"bytes"
	"log/slog"
	"slices"

	"github.com/rock619/gbh/memorybus"
)

const (
	bytesPerROMBank = 16 * 1024
	bytesPerRAMBank = 8 * 1024
	Ki              = 1024
	Mi              = 1024 * Ki
)

// https://gbdev.io/pandocs/The_Cartridge_Header.html#0104-0133--nintendo-logo
var validLogo = []byte{
	0xCE, 0xED, 0x66, 0x66, 0xCC, 0x0D, 0x00, 0x0B, 0x03, 0x73, 0x00, 0x83, 0x00, 0x0C, 0x00, 0x0D,
	0x00, 0x08, 0x11, 0x1F, 0x88, 0x89, 0x00, 0x0E, 0xDC, 0xCC, 0x6E, 0xE6, 0xDD, 0xDD, 0xD9, 0x99,
	0xBB, 0xBB, 0x67, 0x63, 0x6E, 0x0E, 0xEC, 0xCC, 0xDD, 0xDC, 0x99, 0x9F, 0xBB, 0xB9, 0x33, 0x3E,
}

type Cartridge interface {
	memorybus.Memory

	Type() Type
	RAMSize() int
	HasBattery() bool
	SaveRAM() []uint8
	HasValidLogo() bool
	Title() string
}

func New(rom, ram []uint8, logger *slog.Logger) Cartridge {
	switch cartType := Type(rom[0x0147]); cartType {
	case TypeROMOnly:
		// ROM ONLY
		return NewNoMBC(rom, ram, logger)
	case TypeMBC1, TypeMBC1RAM, TypeMBC1RAMBattery:
		return NewMBC1(rom, ram, logger)
	case TypeMBC2, TypeMBC2Battery:
		// TODO: MBC2
		panic("MBC2 not implemented")
	case TypeMMM01, TypeMMM01RAM, TypeMMM01RAMBattery:
		// TODO: MMM01
		panic("MMM01 not implemented")
	case TypeMBC3, TypeMBC3TimerBattery, TypeMBC3TimerRAMBattery, TypeMBC3RAM, TypeMBC3RAMBattery:
		// TODO: MBC3
		// return NewMBC3(rom, ram, logger)
		panic("MBC3 not implemented")
	case TypeMBC5, TypeMBC5RAM, TypeMBC5RAMBattery, TypeMBC5Rumble, TypeMBC5RumbleRAM, TypeMBC5RumbleRAMBattery:
		// TODO: MBC5
		return NewMBC5(rom, ram, logger)
	case TypeMBC6:
		// TODO: MBC6
		panic("MBC6 not implemented")
	case TypeMBC7SensorRumbleRAMBattery:
		// TODO: MBC7
		panic("MBC7 not implemented")
	case TypePocketCamera, TypeBandaiTAMA5, TypeHudsonHuC3, TypeHudsonHuC1:
		// TODO: Other types
		panic("Unsupported cartridge type")
	default:
		// Unsupported MBC
		panic("Unsupported cartridge type")
	}
}

type cart struct {
	roms     [][]uint8
	rams     [][]uint8
	cartType Type
	logger   *slog.Logger
}

func newCart(rom, ram []uint8, logger *slog.Logger) *cart {
	c := &cart{
		cartType: Type(rom[0x0147]),
		logger:   logger,
	}
	c.roms = make([][]uint8, 1)
	// ROM Bank 00 is always present
	c.roms[0] = make([]uint8, bytesPerROMBank)
	copy(c.roms[0], rom[:bytesPerROMBank])

	// Additional ROM banks if needed
	for i := 1; i < c.ROMBankSize(); i++ {
		c.roms = append(c.roms, make([]uint8, bytesPerROMBank))
		copy(c.roms[i], rom[i*bytesPerROMBank:(i+1)*bytesPerROMBank])
	}

	// RAM banks if needed
	for i := 0; i < c.RAMBankSize(); i++ {
		c.rams = append(c.rams, newRAMBank())
		if i*bytesPerRAMBank < len(ram) {
			copy(c.rams[i], ram[i*bytesPerRAMBank:min((i+1)*bytesPerRAMBank, len(ram))])
		}
	}

	return c
}

func newRAMBank() []uint8 {
	s := make([]uint8, bytesPerRAMBank)
	for i := range s {
		s[i] = 0xFF
	}
	return s
}

func (c *cart) HasValidLogo() bool {
	return slices.Equal(c.roms[0][0x0104:0x0134], validLogo)
}

func (c *cart) Title() string {
	return string(bytes.Trim(c.roms[0][0x0134:0x0144], "\x00"))
}

func (c *cart) Type() Type {
	return c.cartType
}

// HasBattery reports whether battery-backed storage is supported for this
// cartridge by the current implementation.
func (c *cart) HasBattery() bool {
	switch c.cartType {
	case TypeMBC1RAMBattery, TypeMBC5RAMBattery, TypeMBC5RumbleRAMBattery:
		return true
	default:
		return false
	}
}

// SaveRAM returns a copy of the cartridge RAM in bank order.
func (c *cart) SaveRAM() []uint8 {
	size := c.RAMSize()
	if size <= 0 {
		return nil
	}

	ram := make([]uint8, size)
	offset := 0
	for _, bank := range c.rams {
		offset += copy(ram[offset:], bank)
		if offset == len(ram) {
			break
		}
	}
	return ram
}

func (c *cart) ROMSize() int {
	switch sizeCode := c.roms[0][0x0148]; sizeCode {
	case 0x00:
		return 32 * Ki // 32KiB
	case 0x01:
		return 64 * Ki // 64KiB
	case 0x02:
		return 128 * Ki // 128KiB
	case 0x03:
		return 256 * Ki // 256KiB
	case 0x04:
		return 512 * Ki // 512KiB
	case 0x05:
		return 1024 * Ki // 1MiB
	case 0x06:
		return 2 * Mi // 2MiB
	case 0x07:
		return 4 * Mi // 4MiB
	case 0x08:
		return 8 * Mi // 8MiB
	default:
		return -1
	}
}

func (c *cart) ROMBankSize() int {
	return c.ROMSize() / (16 * Ki) // 16KiB per ROM bank
}

func (c *cart) RAMSize() int {
	switch sizeCode := c.roms[0][0x0149]; sizeCode {
	case 0x00:
		return 0 // No RAM
	case 0x02:
		return 8 * Ki // 8KiB
	case 0x03:
		return 32 * Ki // 32KiB (4 banks of 8KiB each)
	case 0x04:
		return 128 * Ki // 128KiB (16 banks of 8KiB each)
	case 0x05:
		return 64 * Ki // 64KiB (8 banks of 8KiB each)
	default:
		return -1
	}
}

func (c *cart) RAMBankSize() int {
	return c.RAMSize() / (8 * Ki) // 8KiB per RAM bank
}
