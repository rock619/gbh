package audio

import (
	"log/slog"
	"slices"

	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/memorybus"
	"github.com/rock619/gbh/register"
)

const (
	sampleRate            = 44100
	cpuClockRate          = 4194304
	mixerBufferSamples    = 1 << 15
	targetBufferedSamples = 1 << 11
)

type APU struct {
	memorybus.Memory

	enabled bool

	// DIV-APU counter
	divAPU uint16
	ch1    *Channel1
	ch2    *Channel2
	ch3    *Channel3
	ch4    *Channel4

	Mix *Mixer

	logger *slog.Logger
}

func NewAPU(logger *slog.Logger) *APU {
	apu := &APU{
		logger: logger,
	}
	apu.ch1 = NewChannel1(apu)
	apu.ch2 = NewChannel2(apu)
	apu.ch3 = NewChannel3(apu)
	apu.ch4 = NewChannel4(apu)
	apu.Mix = NewMixer(apu)
	return apu
}

func (a *APU) SetEnabled(enabled bool) {
	if !a.enabled && enabled {
		a.divAPU = 0 // frame sequencer phase reset on power up
	}
	a.enabled = enabled
	if !enabled {
		a.ch1.PowerOff()
		a.ch2.PowerOff()
		a.ch3.PowerOff()
		a.ch4.PowerOff()
		a.Mix.Reset()
	}
}

func (a *APU) Read(addr uint16) uint8 {
	switch {
	case slices.Contains([]uint16{register.NR10, register.NR11, register.NR12, register.NR13, register.NR14}, addr):
		return a.ch1.Read(addr)
	case slices.Contains([]uint16{register.NR21, register.NR22, register.NR23, register.NR24}, addr):
		return a.ch2.Read(addr)
	case slices.Contains([]uint16{register.NR30, register.NR31, register.NR32, register.NR33, register.NR34}, addr):
		return a.ch3.Read(addr)
	case addr >= 0xFF30 && addr <= 0xFF3F:
		// Wave pattern RAM
		return a.ch3.Read(addr)
	case slices.Contains([]uint16{register.NR41, register.NR42, register.NR43, register.NR44}, addr):
		return a.ch4.Read(addr)
	case addr == register.NR50:
		return a.Mix.nr50
	case addr == register.NR51:
		return a.Mix.nr51
	case addr == register.NR52:
		return bits.BoolToBit(a.Enabled())<<7 |
			0b01110000 |
			bits.BoolToBit(a.ch4.Enabled())<<3 |
			bits.BoolToBit(a.ch3.Enabled())<<2 |
			bits.BoolToBit(a.ch2.Enabled())<<1 |
			bits.BoolToBit(a.ch1.Enabled())
	default:
		return 0xFF
	}
}

func (a *APU) Write(addr uint16, value uint8) {
	if !a.Enabled() && addr != register.NR52 {
		// On DMG, the hidden length counters and wave RAM remain writable while the APU is powered off.
		// Other sound register writes are ignored until NR52 powers the APU back on.
		switch addr {
		case register.NR11:
			a.ch1.length.Write(value)
		case register.NR21:
			a.ch2.length.Write(value)
		case register.NR31:
			a.ch3.length.Write(value)
		case register.NR41:
			a.ch4.length.Write(value)
		}
		if addr >= 0xFF30 && addr <= 0xFF3F {
			a.ch3.Write(addr, value)
		}
		return
	}

	switch {
	case slices.Contains([]uint16{register.NR10, register.NR11, register.NR12, register.NR13, register.NR14}, addr):
		a.ch1.Write(addr, value)
	case slices.Contains([]uint16{register.NR21, register.NR22, register.NR23, register.NR24}, addr):
		a.ch2.Write(addr, value)
	case slices.Contains([]uint16{register.NR30, register.NR31, register.NR32, register.NR33, register.NR34}, addr):
		a.ch3.Write(addr, value)
	case addr >= 0xFF30 && addr <= 0xFF3F:
		// Wave pattern RAM
		a.ch3.Write(addr, value)
	case slices.Contains([]uint16{register.NR41, register.NR42, register.NR43, register.NR44}, addr):
		a.ch4.Write(addr, value)
	case addr == register.NR50:
		a.Mix.nr50 = value
	case addr == register.NR51:
		a.Mix.nr51 = value
	case addr == register.NR52:
		a.SetEnabled(bits.BitToBool(value & (1 << 7)))
	}
}

func (a *APU) Enabled() bool {
	return a.enabled
}

func (a *APU) SetSpeed(speed float64) {
	a.Mix.SetSpeed(speed)
}

func (a *APU) SetVolume(volume float64) {
	a.Mix.SetVolume(volume)
}

func (a *APU) Volume() float64 {
	return a.Mix.Volume()
}

func (a *APU) BufferedSamples() int {
	return a.Mix.BufferedSamples()
}

func (a *APU) Tick(cycles int) {
	a.ch1.Tick(cycles)
	a.ch2.Tick(cycles)
	a.ch3.Tick(cycles)
	a.ch4.Tick(cycles)
	a.Mix.Tick(cycles)
}

