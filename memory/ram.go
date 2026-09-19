package memory

type RAM []uint8

func NewRAM(size int) RAM {
	return make(RAM, size)
}

func (r RAM) Read(addr uint16) uint8 {
	return r[addr]
}

func (r RAM) Write(addr uint16, value uint8) {
	r[addr] = value
}
