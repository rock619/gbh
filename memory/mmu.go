package memory

import (
	"log/slog"
	"slices"

	"github.com/rock619/gbh/audio"
	"github.com/rock619/gbh/cartridge"
	"github.com/rock619/gbh/cpu"
	"github.com/rock619/gbh/graphic"
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/joypad"
	"github.com/rock619/gbh/register"
	"github.com/rock619/gbh/serial"
	"github.com/rock619/gbh/timer"
)

// MMU (Memory Management Unit) is responsible for managing memory access and mapping
type MMU struct {
	cart cartridge.Cartridge
	ppu  *graphic.PPU
	apu  *audio.APU
	j    *joypad.Joypad
	ic   *interrupt.Controller
	s    *serial.Serial
	t    *timer.Timer

	wram RAM
	hram RAM

	logger *slog.Logger
}

func NewMMU(
	cart cartridge.Cartridge,
	ppu *graphic.PPU,
	apu *audio.APU,
	j *joypad.Joypad,
	ic *interrupt.Controller,
	s *serial.Serial,
	t *timer.Timer,
	logger *slog.Logger,
) *MMU {
	m := &MMU{
		cart:   cart,
		ppu:    ppu,
		apu:    apu,
		j:      j,
		ic:     ic,
		s:      s,
		t:      t,
		wram:   NewRAM(8 * 1024), // 8KB of Work RAM
		hram:   NewRAM(127),      // 127 bytes of High RAM
		logger: logger,
	}
	return m
}

func (m *MMU) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x3FFF:
		// ROM Bank 0
		return m.cart.Read(addr)
	case addr >= 0x4000 && addr <= 0x7FFF:
		// ROM Bank 1-N
		return m.cart.Read(addr)
	case addr >= 0x8000 && addr <= 0x9FFF:
		// VRAM
		return m.ppu.ReadVRAM(addr)
	case addr >= 0xA000 && addr <= 0xBFFF:
		// External RAM
		return m.cart.Read(addr)
	case addr >= 0xC000 && addr <= 0xDFFF:
		// Work RAM
		return m.wram.Read(addr - 0xC000)
	case addr >= 0xE000 && addr <= 0xFDFF:
		// Echo RAM
		return m.wram.Read(addr - 0xE000)
	case addr >= 0xFE00 && addr <= 0xFE9F:
		// OAM
		return m.ppu.ReadOAM(addr - 0xFE00)
	case addr >= 0xFEA0 && addr <= 0xFEFF:
		// Unusable
		return 0x00
	case addr >= 0xFF00 && addr <= 0xFF7F:
		// I/O Registers
		return m.ReadIORegister(addr)
	case addr >= 0xFF80 && addr < 0xFFFF:
		// High RAM
		return m.hram.Read(addr - 0xFF80)
	case addr == 0xFFFF:
		// Interrupt Enable Register
		return m.ic.Read(addr)
	}

	return 0xFF // Unmapped memory returns 0xFF
}

func (m *MMU) ReadIORegister(addr uint16) uint8 {
	switch {
	case addr == register.P1JOYP:
		// Joypad input
		return m.j.Read(addr)
	case addr == register.SB || addr == register.SC:
		// Serial transfer data and control registers
		return m.s.Read(addr)
	case slices.Contains([]uint16{register.DIV, register.TIMA, register.TMA, register.TAC}, addr):
		// Timer registers
		return m.t.Read(addr)
	case addr == register.IF:
		// Interrupt Flag
		return m.ic.Read(addr)
	case slices.Contains([]uint16{
		register.NR10, register.NR11, register.NR12, register.NR13, register.NR14,
		register.NR21, register.NR22, register.NR23, register.NR24,
		register.NR30, register.NR31, register.NR32, register.NR33, register.NR34,
		register.NR41, register.NR42, register.NR43, register.NR44,
		register.NR50, register.NR51, register.NR52,
	}, addr):
		// APU registers
		return m.apu.Read(addr)
	case addr >= 0xFF30 && addr <= 0xFF3F:
		// Wave pattern RAM (part of APU)
		return m.apu.Read(addr)
	case slices.Contains([]uint16{
		register.LCDC, register.STAT, register.SCY, register.SCX, register.LY, register.LYC, register.DMA, register.BGP, register.OBP0, register.OBP1, register.WY, register.WX,
	}, addr):
		// PPU registers
		return m.ppu.ReadRegister(addr)
	case addr == register.IE:
		// Interrupt Enable Register
		return m.ic.Read(addr)
	default:
		return 0xFF // Unmapped I/O registers return 0xFF
	}
}