func (a *APU) StepSequencer() {
	step := a.divAPU

	// On step 0, 2, 4, 6: // Sound length event (256 Hz)
	if step&0b001 == 0 {
		a.ch1.length.Tick()
		a.ch2.length.Tick()
		a.ch3.length.Tick()
		a.ch4.length.Tick()
	}

	// On step 2, 6: CH1 freq sweep event (128 Hz)
	if step&0b011 == 0b010 {
		a.ch1.TickSweepTimer()
	}

	// On step 7: Envelope sweep event (64 Hz)
	if step == 0b111 {
		a.ch1.envelope.Tick()
		a.ch2.envelope.Tick()
		a.ch4.envelope.Tick()
	}

	a.divAPU = (a.divAPU + 1) & 0b111
}

func (a *APU) NextStepClocksLength() bool {
	next := a.divAPU
	return next&0b001 == 0
}

type Sampler interface {
	Sample()
}

type Mixer struct {
	apu         *APU
	sampleTimer float64
	speed       float64
	volume      float64
	buffer      *RingBuffer[StereoSample]
	nr50        uint8
	nr51        uint8
	lastSample  StereoSample
	logger      *slog.Logger
}

func NewMixer(apu *APU) *Mixer {
	return &Mixer{
		apu:         apu,
		sampleTimer: 0,
		speed:       1,
		volume:      1,
		buffer:      NewRingBuffer[StereoSample](mixerBufferSamples),
		logger:      apu.logger,
	}
}

func (m *Mixer) Reset() {
	m.sampleTimer = 0
	m.buffer = NewRingBuffer[StereoSample](mixerBufferSamples)
	m.lastSample = StereoSample{}
	m.nr50 = 0
	m.nr51 = 0
}

func (m *Mixer) SetSpeed(speed float64) {
	if speed <= 0 {
		speed = 1
	}
	m.speed = speed
}

func (m *Mixer) SetVolume(volume float64) {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	m.volume = volume
}

func (m *Mixer) Volume() float64 {
	return m.volume
}

func (m *Mixer) BufferedSamples() int {
	return m.buffer.Len()
}

func (m *Mixer) Tick(cycles int) {
	m.sampleTimer += float64(cycles)
	cyclesPerOutputSample := float64(cpuClockRate) * m.speed / float64(sampleRate)

	for m.sampleTimer >= cyclesPerOutputSample {
		m.sampleTimer -= cyclesPerOutputSample

		samplers := [4]DigitalSampler{
			m.apu.ch1,
			m.apu.ch2,
			m.apu.ch3,
			m.apu.ch4,
		}

		leftRaw, rightRaw := 0, 0
		for i, sampler := range samplers {
			sample, ok := sampler.DigitalSample()
			if !ok {
				continue
			}
			analog := dac(sample)

			if m.leftEnabled(i) {
				leftRaw += analog
			}
			if m.rightEnabled(i) {
				rightRaw += analog
			}
		}

		leftPCM := int16(float64(leftRaw*m.leftVolume()*32767/(60*8*2)) * m.volume)
		rightPCM := int16(float64(rightRaw*m.rightVolume()*32767/(60*8*2)) * m.volume)
		m.buffer.Push(StereoSample{Left: leftPCM, Right: rightPCM})
	}
}

func (m *Mixer) Read(p []byte) (n int, err error) {
	requestedSamples := len(p) / 4
	m.dropExcessLatency(requestedSamples)

	for i := range requestedSamples {
		sample, ok := m.buffer.Pop()
		if !ok {
			sample = m.lastSample
		} else {
			m.lastSample = sample
		}

		p[i*4] = byte(sample.Left)
		p[i*4+1] = byte(sample.Left >> 8)
		p[i*4+2] = byte(sample.Right)
		p[i*4+3] = byte(sample.Right >> 8)
	}

	return len(p), nil
}

func (m *Mixer) dropExcessLatency(requestedSamples int) {
	// Trim before serving an audio callback, not while producing each sample.
	// Use hysteresis: small drift is less audible than frequent sample drops.
	buffered := m.buffer.Len()
	highBufferedSamples := targetBufferedSamples*2 + requestedSamples
	if buffered <= highBufferedSamples {
		return
	}

	keepBufferedSamples := targetBufferedSamples + requestedSamples
	if extra := buffered - keepBufferedSamples; extra > 0 {
		m.buffer.DiscardOldest(extra)
	}
}

func (m *Mixer) rightVolume() int {
	return int(m.nr50&0x07) + 1
}

func (m *Mixer) leftVolume() int {
	return int((m.nr50>>4)&0x07) + 1
}

func (m *Mixer) rightEnabled(chIndex int) bool {
	return m.nr51&(1<<chIndex) != 0
}

func (m *Mixer) leftEnabled(chIndex int) bool {
	return m.nr51&(1<<(chIndex+4)) != 0
}

type FrameSequencer interface {
	StepSequencer()
}

func dac(digital uint8) int {
	return 15 - int(digital)*2
}

type StereoSample struct {
	Left  int16
	Right int16
}

type DigitalSampler interface {
	DigitalSample() (sample uint8, ok bool)
}
