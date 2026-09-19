package cpu

import (
	"fmt"
	"slices"

	"github.com/rock619/gbh/bits"
)

type Instruction struct {
	Opcode   uint8
	Mnemonic string
	Length   int
	Execute  func(*CPU)
}

func (i Instruction) String() string {
	return fmt.Sprintf("CPU Instruction %s (0x%02X) Length: %d bytes", i.Mnemonic, i.Opcode, i.Length)
}

type Register8 struct {
	Name string
	Get  func(cpu *CPU) uint8
	Set  func(cpu *CPU, v uint8)
}

var R8s = []Register8{
	{
		Name: "B",
		Get:  func(cpu *CPU) uint8 { return cpu.B() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetB(v) },
	},
	{
		Name: "C",
		Get:  func(cpu *CPU) uint8 { return cpu.C() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetC(v) },
	},
	{
		Name: "D",
		Get:  func(cpu *CPU) uint8 { return cpu.D() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetD(v) },
	},
	{
		Name: "E",
		Get:  func(cpu *CPU) uint8 { return cpu.E() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetE(v) },
	},
	{
		Name: "H",
		Get:  func(cpu *CPU) uint8 { return cpu.H() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetH(v) },
	},
	{
		Name: "L",
		Get:  func(cpu *CPU) uint8 { return cpu.L() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetL(v) },
	},
	{
		Name: "[HL]",
		Get:  func(cpu *CPU) uint8 { return cpu.Read8(cpu.HL()) },
		Set:  func(cpu *CPU, v uint8) { cpu.Write8(cpu.HL(), v) },
	},
	{
		Name: "A",
		Get:  func(cpu *CPU) uint8 { return cpu.A() },
		Set:  func(cpu *CPU, v uint8) { cpu.SetA(v) },
	},
}

type Register16 struct {
	Name string
	Get  func(cpu *CPU) uint16
	Set  func(cpu *CPU, v uint16)
}

var R16s = []Register16{
	{
		Name: "BC",
		Get:  func(cpu *CPU) uint16 { return cpu.BC() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetBC(v) },
	},
	{
		Name: "DE",
		Get:  func(cpu *CPU) uint16 { return cpu.DE() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetDE(v) },
	},
	{
		Name: "HL",
		Get:  func(cpu *CPU) uint16 { return cpu.HL() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetHL(v) },
	},
	{
		Name: "SP",
		Get:  func(cpu *CPU) uint16 { return cpu.SP() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetSP(v) },
	},
}

var R16Stacks = []Register16{
	{
		Name: "BC",
		Get:  func(cpu *CPU) uint16 { return cpu.BC() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetBC(v) },
	},
	{
		Name: "DE",
		Get:  func(cpu *CPU) uint16 { return cpu.DE() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetDE(v) },
	},
	{
		Name: "HL",
		Get:  func(cpu *CPU) uint16 { return cpu.HL() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetHL(v) },
	},
	{
		Name: "AF",
		Get:  func(cpu *CPU) uint16 { return cpu.AF() },
		Set:  func(cpu *CPU, v uint16) { cpu.SetAF(v) },
	},
}

