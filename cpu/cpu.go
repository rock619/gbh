package cpu

import (
	"fmt"
	"log/slog"

	"github.com/rock619/gbh/bits"
	"github.com/rock619/gbh/interrupt"
	"github.com/rock619/gbh/memorybus"
)

var ErrStopped = fmt.Errorf("CPU stopped")

type Ticker interface {
	Tick(cycles int)
}

type CPU struct {
	mem memorybus.Memory
	// 8 bit registers
	// A = accumulator, F = flag register, B, C, D, E, H, L are general purpose registers
	// Z = zero flag, N = subtract flag, H = half carry flag, C = carry flag
	a, f, b, c, d, e, h, l Register8Bit
	// 16 bit registers
	// PC = program counter, SP = stack pointer
	pc, sp       Register16Bit
	instructions []Instruction
	logger       *slog.Logger
	done         bool
	stopped      bool
	halted       bool
	// haltBug is set when the HALT bug occurs. HALT bug occurs when HALT is executed while interrupts are disabled and
	// at least one interrupt is pending. In this case, the program counter does not increment after fetching the HALT
	// opcode, causing the next instruction to be executed twice.
	// cf. https://gbdev.io/pandocs/halt.html?highlight=halt#halt-bug
	haltBug bool
	IC      *interrupt.Controller
	tickers []Ticker
	cycles  int
}

func New(mem memorybus.Memory, ic *interrupt.Controller, tickers []Ticker, logger *slog.Logger) *CPU {
	cpu := &CPU{
		instructions: NewInstructions(),
		logger:       logger.With("component", "CPU"),
		mem:          mem,
		IC:           ic,
		tickers:      tickers,
	}

	// Initialize registers to their default values after boot ROM execution
	// https://gbdev.io/pandocs/Power_Up_Sequence.html#hardware-registers
	cpu.SetA(0x01)
	cpu.SetF(0xB0)
	cpu.SetB(0x00)
	cpu.SetC(0x13)
	cpu.SetD(0x00)
	cpu.SetE(0xD8)
	cpu.SetH(0x01)
	cpu.SetL(0x4D)
	cpu.SetSP(0xFFFE)
	cpu.SetPC(0x0100)

	return cpu
}

func (cpu *CPU) TickM() {
	cpu.cycles += 4
	for _, t := range cpu.tickers {
		t.Tick(4)
	}
}

func (cpu *CPU) IdleM() {
	cpu.TickM()
}

func (cpu *CPU) Step() (cycles int, err error) {
	cpu.cycles = 0
	if cpu.IsDone() {
		return 0, ErrStopped
	}
	if cpu.stopped {
		cpu.IdleM()
		return cpu.cycles, nil
	}
	if cpu.halted && cpu.IC.HasPending() {
		cpu.halted = false
	}

	if intr, ok := cpu.IC.RequestedInterrupt(); ok {
		cpu.HandleInterrupt(intr)
		return cpu.cycles, nil
	}
	if cpu.halted {
		cpu.IdleM() // HALT consumes 4 cycles per step while halted
		return cpu.cycles, nil
	}

	opcode := cpu.Fetch()
	instruction := cpu.instructions[opcode]
	instruction.Execute(cpu)
	cpu.IC.Step() // Update interrupt controller state after each instruction

	return cpu.cycles, nil
}

func (cpu *CPU) HandleInterrupt(intr interrupt.Interrupt) {
	cpu.IC.ResetIF(intr)
	cpu.IC.DisableIME()

	cpu.IdleM() // M1
	cpu.IdleM() // M2

	cpu.DecSP()
	cpu.Write8(cpu.SP(), bits.High(cpu.PC())) // M3
	cpu.DecSP()
	cpu.Write8(cpu.SP(), bits.Low(cpu.PC())) // M4

	cpu.SetPC(intr.HandlerAddr)
	cpu.IdleM() // M5
}

func (cpu *CPU) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("A", int(cpu.A())),
		slog.Int("F", int(cpu.F())),
		slog.Int("B", int(cpu.B())),
		slog.Int("C", int(cpu.C())),
		slog.Int("D", int(cpu.D())),
		slog.Int("E", int(cpu.E())),
		slog.Int("H", int(cpu.H())),
		slog.Int("L", int(cpu.L())),
		slog.Int("PC", int(cpu.PC())),
		slog.Int("SP", int(cpu.SP())),
	)
}

