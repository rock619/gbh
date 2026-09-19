package cartridge

import "log/slog"

type MBC1 struct {
	cart
	ramEnabled          bool
	ramBankNumber       uint8
	romBankNumber       uint8
	advancedBankingMode bool
}

func NewMBC1(rom, ram []uint8, logger *slog.Logger) *MBC1 {
	return &MBC1{
		cart:                *newCart(rom, ram, logger),
		ramEnabled:          false,
		ramBankNumber:       0,
		romBankNumber:       1,
		advancedBankingMode: false,
	}
}

func (c *MBC1) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		// ROM Bank X0
		return c.roms[c.effectiveROM0Bank()][addr]
	case addr >= 0x4000 && addr <= 0x7FFF:
		// ROM Bank 01-7F (switchable)
		return c.roms[c.effectiveROMBank()][addr-0x4000]
	case addr >= 0xA000 && addr <= 0xBFFF:
		// RAM Bank 00-03 (switchable)
		if c.ramEnabled && c.ramBankNumber < uint8(c.RAMBankSize()) {
			return c.rams[c.ramBankNumber][addr-0xA000]
		}
	}
	return 0xFF // Unmapped memory returns 0xFF
}

func (c *MBC1) effectiveROM0Bank() int {
	if !c.advancedBankingMode {
		return 0
	}
	return int(c.romBankNumber & 0b01100000)
}

func (c *MBC1) effectiveROMBank() int {
	return int(c.romBankNumber) % c.ROMBankSize()
}

func (c *MBC1) Write(addr uint16, value uint8) {
	switch {
	case addr <= 0x1FFF:
		// RAM Enable
		// Writing 0x0A enables RAM, any other value disables it
		c.ramEnabled = (value & 0x0F) == 0x0A
	case addr >= 0x2000 && addr <= 0x3FFF:
		// ROM Bank Number (lower 5 bits)
		c.romBankNumber = (c.romBankNumber & 0b11100000) | (value & 0b00011111)
		if c.romBankNumber == 0 {
			c.romBankNumber = 1 // Bank 0 cannot be selected, wrap to bank 1
		}
	case addr >= 0x4000 && addr <= 0x5FFF:
		// RAM Bank Number or Upper ROM Bank Bits
		if c.ROMSize() >= 1*Mi {
			// If ROM size is greater than 1MiB, this register controls the upper 2 bits of the ROM bank number
			c.romBankNumber = (0b01100000 & (value << 5)) | (c.romBankNumber & 0b00011111)
			return
		}
		// Otherwise, it controls the RAM bank number
		c.ramBankNumber = value & 0b00000011
	case addr >= 0x6000 && addr <= 0x7FFF:
		// Banking Mode Select
		c.advancedBankingMode = (value & 0x01) == 1
	case addr >= 0xA000 && addr <= 0xBFFF:
		// RAM Bank 00-03 (switchable)
		if c.ramEnabled && c.ramBankNumber < uint8(c.RAMBankSize()) {
			c.rams[c.ramBankNumber][addr-0xA000] = value
		}
	}
}
