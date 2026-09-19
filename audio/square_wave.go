package audio

import (
	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/memorybus"
)

var squareWaveDuty = [4][8]uint8{
	{0, 0, 0, 0, 0, 0, 0, 1}, // 00: 12.5%
	{1, 0, 0, 0, 0, 0, 0, 1}, // 01: 25%
	{1, 0, 0, 0, 0, 1, 1, 1}, // 10: 50%
	{0, 1, 1, 1, 1, 1, 1, 1}, // 11: 75%
}

// SquareWaveChannel represents a square wave audio channel in the Game Boy.
// It contains common functionality for both Channel 1 and Channel 2, which are the two square wave channels.
type SquareWaveChannel struct {
	memorybus.Memory
	apu     *APU
	enabled bool

	lengthAddr   uint16
	length       *Length
	envelopeAddr uint16
	envelope     *Envelope
	// The onTrigger callback is called when the channel is triggered. It allows the parent channel (Channel 1 or Channel 2) to perform additional actions on trigger, such as resetting the sweep for Channel 1.
	onTrigger  func()
	onPowerOff func()

	duty uint8

	lowAddr  uint16
	low      uint8
	highAddr uint16
	high     uint8

	freqTimer int

	waveStep int
}

func NewSquareWaveChannel(apu *APU, lengthAddr, envelopeAddr, lowAddr, highAddr uint16) *SquareWaveChannel {
	c := &SquareWaveChannel{
		apu:          apu,
		lengthAddr:   lengthAddr,
		envelopeAddr: envelopeAddr,
		lowAddr:      lowAddr,
		highAddr:     highAddr,
	}

	c.length = NewLength(c, 6)
	c.envelope = NewEnvelope(c)
	return c
}

func (c *SquareWaveChannel) Enabled() bool {
	return c.enabled
}

func (c *SquareWaveChannel) Disable() {
	c.enabled = false
}

func (c *SquareWaveChannel) Read(addr uint16) uint8 {
	switch addr {
	case c.lengthAddr:
		return c.duty<<6 | 0b111111
	case c.envelopeAddr:
		return c.envelope.Read()
	case c.lowAddr:
		return 0xFF
	case c.highAddr:
		return c.ReadHigh() | 0b10111111
	}
	return 0xFF
}

func (c *SquareWaveChannel) ReadLow() uint8 {
	return c.low
}

func (c *SquareWaveChannel) ReadHigh() uint8 {
	return c.high
}

func (c *SquareWaveChannel) Write(addr uint16, value uint8) {
	switch addr {
	case c.lengthAddr:
		c.duty = value >> 6
		c.length.Write(value)
	case c.envelopeAddr:
		c.envelope.Write(value)
	case c.lowAddr:
		c.WriteLow(value)
	case c.highAddr:
		c.WriteHigh(value)
	}
}

func (c *SquareWaveChannel) WriteLow(value uint8) {
	c.low = value
}

func (c *SquareWaveChannel) WriteHigh(value uint8) {
	c.high = value
	trigger := bits.BitToBool(value & (1 << 7))
	c.length.WriteEnabled(bits.BitToBool(value&(1<<6)), trigger, c.apu.NextStepClocksLength())
	if trigger {
		c.Trigger()
	}
}

func (c *SquareWaveChannel) LengthEnabled() bool {
	return c.high&(1<<6) != 0
}

func (c *SquareWaveChannel) Trigger() {
	c.enabled = c.envelope.DACEnabled()
	c.length.ResetTimerIfExpired(c.apu.NextStepClocksLength())
	c.ResetFreqTimer()
	c.envelope.ResetTimer()
	c.envelope.ResetVolume()
	if c.onTrigger != nil {
		c.onTrigger()
	}
}

func (c *SquareWaveChannel) Period() uint16 {
	return uint16(c.high&0b111)<<8 | uint16(c.low)
}

func (c *SquareWaveChannel) SetPeriod(period uint16) {
	c.low = uint8(period & 0xFF)
	c.high = (c.high & 0b11111000) | uint8((period>>8)&0b111)
}

func (c *SquareWaveChannel) Tick(cycles int) {
	for c.freqTimer -= cycles; c.freqTimer <= 0; {
		c.freqTimer += (2048 - int(c.Period())) * 4
		c.waveStep = (c.waveStep + 1) & 0b111
	}
}

func (c *SquareWaveChannel) DigitalSample() (sample uint8, ok bool) {
	if !c.Enabled() {
		return 0, false
	}
	waveformState := squareWaveDuty[c.duty][c.waveStep]
	return uint8(waveformState * uint8(c.envelope.Volume())), true
}

func (c *SquareWaveChannel) ResetFreqTimer() {
	c.freqTimer = (2048 - int(c.Period())) * 4
}

func (c *SquareWaveChannel) SetOnTrigger(onTrigger func()) {
	c.onTrigger = onTrigger
}

func (c *SquareWaveChannel) PowerOff() {
	c.enabled = false

	c.duty = 0
	c.low = 0
	c.high = 0
	c.freqTimer = 0
	c.waveStep = 0

	c.length.Disable()
	c.envelope.Reset()
	if c.onPowerOff != nil {
		c.onPowerOff()
	}
}

func (c *SquareWaveChannel) SetOnPowerOff(onPowerOff func()) {
	c.onPowerOff = onPowerOff
}

func (c *SquareWaveChannel) Reset() {
	c.PowerOff()
	c.length.Reset()
}
