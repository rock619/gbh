package serial

import (
	"bytes"
	"testing"

	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/register"
)

type recordingRequester struct {
	requested []interrupt.Interrupt
}

func (r *recordingRequester) Request(intr interrupt.Interrupt) {
	r.requested = append(r.requested, intr)
}

type bitCable struct {
	in   []bool
	out  []bool
	next int
}

func (c *bitCable) ExchangeBit(out bool) (bool, bool) {
	c.out = append(c.out, out)
	if c.next >= len(c.in) {
		return true, true
	}
	in := c.in[c.next]
	c.next++
	return in, true
}

type pendingCable struct {
	ready bool
}

func (c *pendingCable) ExchangeBit(bool) (bool, bool) {
	return false, c.ready
}

func TestInternalClockTransferCompletesAfterEightBits(t *testing.T) {
	ir := &recordingRequester{}
	var out bytes.Buffer
	s := New(ir, &out)

	s.Write(register.SB, 0xA5)
	s.Write(register.SC, 0x81)

	if got := out.Len(); got != 0 {
		t.Fatalf("serial output before transfer completes: got %d bytes, want 0", got)
	}
	if got := len(ir.requested); got != 0 {
		t.Fatalf("interrupts before transfer completes: got %d, want 0", got)
	}
	if got := s.Read(register.SC) & 0x80; got == 0 {
		t.Fatalf("start bit before transfer completes: got cleared, want set")
	}

	s.Tick(4095)

	if got := out.Len(); got != 0 {
		t.Fatalf("serial output at 4095 cycles: got %d bytes, want 0", got)
	}
	if got := len(ir.requested); got != 0 {
		t.Fatalf("interrupts at 4095 cycles: got %d, want 0", got)
	}

	s.Tick(1)

	if got, want := out.Bytes(), []byte{0xA5}; !bytes.Equal(got, want) {
		t.Fatalf("serial output after transfer: got % X, want % X", got, want)
	}
	if got, want := s.Read(register.SB), uint8(0xFF); got != want {
		t.Fatalf("received byte without cable: got %02X, want %02X", got, want)
	}
	if got := s.Read(register.SC) & 0x80; got != 0 {
		t.Fatalf("start bit after transfer completes: got set, want cleared")
	}
	if got, want := len(ir.requested), 1; got != want {
		t.Fatalf("interrupt requests: got %d, want %d", got, want)
	}
	if got, want := ir.requested[0], interrupt.Serial; got != want {
		t.Fatalf("interrupt request: got %+v, want %+v", got, want)
	}
}

func TestCableBitsAreExchangedMSBFirst(t *testing.T) {
	s := New(nil, nil)
	cable := &bitCable{
		in: []bool{true, false, true, false, false, true, false, true}, // 0xA5
	}
	s.Connect(cable)

	s.Write(register.SB, 0x3C)
	s.Write(register.SC, 0x81)
	s.Tick(4096)

	if got, want := s.Read(register.SB), uint8(0xA5); got != want {
		t.Fatalf("received byte: got %02X, want %02X", got, want)
	}

	wantOut := []bool{false, false, true, true, true, true, false, false} // 0x3C
	if len(cable.out) != len(wantOut) {
		t.Fatalf("sent bit count: got %d, want %d", len(cable.out), len(wantOut))
	}
	for i, want := range wantOut {
		if got := cable.out[i]; got != want {
			t.Fatalf("sent bit %d: got %t, want %t", i, got, want)
		}
	}
}

func TestExternalClockStartDoesNotSelfClock(t *testing.T) {
	ir := &recordingRequester{}
	s := New(ir, nil)

	s.Write(register.SB, 0x12)
	s.Write(register.SC, 0x80)
	s.Tick(4096)

	if got, want := s.Read(register.SB), uint8(0x12); got != want {
		t.Fatalf("SB changed without internal clock: got %02X, want %02X", got, want)
	}
	if got := len(ir.requested); got != 0 {
		t.Fatalf("interrupt requests without internal clock: got %d, want 0", got)
	}
}

func TestExternalClockTransferDoesNotRequireStartBit(t *testing.T) {
	ir := &recordingRequester{}
	s := New(ir, nil)
	s.Write(register.SB, 0x3C)

	in := []bool{true, false, true, false, false, true, false, true} // 0xA5
	wantOut := []bool{false, false, true, true, true, true, false, false}
	for i := range in {
		out, ok := s.ClockExternalBit(in[i])
		if !ok {
			t.Fatalf("external clock bit %d was rejected", i)
		}
		if out != wantOut[i] {
			t.Fatalf("external clock output bit %d: got %t, want %t", i, out, wantOut[i])
		}
	}

	if got, want := s.Read(register.SB), uint8(0xA5); got != want {
		t.Fatalf("received byte: got %02X, want %02X", got, want)
	}
	if got, want := len(ir.requested), 1; got != want {
		t.Fatalf("interrupt requests: got %d, want %d", got, want)
	}
}

func TestInternalClockWaitsForCableResponse(t *testing.T) {
	s := New(nil, nil)
	cable := &pendingCable{}
	s.Connect(cable)

	s.Write(register.SB, 0xFF)
	s.Write(register.SC, 0x81)
	s.Tick(512)

	if got := s.Read(register.SC) & 0x80; got == 0 {
		t.Fatal("transfer completed while cable response was pending")
	}
	if got, want := s.bitIndex, uint8(0); got != want {
		t.Fatalf("bit index while cable response was pending: got %d, want %d", got, want)
	}

	cable.ready = true
	s.Tick(4)

	if got, want := s.bitIndex, uint8(1); got != want {
		t.Fatalf("bit index after cable response: got %d, want %d", got, want)
	}
}