var R16Mems = []Register8{
	{
		Name: "BC",
		Get:  func(cpu *CPU) uint8 { return cpu.Read8(cpu.BC()) },
		Set:  func(cpu *CPU, v uint8) { cpu.Write8(cpu.BC(), v) },
	},
	{
		Name: "DE",
		Get:  func(cpu *CPU) uint8 { return cpu.Read8(cpu.DE()) },
		Set:  func(cpu *CPU, v uint8) { cpu.Write8(cpu.DE(), v) },
	},
	{
		Name: "HL+",
		Get: func(cpu *CPU) uint8 {
			v := cpu.mem.Read(cpu.HL())
			cpu.MaybeCorruptOAM(cpu.HL(), OAMBugLdHLReadIncDec)
			cpu.TickM()
			cpu.SetHL(cpu.HL() + 1)
			return v
		},
		Set: func(cpu *CPU, v uint8) {
			cpu.Write8(cpu.HL(), v)
			cpu.MaybeCorruptOAM(cpu.HL(), OAMBugLdHLWriteIncDec)
			cpu.SetHL(cpu.HL() + 1)
		},
	},
	{
		Name: "HL-",
		Get: func(cpu *CPU) uint8 {
			v := cpu.mem.Read(cpu.HL())
			cpu.MaybeCorruptOAM(cpu.HL(), OAMBugLdHLReadIncDec)
			cpu.TickM()
			cpu.SetHL(cpu.HL() - 1)
			return v
		}, Set: func(cpu *CPU, v uint8) {
			cpu.Write8(cpu.HL(), v)
			cpu.MaybeCorruptOAM(cpu.HL(), OAMBugLdHLWriteIncDec)
			cpu.SetHL(cpu.HL() - 1)
		},
	},
}

type Condition struct {
	Name string
	Met  func(cpu *CPU) bool
}

var Conds = []Condition{
	{Name: "NZ", Met: func(cpu *CPU) bool { return !cpu.ZFlag() }},
	{Name: "Z", Met: func(cpu *CPU) bool { return cpu.ZFlag() }},
	{Name: "NC", Met: func(cpu *CPU) bool { return !cpu.CFlag() }},
	{Name: "C", Met: func(cpu *CPU) bool { return cpu.CFlag() }},
}

func NewInstructions() []Instruction {
	prefixed := NewPrefixedInstructions()
	instructions := make([]Instruction, 256)
	for opcode := 0; opcode <= 0xFF; opcode++ {
		instructions[opcode] = NewInstruction(uint8(opcode), prefixed)
	}
	return instructions
}

