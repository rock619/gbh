package audio

import (
	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/register"
)

type Channel4 struct {
	apu      *APU
	enabled  bool
	envelope *Envelope
	length   *Length

	freqTimer int
	// LFSR: Linear Feedback Shift Register
	lfsr         uint16
	clockShift   uint8
	shortMode    bool
	clockDivider uint8
}

func NewChannel4(apu *APU) *Channel4 {
	c := &Channel4{
		apu:     apu,
		enabled: false,
	}

	c.envelope = NewEnvelope(c)
	c.length = NewLength(c, 6)
	return c
}

func (c *Channel4) PowerOff() {
	c.enabled = false
	c.freqTimer = 0
	c.lfsr = 0x0000
	c.clockShift = 0
	c.shortMode = false
	c.clockDivider = 0

	c.envelope.Reset()
	c.length.Disable()
}

func (c *Channel4) Reset() {
	c.PowerOff()
	c.length.Reset()
}

func (c *Channel4) Enabled() bool {
	return c.enabled
}

func (c *Channel4) Disable() {
	c.enabled = false
}

func (c *Channel4) Read(addr uint16) uint8 {
	switch addr {
	case register.NR41:
		return 0xFF // Length register is write-only
	case register.NR42:
		return c.envelope.Read()
	case register.NR43:
		return (c.clockShift << 4) | (bits.BoolToBit(c.shortMode) << 3) | c.clockDivider
	case register.NR44:
		return 0b10111111 | (bits.BoolToBit(c.length.Enabled()) << 6)
	default:
		return 0xFF
	}
}

func (c *Channel4) Write(addr uint16, value uint8) {
	switch addr {
	case register.NR41:
		c.length.Write(value)
	case register.NR42:
		c.envelope.Write(value)
	case register.NR43:
		c.clockShift = value >> 4
		c.shortMode = bits.BitToBool((value >> 3) & 0b1)
		c.clockDivider = value & 0b111
	case register.NR44:
		trigger := bits.BitToBool(value & (1 << 7))
		c.length.WriteEnabled(bits.BitToBool(value&(1<<6)), trigger, c.apu.NextStepClocksLength())
		if trigger {
			c.Trigger()
		}
	}
}

func (c *Channel4) Trigger() {
	c.enabled = c.envelope.DACEnabled()
	c.length.ResetTimerIfExpired(c.apu.NextStepClocksLength())
	c.envelope.ResetTimer()
	c.envelope.ResetVolume()
	c.lfsr = 0x0000
}

func (c *Channel4) TickLFSR() {
	// 1. nextBit (= 1 if bit 0 and bit 1 are identical, 0 otherwise) is written to bit 15
	nextBit := (c.lfsr & 0b1) ^ ((c.lfsr & 0b10) >> 1) ^ 0b1
	c.lfsr |= nextBit << 15

	// 2. If "short mode" was selected in NR43, then bit 15 is copied to bit 7 as well.
	if c.shortMode {
		c.lfsr &^= 1 << 7
		c.lfsr |= nextBit << 7
	}

	// 3. The entire register is shifted right by one bit.
	c.lfsr >>= 1
}

func (c *Channel4) DigitalSample() (sample uint8, ok bool) {
	if !c.Enabled() {
		return 0, false
	}

	if c.lfsr&1 == 0 {
		return uint8(c.envelope.Volume()), true
	}
	return 0, true
}

// LFSRClockCycles calculates how many CPU cycles should elapse between each LFSR tick based on the current clock divider and clock shift settings.
//
// LFSR frequency is (262144 / (clockDivider * 2^clockShift)) Hz
// CPU runs at 4194304 Hz, so the number of CPU cycles between each LFSR clock is:
//
//	CPU cycles per LFSR tick
//	= CPU clock / LFSR frequency
//	= 4194304 / (262144 / (clockDivider * 2^clockShift))
//	= 16 * clockDivider * 2^clockShift
//
// Note that divider = 0 is treated as divider = 0.5 instead, so we multiply by 8 instead of 16 in that case.
func (c *Channel4) LFSRClockCycles() int {
	if c.clockDivider == 0 {
		return 8 * (1 << c.clockShift)
	}
	return 16 * int(c.clockDivider) * (1 << c.clockShift)
}

func (c *Channel4) Tick(cycles int) {
	c.freqTimer -= cycles

	for c.freqTimer <= 0 {
		c.freqTimer += c.LFSRClockCycles()

		// If clockShift is 14 or higher, the LFSR will only produce 0s, so we can skip ticking it to save CPU time.
		if c.clockShift >= 14 {
			continue
		}

		c.TickLFSR()
	}
}
