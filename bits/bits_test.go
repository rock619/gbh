package bits

import "testing"

func TestHalfCarryAdd8(t *testing.T) {
	tests := []struct {
		name    string
		a, b    uint8
		carryIn bool
		want    bool
	}{
		{name: "without half carry", a: 0x0E, b: 0x00, carryIn: true, want: false},
		{name: "with carry in", a: 0x0F, b: 0x00, carryIn: true, want: true},
		{name: "with low nibble overflow", a: 0x0F, b: 0x01, carryIn: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HalfCarryAdd8(tt.a, tt.b, tt.carryIn); got != tt.want {
				t.Fatalf("HalfCarryAdd8(%#02x, %#02x, %t) = %t, want %t", tt.a, tt.b, tt.carryIn, got, tt.want)
			}
		})
	}
}

func TestHalfBorrowSub8(t *testing.T) {
	tests := []struct {
		name     string
		a, b     uint8
		borrowIn bool
		want     bool
	}{
		{name: "without half borrow", a: 0x01, b: 0x00, borrowIn: true, want: false},
		{name: "with borrow in", a: 0x00, b: 0x0F, borrowIn: true, want: true},
		{name: "with low nibble borrow", a: 0x10, b: 0x01, borrowIn: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HalfBorrowSub8(tt.a, tt.b, tt.borrowIn); got != tt.want {
				t.Fatalf("HalfBorrowSub8(%#02x, %#02x, %t) = %t, want %t", tt.a, tt.b, tt.borrowIn, got, tt.want)
			}
		})
	}
}

func TestHalfCarryAdd16(t *testing.T) {
	tests := []struct {
		name    string
		a, b    uint16
		carryIn bool
		want    bool
	}{
		{name: "without half carry", a: 0x0FFE, b: 0x0000, carryIn: true, want: false},
		{name: "with carry in", a: 0x0FFF, b: 0x0000, carryIn: true, want: true},
		{name: "with low twelve bits overflow", a: 0x0FFF, b: 0x0001, carryIn: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HalfCarryAdd16(tt.a, tt.b, tt.carryIn); got != tt.want {
				t.Fatalf("HalfCarryAdd16(%#04x, %#04x, %t) = %t, want %t", tt.a, tt.b, tt.carryIn, got, tt.want)
			}
		})
	}
}

func TestHalfBorrowSub16(t *testing.T) {
	tests := []struct {
		name     string
		a, b     uint16
		borrowIn bool
		want     bool
	}{
		{name: "without half borrow", a: 0x0001, b: 0x0000, borrowIn: true, want: false},
		{name: "with borrow in", a: 0x0000, b: 0x0FFF, borrowIn: true, want: true},
		{name: "with low twelve bits borrow", a: 0x1000, b: 0x0001, borrowIn: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HalfBorrowSub16(tt.a, tt.b, tt.borrowIn); got != tt.want {
				t.Fatalf("HalfBorrowSub16(%#04x, %#04x, %t) = %t, want %t", tt.a, tt.b, tt.borrowIn, got, tt.want)
			}
		})
	}
}

func TestAdd8(t *testing.T) {
	tests := []struct {
		name      string
		a, b      uint8
		carryIn   bool
		result    uint8
		halfCarry bool
		carryOut  bool
	}{
		{name: "without carry", a: 0x12, b: 0x23, carryIn: false, result: 0x35, halfCarry: false, carryOut: false},
		{name: "half carry", a: 0x0F, b: 0x01, carryIn: false, result: 0x10, halfCarry: true, carryOut: false},
		{name: "carry out", a: 0xFF, b: 0x01, carryIn: false, result: 0x00, halfCarry: true, carryOut: true},
		{name: "carry in", a: 0x0E, b: 0x01, carryIn: true, result: 0x10, halfCarry: true, carryOut: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, halfCarry, carryOut := Add8(tt.a, tt.b, tt.carryIn)
			if result != tt.result || halfCarry != tt.halfCarry || carryOut != tt.carryOut {
				t.Fatalf("Add8(%#02x, %#02x, %t) = (%#02x, %t, %t), want (%#02x, %t, %t)",
					tt.a, tt.b, tt.carryIn, result, halfCarry, carryOut, tt.result, tt.halfCarry, tt.carryOut)
			}
		})
	}
}