func (m *MMU) Write(addr uint16, value byte) {
	switch {
	case addr <= 0x7FFF:
		// 0x0000-0x7FFF is ROM, which is not writable, but some cartridges use writes to this range to control bank switching, so we pass it to the cartridge
		m.cart.Write(addr, value)
	case addr >= 0x8000 && addr <= 0x9FFF:
		// VRAM
		m.ppu.WriteVRAM(addr, value)
	case addr >= 0xA000 && addr <= 0xBFFF:
		// External RAM
		m.cart.Write(addr, value)
	case addr >= 0xC000 && addr <= 0xDFFF:
		// Work RAM
		m.wram.Write(addr-0xC000, value)
	case addr >= 0xE000 && addr <= 0xFDFF:
		// Echo RAM
		m.wram.Write(addr-0xE000, value)
	case addr >= 0xFE00 && addr <= 0xFE9F:
		// OAM
		m.ppu.WriteOAM(addr-0xFE00, value)
	case addr >= 0xFEA0 && addr <= 0xFEFF:
		// Unusable
	case addr >= 0xFF00 && addr <= 0xFF7F:
		// I/O Registers
		m.WriteIORegister(addr, value)
	case addr >= 0xFF80 && addr <= 0xFFFE:
		// High RAM
		m.hram.Write(addr-0xFF80, value)
	case addr == 0xFFFF:
		// Interrupt Enable Register
		m.ic.Write(addr, value)
	}
}

func (m *MMU) WriteIORegister(addr uint16, value byte) {
	switch {
	case addr == register.P1JOYP:
		// Joypad input
		m.j.Write(addr, value)
	case addr == register.SB || addr == register.SC:
		// Serial transfer data and control registers
		m.s.Write(addr, value)
	case slices.Contains([]uint16{register.DIV, register.TIMA, register.TMA, register.TAC}, addr):
		// Timer registers
		m.t.Write(addr, value)
	case addr == register.IF:
		// Interrupt Flag
		m.ic.Write(addr, value)
	case slices.Contains([]uint16{
		register.NR10, register.NR11, register.NR12, register.NR13, register.NR14,
		register.NR21, register.NR22, register.NR23, register.NR24,
		register.NR30, register.NR31, register.NR32, register.NR33, register.NR34,
		register.NR41, register.NR42, register.NR43, register.NR44,
		register.NR50, register.NR51, register.NR52,
	}, addr):
		// APU registers
		m.apu.Write(addr, value)
	case addr >= 0xFF30 && addr <= 0xFF3F:
		// Wave pattern RAM (part of APU)
		m.apu.Write(addr, value)
	case slices.Contains([]uint16{
		register.LCDC, register.STAT, register.SCY, register.SCX, register.LY, register.LYC,
		register.BGP, register.OBP0, register.OBP1, register.WY, register.WX,
	}, addr):
		// PPU registers
		m.ppu.WriteRegister(addr, value)
	case addr == register.DMA:
		m.ppu.WriteRegister(addr, value)
		// OAM DMA copies 160 bytes from XX00-XX9F into FE00-FE9F.
		// This emulator performs the copy immediately; timing stalls can be
		// modeled later if a ROM depends on cycle-accurate DMA behavior.
		base := uint16(value) << 8
		for offset := range uint16(0xA0) {
			m.ppu.WriteOAM(offset, m.Read(base+offset))
		}
	default:
	}
}

func (m *MMU) CorruptOAM(addr uint16, kind cpu.OAMBugKind) {
	m.ppu.CorruptOAM(addr, kind)
}
