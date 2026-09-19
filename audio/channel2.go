package audio

import "github.com/rock619/gbh/register"

type Channel2 struct {
	*SquareWaveChannel
}

func NewChannel2(apu *APU) *Channel2 {
	return &Channel2{
		SquareWaveChannel: NewSquareWaveChannel(apu, register.NR21, register.NR22, register.NR23, register.NR24),
	}
}