func TestSub8(t *testing.T) {
	tests := []struct {
		name       string
		a, b       uint8
		borrowIn   bool
		result     uint8
		halfBorrow bool
		borrowOut  bool
	}{
		{name: "without borrow", a: 0x34, b: 0x12, borrowIn: false, result: 0x22, halfBorrow: false, borrowOut: false},
		{name: "half borrow", a: 0x10, b: 0x01, borrowIn: false, result: 0x0F, halfBorrow: true, borrowOut: false},
		{name: "borrow out", a: 0x00, b: 0x01, borrowIn: false, result: 0xFF, halfBorrow: true, borrowOut: true},
		{name: "borrow in", a: 0x10, b: 0x0F, borrowIn: true, result: 0x00, halfBorrow: true, borrowOut: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, halfBorrow, borrowOut := Sub8(tt.a, tt.b, tt.borrowIn)
			if result != tt.result || halfBorrow != tt.halfBorrow || borrowOut != tt.borrowOut {
				t.Fatalf("Sub8(%#02x, %#02x, %t) = (%#02x, %t, %t), want (%#02x, %t, %t)",
					tt.a, tt.b, tt.borrowIn, result, halfBorrow, borrowOut, tt.result, tt.halfBorrow, tt.borrowOut)
			}
		})
	}
}

func TestAdd16(t *testing.T) {
	tests := []struct {
		name      string
		a, b      uint16
		carryIn   bool
		result    uint16
		halfCarry bool
		carryOut  bool
	}{
		{name: "without carry", a: 0x1234, b: 0x0101, carryIn: false, result: 0x1335, halfCarry: false, carryOut: false},
		{name: "half carry", a: 0x0FFF, b: 0x0001, carryIn: false, result: 0x1000, halfCarry: true, carryOut: false},
		{name: "carry out", a: 0xFFFF, b: 0x0001, carryIn: false, result: 0x0000, halfCarry: true, carryOut: true},
		{name: "carry in", a: 0x0FFE, b: 0x0001, carryIn: true, result: 0x1000, halfCarry: true, carryOut: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, halfCarry, carryOut := Add16(tt.a, tt.b, tt.carryIn)
			if result != tt.result || halfCarry != tt.halfCarry || carryOut != tt.carryOut {
				t.Fatalf("Add16(%#04x, %#04x, %t) = (%#04x, %t, %t), want (%#04x, %t, %t)",
					tt.a, tt.b, tt.carryIn, result, halfCarry, carryOut, tt.result, tt.halfCarry, tt.carryOut)
			}
		})
	}
}

func TestSub16(t *testing.T) {
	tests := []struct {
		name       string
		a, b       uint16
		borrowIn   bool
		result     uint16
		halfBorrow bool
		borrowOut  bool
	}{
		{name: "without borrow", a: 0x3456, b: 0x1234, borrowIn: false, result: 0x2222, halfBorrow: false, borrowOut: false},
		{name: "half borrow", a: 0x1000, b: 0x0001, borrowIn: false, result: 0x0FFF, halfBorrow: true, borrowOut: false},
		{name: "borrow out", a: 0x0000, b: 0x0001, borrowIn: false, result: 0xFFFF, halfBorrow: true, borrowOut: true},
		{name: "borrow in", a: 0x1000, b: 0x0FFF, borrowIn: true, result: 0x0000, halfBorrow: true, borrowOut: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, halfBorrow, borrowOut := Sub16(tt.a, tt.b, tt.borrowIn)
			if result != tt.result || halfBorrow != tt.halfBorrow || borrowOut != tt.borrowOut {
				t.Fatalf("Sub16(%#04x, %#04x, %t) = (%#04x, %t, %t), want (%#04x, %t, %t)",
					tt.a, tt.b, tt.borrowIn, result, halfBorrow, borrowOut, tt.result, tt.halfBorrow, tt.borrowOut)
			}
		})
	}
}

