package audio

type Length struct {
	channel ChannelDisabler

	initialTimerBits uint8
	enabled          bool
	timerLimit       uint16
	timer            uint16
}

func NewLength(channel ChannelDisabler, initialTimerBits uint8) *Length {
	return &Length{
		channel:          channel,
		initialTimerBits: initialTimerBits,
		timerLimit:       1 << initialTimerBits,
	}
}

func (l *Length) Reset() {
	l.enabled = false
	l.timerLimit = 1 << l.initialTimerBits
	l.timer = 0
}

func (l *Length) Enabled() bool {
	return l.enabled
}

func (l *Length) Disable() {
	l.enabled = false
}

func (l *Length) WriteEnabled(enabled, trigger, nextStepClocksLength bool) {
	// https://gbdev.io/pandocs/Audio_details.html#obscure-behavior
	// Extra length clocking occurs when writing to NRx4 when the DIV-APU next step is one that doesn't clock the
	// length timer. In this case, if the length timer was PREVIOUSLY disabled and now enabled and the length timer is
	// not zero, it is decremented. If this decrement makes it zero and trigger is clear, the channel is disabled.
	if !l.enabled && enabled && !nextStepClocksLength && l.timer != 0 {
		l.timer--
		if l.timer == 0 && !trigger {
			l.channel.Disable()
		}
	}
	l.enabled = enabled
}

func (l *Length) SetInitialTimer(v uint8) {
	l.timer = l.timerLimit - uint16(v)
}

func (l *Length) ResetTimerIfExpired(nextStepClocksLength bool) {
	if l.timer == 0 {
		l.timer = l.timerLimit
		// https://gbdev.io/pandocs/Audio_details.html#obscure-behavior
		// If a channel is triggered when the DIV-APU next step is one that doesn't clock the length timer and the
		// length timer is now enabled and length is being set to 64 (256 for wave channel) because it was previously
		// zero, it is set to 63 instead (255 for wave channel).
		if l.enabled && !nextStepClocksLength {
			l.timer--
		}
	}
}

func (l *Length) Tick() {
	if !l.enabled || l.timer == 0 {
		return
	}

	l.timer--
	if l.timer == 0 {
		l.channel.Disable()
	}
}

func (l *Length) Read() uint8 {
	return 0xFF // Length register is write-only
}

func (l *Length) Write(value uint8) {
	mask := uint8(1)<<l.initialTimerBits - 1
	l.SetInitialTimer(value & mask)
}
