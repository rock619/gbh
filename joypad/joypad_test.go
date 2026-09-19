package joypad

import (
	"testing"

	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/register"
)

type noopRequester struct{}

func (noopRequester) Request(interrupt.Interrupt) {}

type recordingRequester struct {
	requested []interrupt.Interrupt
}

func (r *recordingRequester) Request(intr interrupt.Interrupt) {
	r.requested = append(r.requested, intr)
}

func TestReadReturnsOnlySelectedGroup(t *testing.T) {
	j := New(noopRequester{})
	j.SetPressed([]KeyCode{KeyUp, KeyA})

	j.Write(register.P1JOYP, 0x20)
	if got, want := j.Read(register.P1JOYP)&0x0F, uint8(0b1011); got != want {
		t.Fatalf("d-pad selected: got low bits %04b, want %04b", got, want)
	}

	j.Write(register.P1JOYP, 0x10)
	if got, want := j.Read(register.P1JOYP)&0x0F, uint8(0b1110); got != want {
		t.Fatalf("buttons selected: got low bits %04b, want %04b", got, want)
	}
}

func TestPressedTransitionRequestsJoypadInterrupt(t *testing.T) {
	ir := &recordingRequester{}
	j := New(ir)
	j.Write(register.P1JOYP, 0x20)

	j.SetPressed([]KeyCode{KeyRight})
	if got, want := len(ir.requested), 1; got != want {
		t.Fatalf("interrupt requests: got %d, want %d", got, want)
	}
	if got, want := ir.requested[0], interrupt.Joypad; got != want {
		t.Fatalf("interrupt request: got %+v, want %+v", got, want)
	}

	j.SetPressed([]KeyCode{KeyRight})
	if got, want := len(ir.requested), 1; got != want {
		t.Fatalf("held key should not request another interrupt: got %d requests, want %d", got, want)
	}

	j.SetPressed(nil)
	if got, want := len(ir.requested), 1; got != want {
		t.Fatalf("release should not request another interrupt: got %d requests, want %d", got, want)
	}
}

func TestDirectionDoesNotReadAsButton(t *testing.T) {
	j := New(noopRequester{})
	j.SetPressed([]KeyCode{KeyRight})

	j.Write(register.P1JOYP, 0x10)
	if got, want := j.Read(register.P1JOYP)&0x0F, uint8(0b1111); got != want {
		t.Fatalf("buttons selected with only right pressed: got low bits %04b, want %04b", got, want)
	}

	j.Write(register.P1JOYP, 0x20)
	if got, want := j.Read(register.P1JOYP)&0x0F, uint8(0b1110); got != want {
		t.Fatalf("d-pad selected with right pressed: got low bits %04b, want %04b", got, want)
	}
}