func NewInstruction(opcode uint8, prefixed []Instruction) Instruction {
	switch {
	// Block 0
	case opcode == 0b00000000:
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "NOP",
			Length:   1,
			Execute:  func(cpu *CPU) {},
		}

	case opcode&0b11001111 == 0b00000001:
		// LD r16, n16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("LD %s, n16", R16s[index].Name),
			Length:   3,
			Execute: func(cpu *CPU) {
				v := cpu.Fetch16()
				R16s[index].Set(cpu, v)
			},
		}
	case opcode&0b11001111 == 0b00000010:
		// LD [r16], A
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("LD [%s], A", R16Mems[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				R16Mems[index].Set(cpu, cpu.A())
			},
		}
	case opcode&0b11001111 == 0b00001010:
		// LD A, [r16]
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("LD A, [%s]", R16Mems[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetA(R16Mems[index].Get(cpu))
			},
		}
	case opcode == 0b00001000:
		// LD [a16], SP
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LD [a16], SP",
			Length:   3,
			Execute: func(cpu *CPU) {
				address := cpu.Fetch16()
				cpu.Write16(address, cpu.SP())
			},
		}

	case opcode&0b11001111 == 0b00000011:
		// INC r16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("INC %s", R16s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				v := R16s[index].Get(cpu)
				cpu.MaybeCorruptOAM(v, OAMBugIncDec)
				R16s[index].Set(cpu, v+1)
				// 16-bit arithmetic instructions take an extra cycle
				cpu.IdleM()
			},
		}
	case opcode&0b11001111 == 0b00001011:
		// DEC r16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("DEC %s", R16s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				v := R16s[index].Get(cpu)
				cpu.MaybeCorruptOAM(v, OAMBugIncDec)
				R16s[index].Set(cpu, v-1)
				// 16-bit arithmetic instructions take an extra cycle
				cpu.IdleM()
			},
		}

	case opcode&0b11001111 == 0b00001001:
		// ADD HL, r16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("ADD HL, %s", R16s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Add16(cpu.HL(), R16s[index].Get(cpu), false)
				cpu.SetHL(result)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
				// 16-bit arithmetic instructions take an extra cycle
				cpu.IdleM()
			},
		}

	case opcode&0b11000111 == 0b00000100:
		// INC r8
		index := (opcode & 0b00111000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("INC %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, _ := bits.Add8(R8s[index].Get(cpu), 1, false)
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
			},
		}

	case opcode&0b11000111 == 0b00000101:
		// DEC r8
		index := (opcode & 0b00111000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("DEC %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, _ := bits.Sub8(R8s[index].Get(cpu), 1, false)
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
			},
		}

	case opcode&0b11000111 == 0b00000110:
		// LD r8, n8
		index := (opcode & 0b00111000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("LD %s, n8", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				R8s[index].Set(cpu, n8)
			},
		}

	case opcode == 0b00000111:
		// RLCA https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RLCA
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RLCA",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				result, carry := bits.RotateLeft(cpu.A())
				cpu.SetA(result)
				cpu.SetCFlag(carry)
			},
		}

	case opcode == 0b00001111:
		// RRCA https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RRCA
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RRCA",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				result, carry := bits.RotateRight(cpu.A())
				cpu.SetA(result)
				cpu.SetCFlag(carry)
			},
		}
	case opcode == 0b00010111:
		// RLA https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RLA
		// Rotate the value in register A left through the Carry flag. The bit that was in bit 7 is moved to bit 0 and also copied to the Carry flag.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RLA",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				result, carry := bits.RotateLeftThroughCarry(cpu.A(), cpu.CFlag())
				cpu.SetA(result)
				cpu.SetCFlag(carry)
			},
		}
	case opcode == 0b00011111:
		// RRA https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RRA
		// Rotate the value in register A right through the Carry flag. The bit that was in bit 0 is moved to bit 7 and also copied to the Carry flag.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RRA",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				result, carry := bits.RotateRightThroughCarry(cpu.A(), cpu.CFlag())
				cpu.SetA(result)
				cpu.SetCFlag(carry)
			},
		}
	case opcode == 0b00100111:
		// DAA https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#DAA
		// Decimal Adjust Accumulator.
		// Designed to be used after performing an arithmetic instruction (ADD, ADC, SUB, SBC) whose inputs were in Binary-Coded Decimal (BCD), adjusting the result to likewise be in BCD.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "DAA",
			Length:   1,
			Execute: func(cpu *CPU) {
				if cpu.NFlag() {
					// If the subtract flag N is set:
					// 1. Initialize the adjustment to 0.
					// 2. If the half-carry flag H is set, then add $6 to the adjustment.
					// 3. If the carry flag is set, then add $60 to the adjustment.
					// 4. Subtract the adjustment from A.
					adjustment := uint8(0)
					if cpu.HFlag() {
						adjustment += 0x06
					}
					if cpu.CFlag() {
						adjustment += 0x60
					}
					cpu.SetA(cpu.A() - adjustment)
				} else {
					// If the subtract flag N is not set:
					// 1. Initialize the adjustment to 0.
					// 2. If the half-carry flag H is set or A & $F > $9, then add $6 to the adjustment.
					// 3. If the carry flag is set or A > $99, then add $60 to the adjustment and set the carry flag.
					// 4. Add the adjustment to A.
					adjustment := uint8(0)
					if cpu.HFlag() || (cpu.A()&0x0F) > 0x09 {
						adjustment += 0x06
					}
					if cpu.CFlag() || cpu.A() > 0x99 {
						adjustment += 0x60
						cpu.SetCFlag(true)
					}
					cpu.SetA(cpu.A() + adjustment)
				}
				cpu.SetZFlag(cpu.A() == 0)
				cpu.SetHFlag(false)
			},
		}

	case opcode == 0b00101111:
		// CPL https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#CPL
		// Complement the value in register A (flip all bits).
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "CPL",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetA(^cpu.A())
				cpu.SetNFlag(true)
				cpu.SetHFlag(true)
			},
		}
	case opcode == 0b00110111:
		// SCF https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SCF
		// Set the Carry flag.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "SCF",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(true)
			},
		}

	case opcode == 0b00111111:
		// CCF https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#CCF
		// Complement the Carry flag.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "CCF",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(!cpu.CFlag())
			},
		}

	case opcode == 0b00011000:
		// JR e8
		// Relative Jump to address PC + e8.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "JR e8",
			Length:   2,
			Execute: func(cpu *CPU) {
				e8 := cpu.Fetch()
				result := uint16(int32(cpu.PC()) + int32(int8(e8)))
				cpu.SetPC(result)
				// branch target calculation / PC reload takes an extra cycle
				cpu.IdleM()
			},
		}

	case opcode&0b11100111 == 0b00100000:
		// JR cc, e8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#JR_cc,n16
		// Relative Jump to address PC + e8 if condition Z (Z flag is set) is true.
		index := (opcode & 0b00011000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("JR %s, e8", Conds[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				e8 := cpu.Fetch()
				if Conds[index].Met(cpu) {
					result := uint16(int32(cpu.PC()) + int32(int8(e8)))
					cpu.SetPC(result)
					// branch target calculation / PC reload takes an extra cycle
					cpu.IdleM()
				}
			},
		}

	case opcode == 0b00010000:
		// STOP https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#STOP
		// Enter CPU STOP mode. This halts the CPU until a button is pressed. The screen remains on but the game is effectively paused.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "STOP",
			Length:   2,
			Execute: func(cpu *CPU) {
				cpu.IncPC()
				cpu.Stop()
			},
		}

	// Block 1: 8-bit register-to-register loads
	case opcode == 0b01110110:
		// HALT
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "HALT",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.Halt()
			},
		}

	case opcode&0b11000000 == 0b01000000:
		// LD r8, r8
		src := (opcode & 0b00000111)
		dst := (opcode & 0b00111000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("LD %s, %s", R8s[dst].Name, R8s[src].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				R8s[dst].Set(cpu, R8s[src].Get(cpu))
			},
		}

	// Block 2: 8-bit arithmetic
	case opcode&0b11111000 == 0b10000000:
		// ADD A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("ADD A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Add8(cpu.A(), R8s[index].Get(cpu), false)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode&0b11111000 == 0b10001000:
		// ADC A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("ADC A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Add8(cpu.A(), R8s[index].Get(cpu), cpu.CFlag())
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode&0b11111000 == 0b10010000:
		// SUB A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SUB A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Sub8(cpu.A(), R8s[index].Get(cpu), false)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode&0b11111000 == 0b10011000:
		// SBC A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SBC A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Sub8(cpu.A(), R8s[index].Get(cpu), cpu.CFlag())
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode&0b11111000 == 0b10100000:
		// AND A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("AND A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result := cpu.A() & R8s[index].Get(cpu)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(true)
				cpu.SetCFlag(false)
			},
		}

	case opcode&0b11111000 == 0b10101000:
		// XOR A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("XOR A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result := cpu.A() ^ R8s[index].Get(cpu)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(false)
			},
		}

	case opcode&0b11111000 == 0b10110000:
		// OR A, r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("OR A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result := cpu.A() | R8s[index].Get(cpu)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(false)
			},
		}

	case opcode&0b11111000 == 0b10111000:
		// CP A, r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#CP_A,r8
		// ComPare the value in A with the value in r8.
		// This subtracts the value in r8 from A and sets flags accordingly, but discards the result.
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("CP A, %s", R8s[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				result, hc, c := bits.Sub8(cpu.A(), R8s[index].Get(cpu), false)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	// Block 3
	case opcode == 0b11000110:
		// ADD A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "ADD A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result, hc, c := bits.Add8(cpu.A(), n8, false)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode == 0b11001110:
		// ADC A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "ADC A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result, hc, c := bits.Add8(cpu.A(), n8, cpu.CFlag())
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode == 0b11010110:
		// SUB A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "SUB A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result, hc, c := bits.Sub8(cpu.A(), n8, false)
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode == 0b11011110:
		// SBC A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "SBC A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result, hc, c := bits.Sub8(cpu.A(), n8, cpu.CFlag())
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode == 0b11100110:
		// AND A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "AND A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result := cpu.A() & n8
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(true)
				cpu.SetCFlag(false)
			},
		}

	case opcode == 0b11101110:
		// XOR A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "XOR A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result := cpu.A() ^ n8
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(false)
			},
		}

	case opcode == 0b11110110:
		// OR A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "OR A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result := cpu.A() | n8
				cpu.SetA(result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(false)
			},
		}

	case opcode == 0b11111110:
		// CP A, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "CP A, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				result, hc, c := bits.Sub8(cpu.A(), n8, false)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(true)
				cpu.SetHFlag(hc)
				cpu.SetCFlag(c)
			},
		}

	case opcode == 0b11001001:
		// RET https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RET
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RET",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetPC(cpu.Pop())
				// PC reload / internal branching takes an extra cycle
				cpu.IdleM()
			},
		}

	case opcode == 0b11011001:
		// RETI https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RETI
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "RETI",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetPC(cpu.Pop())
				// PC reload / internal branching takes an extra cycle
				cpu.IdleM()

				cpu.IC.EnableIME()
			},
		}

	case opcode == 0b11000011:
		// JP n16 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#JP_n16
		// Jump to address n16; effectively, copy n16 into PC.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "JP a16",
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				cpu.SetPC(n16)
				// PC reload / internal branching takes an extra cycle
				cpu.IdleM()
			},
		}

	case opcode == 0b11101001:
		// JP HL
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "JP HL",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetPC(cpu.HL())
			},
		}

	case opcode == 0b11001101:
		// CALL n16
		// Call address n16.
		// This pushes the address of the instruction after the CALL on the stack, such that RET can pop it later; then, it executes an implicit JP n16.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "CALL a16",
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				cpu.Push(cpu.PC())
				cpu.SetPC(n16)
			},
		}

	case opcode&0b11100111 == 0b11000000:
		// RET cc https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RET_cc
		// Return from subroutine if condition cc is met.
		index := (opcode & 0b00011000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RET %s", Conds[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				// internal condition checking without fetching an immediate value takes an extra cycle
				cpu.IdleM()
				if Conds[index].Met(cpu) {
					cpu.SetPC(cpu.Pop())
					// PC reload / internal branching takes an extra cycle
					cpu.IdleM()
				}
			},
		}

	case opcode&0b11100111 == 0b11000010:
		// JP cc, n16
		index := (opcode & 0b00011000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("JP %s, a16", Conds[index].Name),
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				if Conds[index].Met(cpu) {
					cpu.SetPC(n16)
					// PC reload / internal branching takes an extra cycle
					cpu.IdleM()
				}
			},
		}

	case opcode&0b11100111 == 0b11000100:
		// CALL cc, n16
		index := (opcode & 0b00011000) >> 3
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("CALL %s, a16", Conds[index].Name),
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				if Conds[index].Met(cpu) {
					cpu.Push(cpu.PC())
					cpu.SetPC(n16)
				}
			},
		}

	case opcode&0b11000111 == 0b11000111:
		// RST vec https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RST_vec
		// Call address vec. This is a shorter and faster equivalent to CALL for suitable values of vec.
		target := uint16(opcode & 0b00111000)
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RST $%02X", target),
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.Push(cpu.PC())
				cpu.SetPC(target)
			},
		}

	case opcode&0b11001111 == 0b11000001:
		// POP r16 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#POP_r16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("POP %s", R16Stacks[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				R16Stacks[index].Set(cpu, cpu.Pop())
			},
		}

	case opcode&0b11001111 == 0b11000101:
		// PUSH r16
		index := (opcode & 0b00110000) >> 4
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("PUSH %s", R16Stacks[index].Name),
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.Push(R16Stacks[index].Get(cpu))
			},
		}

	case opcode == 0b11001011:
		// Prefix CB
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "PREFIX",
			Length:   1,
			Execute: func(cpu *CPU) {
				// The next byte specifies the actual instruction to execute, which is looked up in a separate table of 256 instructions for CB-prefixed opcodes.
				prefixedOpcode := cpu.Fetch()
				prefixedInstruction := prefixed[prefixedOpcode]
				prefixedInstruction.Execute(cpu)
			},
		}

	case opcode == 0b11100000:
		// LDH [n8], A
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LDH [n8], A",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				address := uint16(0xFF00) | uint16(n8)
				cpu.Write8(address, cpu.A())
			},
		}

	case opcode == 0b11100010:
		// LDH [C], A https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#LDH__C_,A
		// Copy the value in register A into the byte at address $FF00+C.
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LDH [C], A",
			Length:   1,
			Execute: func(cpu *CPU) {
				address := uint16(0xFF00) | uint16(cpu.C())
				cpu.Write8(address, cpu.A())
			},
		}

	case opcode == 0b11101010:
		// LD [n16], A
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LD [n16], A",
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				cpu.Write8(n16, cpu.A())
			},
		}

	case opcode == 0b11110000:
		// LDH A, [n8]
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LDH A, [n8]",
			Length:   2,
			Execute: func(cpu *CPU) {
				n8 := cpu.Fetch()
				address := uint16(0xFF00) | uint16(n8)
				value := cpu.Read8(address)
				cpu.SetA(value)
			},
		}

	case opcode == 0b11110010:
		// LDH A, [C]
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LDH A, [C]",
			Length:   1,
			Execute: func(cpu *CPU) {
				address := uint16(0xFF00) | uint16(cpu.C())
				value := cpu.Read8(address)
				cpu.SetA(value)
			},
		}

	case opcode == 0b11111010:
		// LD A, [n16]
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LD A, [n16]",
			Length:   3,
			Execute: func(cpu *CPU) {
				n16 := cpu.Fetch16()
				value := cpu.Read8(n16)
				cpu.SetA(value)
			},
		}

	case opcode == 0b11101000:
		// ADD SP, n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "ADD SP, n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				// M2
				n8 := cpu.Fetch()
				// M3
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag((cpu.SP()&0xF)+(uint16(n8)&0xF) > 0xF)
				cpu.SetCFlag((cpu.SP()&0xFF)+(uint16(n8)&0xFF) > 0xFF)
				cpu.IdleM()
				// M4
				result := uint16(int32(cpu.SP()) + int32(int8(n8)))
				cpu.SetSP(result)
				cpu.IdleM()
			},
		}

	case opcode == 0b11111000:
		// LD HL, SP+n8
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LD HL, SP+n8",
			Length:   2,
			Execute: func(cpu *CPU) {
				// M2
				n8 := cpu.Fetch()
				// M3
				cpu.SetZFlag(false)
				cpu.SetNFlag(false)
				cpu.SetHFlag((cpu.SP()&0xF)+(uint16(n8)&0xF) > 0xF)
				cpu.SetCFlag((cpu.SP()&0xFF)+(uint16(n8)&0xFF) > 0xFF)
				cpu.IdleM()
				result := uint16(int32(cpu.SP()) + int32(int8(n8)))
				cpu.SetHL(result)
			},
		}

	case opcode == 0b11111001:
		// LD SP, HL
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "LD SP, HL",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.SetSP(cpu.HL())
				cpu.IdleM()
			},
		}

	case opcode == 0b11110011:
		// DI
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "DI",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.IC.DisableIME()
			},
		}

	case opcode == 0b11111011:
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "EI",
			Length:   1,
			Execute: func(cpu *CPU) {
				cpu.IC.RequestIME()
			},
		}

	case slices.Contains([]uint8{0xD3, 0xDB, 0xDD, 0xE3, 0xE4, 0xEB, 0xEC, 0xED, 0xF4, 0xFC, 0xFD}, opcode):
		// invalid opcodes that do nothing
		return Instruction{
			Opcode:   opcode,
			Mnemonic: "INVALID",
			Length:   1,
			Execute: func(cpu *CPU) {
				panic(fmt.Sprintf("invalid opcode: 0x%02X", opcode))
			},
		}
	}
	panic(fmt.Sprintf("unimplemented opcode: 0x%02X", opcode))
}

