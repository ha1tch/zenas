package z80enc

import "fmt"

// EncodeFixedOperand encodes a Z80N instruction whose single operand
// is hard-wired to a specific register name (e.g. MIRROR A): prefix,
// opcode if got equals want, otherwise an error. mnemonic names the
// instruction in the error message.
func EncodeFixedOperand(prefix, opcode uint8, mnemonic, got, want string) ([]byte, error) {
	if got != want {
		return nil, fmt.Errorf("z80enc: %s takes only the operand %s", mnemonic, want)
	}
	return []byte{prefix, opcode}, nil
}

// EncodeFixedOperandPair encodes a Z80N instruction whose two operands
// are both hard-wired (e.g. MUL D,E or BSLA DE,B): prefix, opcode if
// both operands match, otherwise an error.
func EncodeFixedOperandPair(prefix, opcode uint8, mnemonic, gotA, wantA, gotB, wantB string) ([]byte, error) {
	if gotA != wantA || gotB != wantB {
		return nil, fmt.Errorf("z80enc: %s takes only the operands %s,%s", mnemonic, wantA, wantB)
	}
	return []byte{prefix, opcode}, nil
}

// EncodeZ80NJPc encodes the Z80N JP (C) (ED 98). reg must be "C" --
// any other indirect register belongs to the base JP (HL)/(IX)/(IY)
// encoder instead, not this one.
func EncodeZ80NJPc(reg string) ([]byte, error) {
	if reg != "C" {
		return nil, fmt.Errorf("z80enc: expected JP (C)")
	}
	return []byte{0xED, 0x98}, nil
}

// EncodeZ80NTEST encodes TEST n (ED 27 n).
func EncodeZ80NTEST(n uint8) []byte {
	return []byte{0xED, 0x27, n}
}

// EncodeZ80NADDrrA encodes ADD HL,A / ADD DE,A / ADD BC,A (ED
// 31/32/33). dst must be HL, DE, or BC; src must be A.
func EncodeZ80NADDrrA(dst, src string) ([]byte, error) {
	if src != "A" {
		return nil, fmt.Errorf("z80enc: ADD %s,%s is not a Z80N instruction", dst, src)
	}
	var op uint8
	switch dst {
	case "HL":
		op = 0x31
	case "DE":
		op = 0x32
	case "BC":
		op = 0x33
	default:
		return nil, fmt.Errorf("z80enc: ADD %s,A: only HL, DE, BC are valid", dst)
	}
	return []byte{0xED, op}, nil
}

// EncodeZ80NADDrrnn encodes ADD HL,nn / ADD DE,nn / ADD BC,nn (ED
// 34/35/36). dst must be HL, DE, or BC. The 16-bit immediate is
// little-endian (unlike PUSH nn, which is not).
func EncodeZ80NADDrrnn(dst string, nn uint16) ([]byte, error) {
	var op uint8
	switch dst {
	case "HL":
		op = 0x34
	case "DE":
		op = 0x35
	case "BC":
		op = 0x36
	default:
		return nil, fmt.Errorf("z80enc: ADD %s,nn: only HL, DE, BC are valid", dst)
	}
	return []byte{0xED, op, uint8(nn), uint8(nn >> 8)}, nil
}

// EncodeZ80NPUSHnn encodes PUSH nn (ED 8A hi lo). This is the only
// Z80(N) operand encoded big-endian: the high byte precedes the low
// byte, unlike every other 16-bit immediate in this instruction set.
func EncodeZ80NPUSHnn(nn uint16) []byte {
	return []byte{0xED, 0x8A, uint8(nn >> 8), uint8(nn)}
}

// EncodeZ80NNEXTREGnn encodes NEXTREG n,n (ED 91 reg val).
func EncodeZ80NNEXTREGnn(reg, val uint8) []byte {
	return []byte{0xED, 0x91, reg, val}
}

// EncodeZ80NNEXTREGnA encodes NEXTREG n,A (ED 92 reg); the value comes
// from A, so src must be "A".
func EncodeZ80NNEXTREGnA(reg uint8, src string) ([]byte, error) {
	if src != "A" {
		return nil, fmt.Errorf("z80enc: NEXTREG n,%s: second operand must be an immediate or A", src)
	}
	return []byte{0xED, 0x92, reg}, nil
}
