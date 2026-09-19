package cartridge

import "log/slog"

// NoMBCCartridge is a simple cartridge with no memory bank controller (MBC)
type NoMBCCartridge struct {
	cart
}

func NewNoMBC(rom, ram []uint8, logger *slog.Logger) *NoMBCCartridge {
	return &NoMBCCartridge{
		cart: *newCart(rom, ram, logger),
	}
}

func (c *NoMBCCartridge) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		// ROM Bank 0
		return c.roms[0][addr]
	case addr >= 0x4000 && addr <= 0x7FFF:
		// ROM Bank 1
		return c.roms[1][addr-0x4000]
	case addr >= 0xA000 && addr <= 0xBFFF:
		// RAM Bank 0
		if len(c.rams) == 0 {
			return 0xFF
		}
		return c.rams[0][addr-0xA000]
	}
	return 0xFF // Unmapped memory returns 0xFF
}

func (c *NoMBCCartridge) Write(addr uint16, value uint8) {
	if addr >= 0xA000 && addr <= 0xBFFF {
		if len(c.rams) == 0 {
			return
		}
		c.rams[0][addr-0xA000] = value
	}
}
