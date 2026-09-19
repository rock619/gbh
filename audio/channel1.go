package audio

import (
	"github.com/rock619/gbh/register"
)

type Channel1 struct {
	*SquareWaveChannel

	aud1Sweep uint8

	sweepTimer      int
	sweepEnabled    bool
	sweepPeriod     uint16
	sweepNegateUsed bool
}

func NewChannel1(apu *APU) *Channel1 {
	c := &Channel1{
		SquareWaveChannel: NewSquareWaveChannel(apu, register.NR11, register.NR12, register.NR13, register.NR14),
	}
	c.SetOnTrigger(c.onTrigger)
	c.SetOnPowerOff(c.Reset)
	return c
}

func (c *Channel1) Reset() {
	c.aud1Sweep = 0
	c.sweepTimer = 0
	c.sweepEnabled = false
	c.sweepPeriod = 0
}

func (c *Channel1) Read(addr uint16) uint8 {
	switch addr {
	case register.NR10:
		return 1<<7 | c.aud1Sweep
	case register.NR11, register.NR12, register.NR13, register.NR14:
		return c.SquareWaveChannel.Read(addr)
	default:
		return 0xFF
	}
}

func (c *Channel1) Write(addr uint16, value uint8) {
	switch addr {
	case register.NR10:
		prevDirection := c.SweepDirection()
		c.aud1Sweep = value
		// Clearing the sweep direction bit in NR10 after at least one sweep calculation has been made using the
		// substraction mode since the last trigger causes the channel to be immediately disabled. This prevents you
		// from having the sweep lower the frequency then raise the frequency without a trigger inbetween.
		sweepDirectionBitCleared := prevDirection == 1 && c.SweepDirection() == 0
		if sweepDirectionBitCleared && c.sweepNegateUsed {
			c.Disable()
		}
	case register.NR11, register.NR12, register.NR13, register.NR14:
		c.SquareWaveChannel.Write(addr, value)
	}
}

func (c *Channel1) Sweep() {
	next, overflowed := c.NextPeriod()
	if overflowed {
		c.Disable()
		return
	}

	if c.SweepStep() > 0 {
		c.sweepPeriod = next
		c.SetPeriod(next)

		// After updating frequency, calculate again and disable on overflow.
		if _, overflowed := c.NextPeriod(); overflowed {
			c.Disable()
		}
	}
}

func (c *Channel1) NextPeriod() (next uint16, overflowed bool) {
	offset := c.sweepPeriod >> c.SweepStep()
	next = c.sweepPeriod
	if c.SweepDirection() == SweepDirectionIncrease {
		next += offset
	} else {
		next -= offset
		c.sweepNegateUsed = true
	}

	return next, next > 2047
}

func (c *Channel1) SweepPace() int {
	return int((c.aud1Sweep >> 4) & 0x7)
}

func (c *Channel1) SweepDirection() int {
	return int((c.aud1Sweep >> 3) & 0b1)
}

func (c *Channel1) SweepStep() int {
	return int(c.aud1Sweep & 0b111)
}

func (c *Channel1) TickSweepTimer() {
	c.sweepTimer--
	if c.sweepTimer > 0 {
		return
	}
	c.ResetSweepTimer()
	if c.sweepEnabled && c.SweepPace() > 0 {
		c.Sweep()
	}
}

func (c *Channel1) ResetSweepTimer() {
	c.sweepTimer = c.SweepPace()
	if c.sweepTimer == 0 {
		c.sweepTimer = 8
	}
}

func (c *Channel1) onTrigger() {
	c.sweepNegateUsed = false
	// CH1 period value is copied to the "shadow register".
	c.sweepPeriod = c.Period()
	// The "sweep timer" is reset.
	c.ResetSweepTimer()
	// The "enabled flag" is set if either the sweep pace or individual step are non-zero, cleared otherwise.
	c.sweepEnabled = c.SweepPace() > 0 || c.SweepStep() > 0
	// If the individual step is non-zero, frequency calculation and overflow check are performed immediately.
	if c.SweepStep() > 0 {
		if _, overflowed := c.NextPeriod(); overflowed {
			c.Disable()
		}
	}
}

const (
	SweepDirectionIncrease = 0
	SweepDirectionDecrease = 1
)
