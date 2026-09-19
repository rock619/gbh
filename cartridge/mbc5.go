package cartridge

import "log/slog"

type MBC5 struct {
	*cart
	romBank    uint16
	ramBank    uint8
	ramEnabled bool
}

func NewMBC5(rom, ram []uint8, logger *slog.Logger) *MBC5 {
	return &MBC5{
		cart: newCart(rom, ram, logger),
	}
}

func (c *MBC5) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		// ROM Bank 00 (fixed)
		return c.roms[0][addr]
	case addr >= 0x4000 && addr <= 0x7FFF:
		// ROM Bank 01-1FF (switchable)
		return c.roms[c.romBank][addr-0x4000]
	case addr >= 0xA000 && addr <= 0xBFFF:
		// RAM Bank 00-0F (switchable)
		if c.ramEnabled && c.ramBank < uint8(c.RAMBankSize()) {
			return c.rams[c.ramBank][addr-0xA000]
		}
	}
	return 0xFF // Unmapped memory returns 0xFF
}

func (c *MBC5) Write(addr uint16, value uint8) {
	switch {
	case addr <= 0x1FFF:
		if value&0x0F == 0x0A {
			c.ramEnabled = true
		} else {
			c.ramEnabled = false
		}
	case addr >= 0x2000 && addr <= 0x2FFF:
		c.romBank = (c.romBank & 0x100) | uint16(value)
	case addr >= 0x3000 && addr <= 0x3FFF:
		c.romBank = (c.romBank & 0xFF) | (uint16(value&1) << 8)
	case addr >= 0x4000 && addr <= 0x5FFF:
		c.ramBank = value & 0x0F
	case addr >= 0xA000 && addr <= 0xBFFF:
		if c.ramEnabled && c.ramBank < uint8(c.RAMBankSize()) {
			c.rams[c.ramBank][addr-0xA000] = value
		}
	}
}