func NewPrefixedInstructions() []Instruction {
	instructions := make([]Instruction, 256)
	for opcode := 0; opcode <= 0xFF; opcode++ {
		instructions[opcode] = NewPrefixedInstruction(uint8(opcode))
	}
	return instructions
}

func NewPrefixedInstruction(opcode uint8) Instruction {
	switch {
	case opcode&0b11111000 == 0b00000000:
		// RLC r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RLC %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.RotateLeft(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00001000:
		// RRC r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RRC %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.RotateRight(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00010000:
		// RL r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RL %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.RotateLeftThroughCarry(R8s[index].Get(cpu), cpu.CFlag())
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00011000:
		// RR r8
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RR %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.RotateRightThroughCarry(R8s[index].Get(cpu), cpu.CFlag())
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00100000:
		// SLA r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SLA_r8
		// Shift Left Arithmetically register r8.
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SLA %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.ShiftLeft(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00101000:
		// SRA r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SRA_r8
		// Shift Right Arithmetically register r8.
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SRA %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.ShiftRightArithmetic(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11111000 == 0b00110000:
		// SWAP r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SWAP_r8
		// Swap the upper 4 bits in register r8 and the lower 4 ones.
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SWAP %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result := bits.SwapNibbles(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(false)
			},
		}

	case opcode&0b11111000 == 0b00111000:
		// SRL r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SRL_r8
		// Shift Right Logically register r8.
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SRL %s", R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				result, carry := bits.ShiftRightLogical(R8s[index].Get(cpu))
				R8s[index].Set(cpu, result)
				cpu.SetZFlag(result == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(false)
				cpu.SetCFlag(carry)
			},
		}

	case opcode&0b11000000 == 0b01000000:
		// BIT u3, r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#BIT_u3,r8
		// Test bit u3 in register r8, set the zero flag if bit not set.
		bit := (opcode & 0b00111000) >> 3
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("BIT %d, %s", bit, R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				value := R8s[index].Get(cpu)
				cpu.SetZFlag((value & (1 << bit)) == 0)
				cpu.SetNFlag(false)
				cpu.SetHFlag(true)
			},
		}

	case opcode&0b11000000 == 0b10000000:
		// RES u3, r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#RES_u3,r8
		// Set bit u3 in register r8 to 0. Bit 0 is the rightmost one, bit 7 the leftmost one.
		bit := (opcode & 0b00111000) >> 3
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("RES %d, %s", bit, R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				value := R8s[index].Get(cpu)
				value &^= (1 << bit)
				R8s[index].Set(cpu, value)
			},
		}

	case opcode&0b11000000 == 0b11000000:
		// SET u3, r8 https://rgbds.gbdev.io/docs/v1.0.1/gbz80.7#SET_u3,r8
		// Set bit u3 in register r8 to 1. Bit 0 is the rightmost one, bit 7 the leftmost one.
		bit := (opcode & 0b00111000) >> 3
		index := opcode & 0b00000111
		return Instruction{
			Opcode:   opcode,
			Mnemonic: fmt.Sprintf("SET %d, %s", bit, R8s[index].Name),
			Length:   2,
			Execute: func(cpu *CPU) {
				value := R8s[index].Get(cpu)
				value |= (1 << bit)
				R8s[index].Set(cpu, value)
			},
		}
	}
	panic(fmt.Sprintf("unimplemented prefixed opcode: 0x%02X", opcode))
}
