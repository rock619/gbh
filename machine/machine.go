// TODO: implement
package machine

import (
	"image"

	"github.com/rock619/gbh/audio"
	"github.com/rock619/gbh/cartridge"
	"github.com/rock619/gbh/cpu"
	"github.com/rock619/gbh/graphic"
	"github.com/rock619/gbh/joypad"
	"github.com/rock619/gbh/memory"
)

// Machine represents the entire Game Boy hardware. It does not depend on Ebiten.
type Machine struct {
	cpu    *cpu.CPU
	mmu    *memory.MMU
	ppu    *graphic.PPU
	apu    *audio.APU
	joypad *joypad.Joypad
	cart   cartridge.Cartridge
	// TODO: Implement Serial port
	// Serial *serial.Port

	Cycles uint64
}

func (m *Machine) Step() (int, error) {
	cycles, err := m.cpu.Step()
	if err != nil {
		return 0, err
	}
	return cycles, nil
}

func (m *Machine) SetPressed(keys []joypad.KeyCode) {
	m.joypad.SetPressed(keys)
}

func (m *Machine) Framebuffer() *image.RGBA {
	return m.ppu.Framebuffer()
}

func (m *Machine) Title() string {
	return m.cart.Title()
}

func (m *Machine) SaveRAM() []byte {
	return m.cart.SaveRAM()
}
