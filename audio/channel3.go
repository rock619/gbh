package audio

import (
	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/register"
)

type Channel3 struct {
	apu     *APU
	enabled bool

	dacEnabled  bool
	outputLevel uint8
	period      uint16
	periodReg   uint16

	length *Length

	freqTimer     int
	currentVolume int

	wavePattern   [16]uint8
	waveIndex     uint8
	waveReadCount uint8
}

func NewChannel3(apu *APU) *Channel3 {
	c := &Channel3{
		apu: apu,
	}
	c.length = NewLength(c, 8)
	return c
}

func (c *Channel3) PowerOff() {
	c.enabled = false
	c.dacEnabled = false
	c.outputLevel = 0
	c.period = 0
	c.periodReg = 0
	c.length.Disable()
	c.freqTimer = 0
	c.currentVolume = 0
	c.waveIndex = 0
	c.waveReadCount = 0
}

func (c *Channel3) Reset() {
	c.PowerOff()
	c.length.Reset()
}

func (c *Channel3) Enabled() bool {
	return c.enabled
}

func (c *Channel3) Disable() {
	c.enabled = false
}

func (c *Channel3) Read(addr uint16) uint8 {
	switch {
	case addr == register.NR30:
		if c.dacEnabled {
			return 0xFF
		}
		return 0x7F
	case addr == register.NR31:
		return 0xFF // Length register is write-only
	case addr == register.NR32:
		return (c.outputLevel << 5) | 0b10011111
	case addr == register.NR33:
		return 0xFF // Frequency low register is write-only
	case addr == register.NR34:
		return 0b10111111 | (bits.BoolToBit(c.length.Enabled()) << 6)
	case addr >= 0xFF30 && addr <= 0xFF3F:
		if c.Enabled() {
			if c.freqTimer != 2 {
				return 0xFF
			}
			if c.waveReadCount == 0 {
				return 0xFF
			}
			return c.wavePattern[c.waveIndex/2]
		}
		return c.wavePattern[addr-0xFF30]
	default:
		return 0xFF
	}
}

func (c *Channel3) Write(addr uint16, value uint8) {
	switch {
	case addr == register.NR30:
		c.dacEnabled = value&(1<<7) != 0
		if !c.dacEnabled {
			c.Disable()
		}
	case addr == register.NR31:
		c.length.Write(value)
	case addr == register.NR32:
		c.WriteLevel(value)
	case addr == register.NR33:
		c.periodReg = (c.periodReg & 0xFF00) | uint16(value)
	case addr == register.NR34:
		c.periodReg = (c.periodReg & 0x00FF) | (uint16(value&0b111) << 8)
		trigger := bits.BitToBool(value & (1 << 7))
		c.length.WriteEnabled(bits.BitToBool(value&(1<<6)), trigger, c.apu.NextStepClocksLength())
		if trigger {
			c.corruptWaveRAMOnRetrigger()
			c.Trigger()
		}
	case addr >= 0xFF30 && addr <= 0xFF3F:
		if c.Enabled() {
			if c.freqTimer == 2 && c.waveReadCount != 0 {
				c.wavePattern[c.waveIndex/2] = value
			}
			return
		}
		c.wavePattern[addr-0xFF30] = value
	}
}

func (c *Channel3) WriteLevel(value uint8) {
	c.outputLevel = (value >> 5) & 0b11
}

func (c *Channel3) corruptWaveRAMOnRetrigger() {
	if !c.Enabled() || c.freqTimer != 4 || c.waveReadCount == 0 {
		return
	}

	index := c.waveIndex / 2
	if index < 4 {
		c.wavePattern[0] = c.wavePattern[index]
		return
	}

	base := index &^ 0b11
	copy(c.wavePattern[0:4], c.wavePattern[base:base+4])
}

func (c *Channel3) ResetFreqTimer() {
	c.freqTimer = (2048-int(c.period))*2 + 4
}

func (c *Channel3) Trigger() {
	c.enabled = c.dacEnabled
	c.length.ResetTimerIfExpired(c.apu.NextStepClocksLength())
	c.period = c.periodReg
	c.ResetFreqTimer()
	c.ResetVolume()
	c.ResetWaveIndex()
}

func (c *Channel3) ResetVolume() {
	c.currentVolume = int(c.outputLevel)
}

func (c *Channel3) ResetWaveIndex() {
	c.waveIndex = 0
	c.waveReadCount = 0
}

func (c *Channel3) Tick(cycles int) {
	if !c.Enabled() {
		return
	}

	for c.freqTimer -= cycles; c.freqTimer <= 0; {
		c.period = c.periodReg
		c.freqTimer += (2048 - int(c.period)) * 2
		c.waveIndex = (c.waveIndex + 1) % 32
		c.waveReadCount++
	}
}

func (c *Channel3) DigitalSample() (sample uint8, ok bool) {
	if !c.Enabled() || !c.dacEnabled || c.outputLevel == 0 {
		return 0, false
	}

	waveValue := c.wavePattern[c.waveIndex/2]
	if c.waveIndex%2 == 0 {
		waveValue >>= 4
	} else {
		waveValue &= 0x0F
	}

	return waveValue >> c.OutputLevelShift(), true
}

func (c *Channel3) OutputLevelShift() uint8 {
	switch c.outputLevel {
	case 0b00:
		// Mute
		return 8
	case 0b01:
		// 100% volume
		return 0
	case 0b10:
		// 50% volume
		return 1
	case 0b11:
		// 25% volume
		return 2
	default:
		return 8
	}
}
