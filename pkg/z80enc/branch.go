package z80enc

import "fmt"

// EncodeAddr16 encodes a fixed opcode followed by a little-endian
// 16-bit address: JP nn (opcode 0xC3), CALL nn (opcode 0xCD).
func EncodeAddr16(opcode uint8, addr uint16) []byte {
	return []byte{opcode, uint8(addr), uint8(addr >> 8)}
}

// EncodeCondAddr16 encodes "base | (condCode<<3), addr-lo, addr-hi" --
// JP cc,nn's own shape (base 0xC2) and CALL cc,nn's own shape (base
// 0xC4), using the full 8-condition set (Condition, not JRCondition).
func EncodeCondAddr16(base uint8, cond string, addr uint16) ([]byte, error) {
	cc, err := Condition(cond)
	if err != nil {
		return nil, err
	}
	return []byte{base | (cc << 3), uint8(addr), uint8(addr >> 8)}, nil
}

// EncodeRelBranch encodes a fixed opcode followed by a signed 8-bit
// displacement: JR d (opcode 0x18), DJNZ d (opcode 0x10).
func EncodeRelBranch(opcode uint8, displacement int8) []byte {
	return []byte{opcode, uint8(displacement)}
}

// EncodeCondRelBranch encodes "base | (condCode<<3), displacement" --
// JR cc,d's own shape (base 0x20), using JR's own narrower 4-condition
// set (JRCondition, not Condition -- JR has no PO/PE/P/M forms).
func EncodeCondRelBranch(base uint8, cond string, displacement int8) ([]byte, error) {
	cc, err := JRCondition(cond)
	if err != nil {
		return nil, err
	}
	return []byte{base | (cc << 3), uint8(displacement)}, nil
}

// EncodeJPIndirect encodes JP (HL) (0xE9), JP (IX) (DD E9), or JP (IY)
// (FD E9).
func EncodeJPIndirect(reg string) ([]byte, error) {
	switch reg {
	case "HL":
		return []byte{0xE9}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(reg)
		return []byte{prefix, 0xE9}, nil
	default:
		return nil, fmt.Errorf("z80enc: JP only supports (HL), (IX), (IY) as indirect targets, got %q", reg)
	}
}

// EncodeCondOnly encodes "base | (condCode<<3)" with no further
// operand -- RET cc's own shape (base 0xC0).
func EncodeCondOnly(base uint8, cond string) ([]byte, error) {
	cc, err := Condition(cond)
	if err != nil {
		return nil, err
	}
	return []byte{base | (cc << 3)}, nil
}

// EncodeRST encodes RST n: 0xC7 | n, where n must be one of
// 0x00,0x08,0x10,0x18,0x20,0x28,0x30,0x38 -- the eight fixed restart
// vectors; any other value isn't encodable (the low 3 bits of the
// opcode select the vector, so n must be a multiple of 8 in range).
func EncodeRST(n int) ([]byte, error) {
	if n < 0 || n > 0x38 || (n&0x07) != 0 {
		return nil, fmt.Errorf("z80enc: RST target must be one of 00h,08h,10h,18h,20h,28h,30h,38h")
	}
	return []byte{0xC7 | uint8(n)}, nil
}
