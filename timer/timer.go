package timer

import (
	"log/slog"

	"github.com/rock619/gbh/audio"
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/memorybus"
	"github.com/rock619/gbh/register"
)

const divAPUBit uint16 = 1 << 12

type Timer struct {
	memorybus.Memory

	tima uint8
	tma  uint8
	tac  uint8

	internalCounter uint16
	timaCounter     int

	fs audio.FrameSequencer

	ir interrupt.Requester

	logger *slog.Logger
}

func New(fs audio.FrameSequencer, ir interrupt.Requester, logger *slog.Logger) *Timer {
	return &Timer{
		fs:     fs,
		ir:     ir,
		logger: logger,
	}
}

func (t *Timer) Read(addr uint16) uint8 {
	switch addr {
	case register.DIV:
		return t.DIV()
	case register.TIMA:
		return t.tima
	case register.TMA:
		return t.tma
	case register.TAC:
		return t.tac
	default:
		return 0xFF
	}
}

func (t *Timer) Write(addr uint16, value uint8) {
	switch addr {
	case register.DIV:
		t.ResetDIV()
	case register.TIMA:
		t.tima = value
	case register.TMA:
		t.tma = value
	case register.TAC:
		t.tac = value & 0b111 // Only lower 3 bits are used
	}
}

func (t *Timer) ResetDIV() {
	prev := t.internalCounter
	t.internalCounter = 0

	if prev&divAPUBit != 0 {
		t.fs.StepSequencer()
	}
}

func (t *Timer) Tick(cycles int) {
	prev := t.internalCounter
	t.internalCounter += uint16(cycles)

	// A DIV-APU counter is increased every time DIV's bit 4 (5 in double-speed mode) goes from 1 to 0
	// DIV = internalCounter >> 8; so we check bit 12 of internalCounter for the transition
	if prev&divAPUBit != 0 && t.internalCounter&divAPUBit == 0 {
		t.fs.StepSequencer()
	}

	if !t.Enabled() {
		return
	}

	t.timaCounter += cycles

	for t.timaCounter >= t.ClockFrequency() {
		t.timaCounter -= t.ClockFrequency()
		if t.tima == 0xFF {
			t.tima = t.tma
			t.ir.Request(interrupt.Timer)
			continue
		}
		t.tima++
	}
}

func (t *Timer) DIV() uint8 {
	return uint8(t.internalCounter >> 8)
}

func (t *Timer) Enabled() bool {
	return t.tac&0b100 > 0
}

func (t *Timer) ClockSelect() uint8 {
	return t.tac & 0b11
}

func (t *Timer) ClockFrequency() int {
	switch t.ClockSelect() {
	case 0x00:
		return 1024 // 4096 Hz   (4194304 / 4096 = 1024 cycles)
	case 0x01:
		return 16 // 262144 Hz (4194304 / 262144 = 16 cycles)
	case 0x02:
		return 64 // 65536 Hz  (4194304 / 65536 = 64 cycles)
	case 0x03:
		return 256 // 16384 Hz  (4194304 / 16384 = 256 cycles)
	default:
		return 1024
	}
}
