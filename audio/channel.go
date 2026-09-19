package audio

type Channel interface {
	Enabled() bool
	Disable()
	Trigger()
	Read(addr uint16) uint8
	Write(addr uint16, value uint8)
	DigitalSample() uint8
}

type ChannelDisabler interface {
	Disable()
}