func TestRotateLeft(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		result uint8
		carry  bool
	}{
		{name: "wraps bit 7", value: 0x81, result: 0x03, carry: true},
		{name: "without carry", value: 0x40, result: 0x80, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := RotateLeft(tt.value)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("RotateLeft(%#02x) = (%#02x, %t), want (%#02x, %t)", tt.value, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestRotateRight(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		result uint8
		carry  bool
	}{
		{name: "wraps bit 0", value: 0x81, result: 0xC0, carry: true},
		{name: "without carry", value: 0x02, result: 0x01, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := RotateRight(tt.value)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("RotateRight(%#02x) = (%#02x, %t), want (%#02x, %t)", tt.value, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestRotateLeftThroughCarry(t *testing.T) {
	tests := []struct {
		name    string
		value   uint8
		carryIn bool
		result  uint8
		carry   bool
	}{
		{name: "uses carry in", value: 0x80, carryIn: true, result: 0x01, carry: true},
		{name: "without carry in", value: 0x40, carryIn: false, result: 0x80, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := RotateLeftThroughCarry(tt.value, tt.carryIn)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("RotateLeftThroughCarry(%#02x, %t) = (%#02x, %t), want (%#02x, %t)",
					tt.value, tt.carryIn, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestRotateRightThroughCarry(t *testing.T) {
	tests := []struct {
		name    string
		value   uint8
		carryIn bool
		result  uint8
		carry   bool
	}{
		{name: "uses carry in", value: 0x01, carryIn: true, result: 0x80, carry: true},
		{name: "without carry in", value: 0x02, carryIn: false, result: 0x01, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := RotateRightThroughCarry(tt.value, tt.carryIn)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("RotateRightThroughCarry(%#02x, %t) = (%#02x, %t), want (%#02x, %t)",
					tt.value, tt.carryIn, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestShiftLeft(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		result uint8
		carry  bool
	}{
		{name: "sets carry", value: 0x80, result: 0x00, carry: true},
		{name: "without carry", value: 0x40, result: 0x80, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := ShiftLeft(tt.value)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("ShiftLeft(%#02x) = (%#02x, %t), want (%#02x, %t)", tt.value, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestShiftRightLogical(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		result uint8
		carry  bool
	}{
		{name: "sets carry", value: 0x81, result: 0x40, carry: true},
		{name: "without carry", value: 0x02, result: 0x01, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := ShiftRightLogical(tt.value)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("ShiftRightLogical(%#02x) = (%#02x, %t), want (%#02x, %t)", tt.value, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestShiftRightArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		result uint8
		carry  bool
	}{
		{name: "keeps sign bit", value: 0x81, result: 0xC0, carry: true},
		{name: "without sign bit or carry", value: 0x02, result: 0x01, carry: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, carry := ShiftRightArithmetic(tt.value)
			if result != tt.result || carry != tt.carry {
				t.Fatalf("ShiftRightArithmetic(%#02x) = (%#02x, %t), want (%#02x, %t)", tt.value, result, carry, tt.result, tt.carry)
			}
		})
	}
}

func TestSwapNibbles(t *testing.T) {
	tests := []struct {
		name  string
		value uint8
		want  uint8
	}{
		{name: "swaps high and low nibbles", value: 0xAB, want: 0xBA},
		{name: "zero", value: 0x00, want: 0x00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SwapNibbles(tt.value); got != tt.want {
				t.Fatalf("SwapNibbles(%#02x) = %#02x, want %#02x", tt.value, got, tt.want)
			}
		})
	}
}

func TestBits(t *testing.T) {
	tests := []struct {
		name         string
		op           uint8
		shift, width uint8
		want         uint8
	}{
		{name: "extracts bit range", op: 0b1101_0110, shift: 2, width: 3, want: 0b101},
		{name: "extracts low bit", op: 0b0000_0001, shift: 0, width: 1, want: 0b1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Bits(tt.op, tt.shift, tt.width); got != tt.want {
				t.Fatalf("Bits(%08b, %d, %d) = %08b, want %08b", tt.op, tt.shift, tt.width, got, tt.want)
			}
		})
	}
}

func TestBoolToBit(t *testing.T) {
	tests := []struct {
		name string
		b    bool
		want uint8
	}{
		{name: "true", b: true, want: 1},
		{name: "false", b: false, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BoolToBit(tt.b); got != tt.want {
				t.Fatalf("BoolToBit(%t) = %d, want %d", tt.b, got, tt.want)
			}
		})
	}
}

func TestBitToBool(t *testing.T) {
	tests := []struct {
		name string
		bit  uint8
		want bool
	}{
		{name: "zero", bit: 0, want: false},
		{name: "one", bit: 1, want: true},
		{name: "non-zero", bit: 2, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BitToBool(tt.bit); got != tt.want {
				t.Fatalf("BitToBool(%d) = %t, want %t", tt.bit, got, tt.want)
			}
		})
	}
}

func TestHigh(t *testing.T) {
	tests := []struct {
		name  string
		value uint16
		want  uint8
	}{
		{name: "high byte", value: 0xABCD, want: 0xAB},
		{name: "zero high byte", value: 0x00CD, want: 0x00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := High(tt.value); got != tt.want {
				t.Fatalf("High(%#04x) = %#02x, want %#02x", tt.value, got, tt.want)
			}
		})
	}
}

func TestLow(t *testing.T) {
	tests := []struct {
		name  string
		value uint16
		want  uint8
	}{
		{name: "low byte", value: 0xABCD, want: 0xCD},
		{name: "zero low byte", value: 0xAB00, want: 0x00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Low(tt.value); got != tt.want {
				t.Fatalf("Low(%#04x) = %#02x, want %#02x", tt.value, got, tt.want)
			}
		})
	}
}

func TestCombine(t *testing.T) {
	tests := []struct {
		name      string
		high, low uint8
		want      uint16
	}{
		{name: "combines high and low bytes", high: 0xAB, low: 0xCD, want: 0xABCD},
		{name: "zero bytes", high: 0x00, low: 0x00, want: 0x0000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Combine(tt.high, tt.low); got != tt.want {
				t.Fatalf("Combine(%#02x, %#02x) = %#04x, want %#04x", tt.high, tt.low, got, tt.want)
			}
		})
	}
}