func (cpu *CPU) A() uint8 {
	return uint8(cpu.a)
}

func (cpu *CPU) SetA(value uint8) {
	cpu.a.Set(value)
}

func (cpu *CPU) F() uint8 {
	return uint8(cpu.f)
}

func (cpu *CPU) SetF(value uint8) {
	cpu.f.Set(value & 0xF0) // Lower 4 bits of F are always 0
}

func (cpu *CPU) B() uint8 {
	return uint8(cpu.b)
}

func (cpu *CPU) SetB(value uint8) {
	cpu.b.Set(value)
}

func (cpu *CPU) C() uint8 {
	return uint8(cpu.c)
}

func (cpu *CPU) SetC(value uint8) {
	cpu.c.Set(value)
}

func (cpu *CPU) D() uint8 {
	return uint8(cpu.d)
}

func (cpu *CPU) SetD(value uint8) {
	cpu.d.Set(value)
}

func (cpu *CPU) E() uint8 {
	return uint8(cpu.e)
}

func (cpu *CPU) SetE(value uint8) {
	cpu.e.Set(value)
}

func (cpu *CPU) H() uint8 {
	return uint8(cpu.h)
}

func (cpu *CPU) SetH(value uint8) {
	cpu.h.Set(value)
}

func (cpu *CPU) L() uint8 {
	return uint8(cpu.l)
}

func (cpu *CPU) SetL(value uint8) {
	cpu.l.Set(value)
}

func (cpu *CPU) AF() uint16 {
	return bits.Combine(cpu.A(), cpu.F())
}

func (cpu *CPU) SetAF(value uint16) {
	cpu.SetA(bits.High(value))
	cpu.SetF(bits.Low(value))
}

func (cpu *CPU) BC() uint16 {
	return bits.Combine(cpu.B(), cpu.C())
}

func (cpu *CPU) SetBC(value uint16) {
	cpu.SetB(bits.High(value))
	cpu.SetC(bits.Low(value))
}

func (cpu *CPU) DE() uint16 {
	return bits.Combine(cpu.D(), cpu.E())
}

func (cpu *CPU) SetDE(value uint16) {
	cpu.SetD(bits.High(value))
	cpu.SetE(bits.Low(value))
}

func (cpu *CPU) HL() uint16 {
	return bits.Combine(cpu.H(), cpu.L())
}

func (cpu *CPU) SetHL(value uint16) {
	cpu.SetH(bits.High(value))
	cpu.SetL(bits.Low(value))
}

func (cpu *CPU) ZFlag() bool {
	return cpu.F()&(1<<7) != 0
}

func (cpu *CPU) SetZFlag(value bool) {
	if value {
		cpu.SetF(cpu.F() | (1 << 7))
	} else {
		cpu.SetF(cpu.F() &^ (1 << 7))
	}
}

func (cpu *CPU) NFlag() bool {
	return cpu.F()&(1<<6) != 0
}

func (cpu *CPU) SetNFlag(value bool) {
	if value {
		cpu.SetF(cpu.F() | (1 << 6))
	} else {
		cpu.SetF(cpu.F() &^ (1 << 6))
	}
}

func (cpu *CPU) HFlag() bool {
	return cpu.F()&(1<<5) != 0
}

func (cpu *CPU) SetHFlag(value bool) {
	if value {
		cpu.SetF(cpu.F() | (1 << 5))
	} else {
		cpu.SetF(cpu.F() &^ (1 << 5))
	}
}

func (cpu *CPU) CFlag() bool {
	return cpu.F()&(1<<4) != 0
}

func (cpu *CPU) SetCFlag(value bool) {
	if value {
		cpu.SetF(cpu.F() | (1 << 4))
	} else {
		cpu.SetF(cpu.F() &^ (1 << 4))
	}
}

func (cpu *CPU) PC() uint16 {
	return uint16(cpu.pc)
}

func (cpu *CPU) SetPC(value uint16) {
	cpu.pc.Set(value)
}

func (cpu *CPU) IncPC() {
	cpu.pc.Set(cpu.PC() + 1)
}

func (cpu *CPU) SP() uint16 {
	return uint16(cpu.sp)
}

func (cpu *CPU) SPHigh() uint8 {
	return uint8(cpu.SP() >> 8)
}

