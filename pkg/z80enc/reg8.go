package z80enc

import "fmt"

// EncodeReg8Shift3 encodes "base | (regCode << 3)" -- INC r and DEC r's
// own shape: the register field occupies bits 3-5, no prefix.
func EncodeReg8Shift3(base uint8, reg string) ([]byte, error) {
	r, err := Reg8(reg)
	if err != nil {
		return nil, err
	}
	return []byte{base | (r << 3)}, nil
}

// EncodeReg8Imm encodes "base | (regCode << 3), immediate" -- LD r,n's
// own shape.
func EncodeReg8Imm(base uint8, reg string, value uint8) ([]byte, error) {
	r, err := Reg8(reg)
	if err != nil {
		return nil, err
	}
	return []byte{base | (r << 3), value}, nil
}

// EncodeReg8 encodes "base | regCode" -- the register field occupies
// bits 0-2, no shift, no prefix. ALU r's own shape (SUB r, AND r,
// XOR r, OR r, CP r, and the two-operand ADD/ADC/SBC A,r forms once
// the caller has already validated the first operand is A).
func EncodeReg8(base uint8, reg string) ([]byte, error) {
	r, err := Reg8(reg)
	if err != nil {
		return nil, err
	}
	return []byte{base | r}, nil
}

// EncodeCBReg8 encodes a CB-prefixed "base | regCode" -- CB r's own
// shape (RLC r, RRC r, ... SRL r on a plain register, not (HL)/(IX+d),
// which is EncodeCBIndirect's job instead).
func EncodeCBReg8(base uint8, reg string) ([]byte, error) {
	r, err := Reg8(reg)
	if err != nil {
		return nil, err
	}
	return []byte{0xCB, base | r}, nil
}

// EncodeCBBitReg8 encodes a CB-prefixed "base | (bit<<3) | regCode" --
// BIT/RES/SET b,r's own shape (on a plain register; (HL)/(IX+d)/(IY+d)
// is EncodeCBBitIndirect's job instead). base is 0x40 (BIT), 0x80
// (RES), or 0xC0 (SET).
func EncodeCBBitReg8(base, bit uint8, reg string) ([]byte, error) {
	if bit > 7 {
		return nil, fmt.Errorf("z80enc: bit number must be 0-7, got %d", bit)
	}
	r, err := Reg8(reg)
	if err != nil {
		return nil, err
	}
	return []byte{0xCB, base | (bit << 3) | r}, nil
}

// EncodeLDrr encodes LD r,r' (0x40 | (dstCode<<3) | srcCode), including
// the ED-prefixed I/R special cases: LD A,I (ED 57), LD A,R (ED 5F),
// LD I,A (ED 47), LD R,A (ED 4F). I and R are otherwise invalid as
// 8-bit LD operands -- they're interrupt-vector and memory-refresh
// registers, not general-purpose ones, and only transfer to/from A.
func EncodeLDrr(dst, src string) ([]byte, error) {
	switch {
	case dst == "A" && src == "I":
		return []byte{0xED, 0x57}, nil
	case dst == "A" && src == "R":
		return []byte{0xED, 0x5F}, nil
	case dst == "I" && src == "A":
		return []byte{0xED, 0x47}, nil
	case dst == "R" && src == "A":
		return []byte{0xED, 0x4F}, nil
	}
	if dst == "I" || dst == "R" || src == "I" || src == "R" {
		return nil, fmt.Errorf("z80enc: registers I and R may only be used as LD A,I / LD A,R / LD I,A / LD R,A")
	}
	d, err := Reg8(dst)
	if err != nil {
		return nil, err
	}
	s, err := Reg8(src)
	if err != nil {
		return nil, err
	}
	return []byte{0x40 | (d << 3) | s}, nil
}

// EncodeImm8 encodes "base, value" -- the ALU-immediate shape (SUB n,
// AND n, XOR n, OR n, CP n, and the two-operand ADD/ADC/SBC A,n forms
// once the caller has already validated the first operand is A).
// Barely needs a function at all -- base is already fixed per
// mnemonic, no register lookup involved -- included for consistency
// with the rest of this package rather than because the arithmetic
// itself is complex.
func EncodeImm8(base, value uint8) []byte {
	return []byte{base, value}
}
