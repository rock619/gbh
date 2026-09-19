package audio

import "github.com/rock619/gbh/bits"

type Envelope struct {
	channel ChannelDisabler

	initialVolume uint8
	currentVolume int
	increasing    bool
	sweepPace     uint8
	timer         uint8
}

func NewEnvelope(channel ChannelDisabler) *Envelope {
	return &Envelope{channel: channel}
}

func (e *Envelope) Reset() {
	e.initialVolume = 0
	e.currentVolume = 0
	e.increasing = false
	e.sweepPace = 0
	e.timer = 0
}

func (e *Envelope) Enabled() bool {
	return e.sweepPace > 0
}

func (e *Envelope) ResetTimer() {
	e.timer = e.sweepPace
}

func (e *Envelope) ResetVolume() {
	e.currentVolume = int(e.initialVolume)
}

func (e *Envelope) Apply() {
	switch {
	case e.increasing && e.currentVolume < 15:
		e.currentVolume++
	case !e.increasing && e.currentVolume > 0:
		e.currentVolume--
	}
}

func (e *Envelope) Tick() {
	if !e.Enabled() {
		return
	}

	e.timer--
	if e.timer <= 0 {
		e.ResetTimer()
		e.Apply()
	}
}

func (e *Envelope) Volume() int {
	return e.currentVolume
}

func (e *Envelope) Write(value uint8) {
	e.initialVolume = value >> 4
	e.increasing = (value & (1 << 3)) != 0
	if value&0b11111000 == 0 {
		e.channel.Disable()
	}
	e.sweepPace = value & 0b111
}

func (e *Envelope) Read() uint8 {
	return (e.initialVolume << 4) | (bits.BoolToBit(e.increasing) << 3) | e.sweepPace
}

func (e *Envelope) DACEnabled() bool {
	return e.initialVolume != 0 || e.increasing
}