func (cpu *CPU) SPLow() uint8 {
	return uint8(cpu.SP() & 0xFF)
}

func (cpu *CPU) SetSP(value uint16) {
	cpu.sp.Set(value)
}

// Fetch fetches uint8 value at the program counter from memory and increments the program counter
func (cpu *CPU) Fetch() uint8 {
	v := cpu.mem.Read(cpu.PC())
	// HALT bug causes the program counter to not increment after fetching the HALT opcode
	if cpu.haltBug {
		cpu.haltBug = false
	} else {
		cpu.IncPC()
	}
	cpu.TickM()
	return v
}

func (cpu *CPU) Fetch16() uint16 {
	low := cpu.Fetch()
	high := cpu.Fetch()
	return (uint16(high) << 8) | uint16(low)
}

func (cpu *CPU) Read8(addr uint16) uint8 {
	value := cpu.mem.Read(addr)
	cpu.TickM()
	return value
}

func (cpu *CPU) Read16(addr uint16) uint16 {
	low := cpu.Read8(addr)
	high := cpu.Read8(addr + 1)
	return bits.Combine(high, low)
}

func (cpu *CPU) Write8(addr uint16, value uint8) {
	cpu.mem.Write(addr, value)
	cpu.TickM()
}

func (cpu *CPU) Write16(addr uint16, value uint16) {
	cpu.Write8(addr, bits.Low(value))
	cpu.Write8(addr+1, bits.High(value))
}

func (cpu *CPU) IncSP() {
	cpu.SetSP(cpu.SP() + 1)
}

func (cpu *CPU) DecSP() {
	cpu.SetSP(cpu.SP() - 1)
}

func (cpu *CPU) Push(value uint16) {
	cpu.MaybeCorruptOAM(cpu.SP(), OAMBugPush)
	cpu.IdleM()

	cpu.DecSP()
	cpu.MaybeCorruptOAM(cpu.SP(), OAMBugPush)
	cpu.Write8(cpu.SP(), bits.High(value))

	cpu.DecSP()
	cpu.MaybeCorruptOAM(cpu.SP(), OAMBugPush)
	cpu.Write8(cpu.SP(), bits.Low(value))
}

func (cpu *CPU) Pop() uint16 {
	low := cpu.mem.Read(cpu.SP())
	cpu.MaybeCorruptOAM(cpu.SP(), OAMBugPop)
	cpu.TickM()
	cpu.IncSP()

	high := cpu.mem.Read(cpu.SP())
	cpu.MaybeCorruptOAM(cpu.SP(), OAMBugPop)
	cpu.TickM()
	cpu.IncSP()

	return bits.Combine(high, low)
}

func (cpu *CPU) IsDone() bool {
	return cpu.done
}

func (cpu *CPU) Stop() {
	cpu.stopped = true
}

func (cpu *CPU) Wake() {
	cpu.stopped = false
}

func (cpu *CPU) Halt() {
	if !cpu.IC.IME() && cpu.IC.HasPending() {
		cpu.haltBug = true
		return
	}
	cpu.halted = true
}

type OAMBugKind int

const (
	// OAMBugIncDec INC r16 / DEC r16
	OAMBugIncDec OAMBugKind = iota
	// OAMBugPop POP r16
	OAMBugPop
	// OAMBugPush PUSH r16
	OAMBugPush
	// OAMBugLdHLReadIncDec LD A,(HL+) / LD A,(HL-)
	OAMBugLdHLReadIncDec
	// OAMBugLdHLWriteIncDec LD (HL+),A / LD (HL-),A
	OAMBugLdHLWriteIncDec
)

func (cpu *CPU) MaybeCorruptOAM(addr uint16, kind OAMBugKind) {
	if b, ok := cpu.mem.(OAMCorrupter); ok {
		b.CorruptOAM(addr, kind)
	}
}

type Register8Bit uint8

func (r *Register8Bit) Get() uint8 {
	return uint8(*r)
}

func (r *Register8Bit) Set(value uint8) {
	*r = Register8Bit(value)
}

type Register16Bit uint16

func (r *Register16Bit) Get() uint16 {
	return uint16(*r)
}

func (r *Register16Bit) Set(value uint16) {
	*r = Register16Bit(value)
}

type OAMCorrupter interface {
	CorruptOAM(addr uint16, kind OAMBugKind)
}
