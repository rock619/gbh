package serial

import (
	"io"
	"sync"

	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/register"
)

type Serial struct {
	mu           sync.Mutex
	cyclesPerBit int

	sb uint8
	sc uint8

	ir    interrupt.Requester
	out   io.Writer
	cable LinkCable

	transferring bool  // Whether a transfer is in progress
	tx           uint8 // Byte being transmitted
	rx           uint8 // Byte being received
	bitIndex     uint8 // Number of bits transferred: 0..8
	cycles       int   // Cycles remaining until the next bit is transferred
}

type State struct {
	SB            uint8
	SC            uint8
	Transferring  bool
	InternalClock bool
	BitIndex      uint8
}

func New(ir interrupt.Requester, out io.Writer) *Serial {
	return &Serial{
		cyclesPerBit: 512, // 8192 bps (4.194304 MHz / 512)
		ir:           ir,
		out:          out,
	}
}

func (s *Serial) Tick(cycles int) {
	s.mu.Lock()
	if !s.transferring || !s.internalClock() {
		s.mu.Unlock()
		return
	}

	s.cycles -= cycles
	for s.transferring && s.cycles <= 0 {
		out := s.nextOutBit()
		cable := s.cable
		s.mu.Unlock()

		in := true
		if cable != nil {
			bit, ok := cable.ExchangeBit(out)
			if !ok {
				s.mu.Lock()
				s.cycles = 0
				s.mu.Unlock()
				return
			}
			in = bit
		}

		s.mu.Lock()
		if !s.transferring || !s.internalClock() {
			s.mu.Unlock()
			return
		}
		s.receiveBit(in)
		s.cycles += s.cyclesPerBit
	}
	s.mu.Unlock()
}

func (s *Serial) ClockExternalBit(in bool) (out bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.internalClock() {
		return true, false
	}
	if !s.transferring {
		s.startTransfer()
	}

	out = s.nextOutBit()
	s.receiveBit(in)
	return out, true
}

func (s *Serial) internalClock() bool {
	return s.sc&0x01 != 0
}

func (s *Serial) nextOutBit() bool {
	return s.tx&(0x80>>s.bitIndex) != 0
}

func (s *Serial) receiveBit(in bool) {
	s.rx <<= 1
	if in {
		s.rx |= 1
	}

	s.bitIndex++
	if s.bitIndex == 8 {
		s.finishTransfer()
	}
}

func (s *Serial) finishTransfer() {
	s.sb = s.rx
	s.transferring = false
	s.sc &^= 0x80

	if s.out != nil {
		_, _ = s.out.Write([]byte{s.tx})
	}
	if s.ir != nil {
		s.ir.Request(interrupt.Serial)
	}
}

func (s *Serial) Read(addr uint16) uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch addr {
	case register.SB:
		return s.sb
	case register.SC:
		return s.sc | 0x7E // unused bits read as 1
	default:
		return 0xFF
	}
}

func (s *Serial) Write(addr uint16, value uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch addr {
	case register.SB:
		s.sb = value

	case register.SC:
		s.sc = value

		if value&0x80 != 0 {
			s.startTransfer()
		}
	}
}

func (s *Serial) startTransfer() {
	s.transferring = true
	s.tx = s.sb
	s.rx = 0
	s.bitIndex = 0
	s.cycles = s.cyclesPerBit
}

func (s *Serial) Connect(c LinkCable) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cable = c
}

func (s *Serial) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return State{
		SB:            s.sb,
		SC:            s.sc | 0x7E,
		Transferring:  s.transferring,
		InternalClock: s.internalClock(),
		BitIndex:      s.bitIndex,
	}
}

type LinkCable interface {
	ExchangeBit(out bool) (in bool, ok bool)
}
