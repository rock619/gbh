package joypad

import (
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/memorybus"
	"github.com/rock619/gbh/register"
)

const (
	selectDPadMask    uint8 = 1 << 4
	selectButtonsMask uint8 = 1 << 5
)

type KeyCode uint8

const (
	KeyA      KeyCode = iota
	KeyB      KeyCode = iota
	KeySelect KeyCode = iota
	KeyStart  KeyCode = iota
	KeyRight  KeyCode = iota
	KeyLeft   KeyCode = iota
	KeyUp     KeyCode = iota
	KeyDown   KeyCode = iota
)

type Joypad struct {
	selectBits uint8 // bit 4/5: ROM select bits. 0=select, 1=deselect
	buttons    uint8 // bit 0-3: A, B, Select, Start. 1=released, 0=pressed
	dpad       uint8 // bit 0-3: Right, Left, Up, Down. 1=released, 0=pressed

	memorybus.Memory
	ir interrupt.Requester
}

func New(ir interrupt.Requester) *Joypad {
	return &Joypad{
		selectBits: 0x30, // Both button and dpad groups deselected (bits 4 and 5 are 1)
		buttons:    0x0F,
		dpad:       0x0F,
		ir:         ir,
	}
}

func (j *Joypad) Read(addr uint16) uint8 {
	if addr != register.P1JOYP {
		return 0xFF
	}

	low := uint8(0x0F)
	if j.ButtonsSelected() {
		low &= j.buttons
	}
	if j.DPadSelected() {
		low &= j.dpad
	}
	return 0xC0 | j.selectBits | low
}

func (j *Joypad) Write(addr uint16, value uint8) {
	if addr == register.P1JOYP {
		j.selectBits = value & 0x30
	}
}

func (j *Joypad) SetPressed(keys []KeyCode) {
	before := j.Read(register.P1JOYP)

	j.buttons = 0x0F
	j.dpad = 0x0F

	for _, key := range keys {
		switch key {
		case KeyA:
			j.buttons &^= 1 << 0
		case KeyB:
			j.buttons &^= 1 << 1
		case KeySelect:
			j.buttons &^= 1 << 2
		case KeyStart:
			j.buttons &^= 1 << 3
		case KeyRight:
			j.dpad &^= 1 << 0
		case KeyLeft:
			j.dpad &^= 1 << 1
		case KeyUp:
			j.dpad &^= 1 << 2
		case KeyDown:
			j.dpad &^= 1 << 3
		}
	}

	if after := j.Read(register.P1JOYP); before&^after&0x0F != 0 {
		j.ir.Request(interrupt.Joypad)
	}
}

func (j *Joypad) ButtonsSelected() bool {
	return j.selectBits&selectButtonsMask == 0
}

func (j *Joypad) DPadSelected() bool {
	return j.selectBits&selectDPadMask == 0
}
