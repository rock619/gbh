package bits

func HalfCarryAdd8(a, b uint8, carryIn bool) bool {
	if carryIn {
		return (a&0xF)+(b&0xF)+1 > 0xF
	}
	return (a&0xF)+(b&0xF) > 0xF
}

func HalfBorrowSub8(a, b uint8, borrowIn bool) bool {
	if borrowIn {
		return (a & 0xF) < (b&0xF)+1
	}
	return (a & 0xF) < (b & 0xF)
}

func HalfCarryAdd16(a, b uint16, carryIn bool) bool {
	if carryIn {
		return (a&0xFFF)+(b&0xFFF)+1 > 0xFFF
	}
	return (a&0xFFF)+(b&0xFFF) > 0xFFF
}

func HalfBorrowSub16(a, b uint16, borrowIn bool) bool {
	if borrowIn {
		return (a & 0xFFF) < (b&0xFFF)+1
	}
	return (a & 0xFFF) < (b & 0xFFF)
}

func Add8(a, b uint8, carryIn bool) (result uint8, halfCarry, carryOut bool) {
	sum := uint16(a) + uint16(b)
	if carryIn {
		sum++
	}
	return uint8(sum), HalfCarryAdd8(a, b, carryIn), sum > 0xFF
}

func Sub8(a, b uint8, borrowIn bool) (result uint8, halfBorrow, borrowOut bool) {
	diff := int16(a) - int16(b)
	if borrowIn {
		diff--
	}
	return uint8(diff), HalfBorrowSub8(a, b, borrowIn), diff < 0
}

func Add16(a, b uint16, carryIn bool) (result uint16, halfCarry, carry bool) {
	sum := uint32(a) + uint32(b)
	if carryIn {
		sum++
	}
	return uint16(sum), HalfCarryAdd16(a, b, carryIn), sum > 0xFFFF
}

func Sub16(a, b uint16, borrowIn bool) (result uint16, halfBorrow, borrowOut bool) {
	diff := int32(a) - int32(b)
	if borrowIn {
		diff--
	}
	return uint16(diff), HalfBorrowSub16(a, b, borrowIn), diff < 0
}

func RotateLeft(value uint8) (result uint8, carry bool) {
	carry = (value & 0x80) != 0
	result = (value << 1) | (value >> 7)
	return result, carry
}

func RotateRight(value uint8) (result uint8, carry bool) {
	carry = (value & 0x01) != 0
	result = (value >> 1) | (value << 7)
	return result, carry
}

func RotateLeftThroughCarry(value uint8, carryIn bool) (result uint8, carryOut bool) {
	carryOut = (value & 0x80) != 0
	result = value << 1
	if carryIn {
		result |= 0x01
	}
	return result, carryOut
}

func RotateRightThroughCarry(value uint8, carryIn bool) (result uint8, carryOut bool) {
	carryOut = (value & 0x01) != 0
	result = value >> 1
	if carryIn {
		result |= 0x80
	}
	return result, carryOut
}

func ShiftLeft(value uint8) (result uint8, carry bool) {
	carry = (value & 0x80) != 0
	result = value << 1
	return result, carry
}

func ShiftRightLogical(value uint8) (result uint8, carry bool) {
	carry = (value & 0x01) != 0
	result = value >> 1
	return result, carry
}

func ShiftRightArithmetic(value uint8) (result uint8, carry bool) {
	carry = (value & 0x01) != 0
	result = (value >> 1) | (value & 0x80)
	return result, carry
}

func SwapNibbles(value uint8) uint8 {
	return (value << 4) | (value >> 4)
}

func Bits(op uint8, shift, width uint8) uint8 {
	mask := uint8((1 << width) - 1)
	return (op >> shift) & mask
}

func BoolToBit(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func BitToBool(bit uint8) bool {
	return bit != 0
}

func High(value uint16) uint8 {
	return uint8(value >> 8)
}

func Low(value uint16) uint8 {
	return uint8(value & 0xFF)
}

func Combine(high, low uint8) uint16 {
	return (uint16(high) << 8) | uint16(low)
}
