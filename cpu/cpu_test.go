package cpu

import (
	"log/slog"
	"testing"

	"github.com/rock619/gbh/interrupt"
)

type testMemory [0x10000]uint8

func (m *testMemory) Read(addr uint16) uint8 {
	return m[addr]
}

func (m *testMemory) Write(addr uint16, value uint8) {
	m[addr] = value
}

func TestStopCanWakeAndResumeExecution(t *testing.T) {
	mem := &testMemory{}
	mem[0x0100] = 0x10 // STOP
	mem[0x0101] = 0x00
	mem[0x0102] = 0x04 // INC B

	c := New(mem, interrupt.New(), nil, slog.Default())

	if _, err := c.Step(); err != nil {
		t.Fatalf("STOP step: %v", err)
	}
	if got, want := c.PC(), uint16(0x0102); got != want {
		t.Fatalf("PC after STOP: got %04X, want %04X", got, want)
	}

	if _, err := c.Step(); err != nil {
		t.Fatalf("stopped idle step: %v", err)
	}
	if got, want := c.B(), uint8(0x00); got != want {
		t.Fatalf("B while stopped: got %02X, want %02X", got, want)
	}

	c.Wake()

	if _, err := c.Step(); err != nil {
		t.Fatalf("step after wake: %v", err)
	}
	if got, want := c.B(), uint8(0x01); got != want {
		t.Fatalf("B after wake: got %02X, want %02X", got, want)
	}
}
