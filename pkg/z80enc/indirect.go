package z80enc

import "fmt"

// EncodeIndirectByte encodes a single fixed-purpose opcode byte for an
// (HL)/(IX+d)/(IY+d) operand: HL emits just the opcode; IX/IY emit the
// DD/FD prefix, the same opcode, then the displacement byte. This is
// the general shape behind INC (HL)/(IX+d)/(IY+d), DEC (HL)/(IX+d)/
// (IY+d), and any ALU-style op on an indirect operand once the caller
// has already combined its own base with any register-field bits
// (e.g. base|6 for the ALU family).
func EncodeIndirectByte(opcode uint8, indReg string, displacement int8) ([]byte, error) {
	switch indReg {
	case "HL":
		return []byte{opcode}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(indReg)
		return []byte{prefix, opcode, byte(displacement)}, nil
	default:
		return nil, fmt.Errorf("z80enc: expected HL, IX, or IY, got %q", indReg)
	}
}

// EncodeIndirectImm8 encodes LD (HL),n / LD (IX+d),n / LD (IY+d),n:
// 0x36, then the immediate value for (HL); DD/FD, 0x36, displacement,
// value for the indexed forms.
func EncodeIndirectImm8(indReg string, displacement int8, value uint8) ([]byte, error) {
	switch indReg {
	case "HL":
		return []byte{0x36, value}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(indReg)
		return []byte{prefix, 0x36, byte(displacement), value}, nil
	default:
		return nil, fmt.Errorf("z80enc: expected HL, IX, or IY as destination, got %q", indReg)
	}
}

// EncodeLDRegFromIndirect encodes "LD r,(...)": the general (HL)/
// (IX+d)/(IY+d) form (0x46 | (regCode<<3), with index prefix and
// displacement as usual), plus the three special-cased forms that only
// work with A as the destination: LD A,(BC) (0x0A), LD A,(DE) (0x1A),
// and LD A,(nn) (0x3A, absolute address -- indReg "" selects this
// form, matching the convention the rest of this codebase already
// uses for "no register means it's an address").
func EncodeLDRegFromIndirect(reg8, indReg string, addr uint16, displacement int8) ([]byte, error) {
	switch {
	case indReg == "HL":
		r, err := Reg8(reg8)
		if err != nil {
			return nil, err
		}
		return []byte{0x46 | (r << 3)}, nil
	case indReg == "BC" && reg8 == "A":
		return []byte{0x0A}, nil
	case indReg == "DE" && reg8 == "A":
		return []byte{0x1A}, nil
	case indReg == "":
		if reg8 != "A" {
			return nil, fmt.Errorf("z80enc: only LD A,(nn) supported for absolute addressing")
		}
		return []byte{0x3A, uint8(addr), uint8(addr >> 8)}, nil
	case indReg == "IX" || indReg == "IY":
		r, err := Reg8(reg8)
		if err != nil {
			return nil, err
		}
		prefix, _ := IndexPrefix(indReg)
		return []byte{prefix, 0x46 | (r << 3), byte(displacement)}, nil
	}
	return nil, fmt.Errorf("z80enc: unsupported indirect addressing mode: (%s)", indReg)
}

// EncodeLDIndirectFromReg encodes "LD (...),r": the mirror of
// EncodeLDRegFromIndirect. The general form is 0x70 | regCode; the
// special-cased A-only forms are LD (BC),A (0x02), LD (DE),A (0x12),
// and LD (nn),A (0x32, absolute address, indReg "").
func EncodeLDIndirectFromReg(indReg string, addr uint16, reg8 string, displacement int8) ([]byte, error) {
	switch {
	case indReg == "HL":
		r, err := Reg8(reg8)
		if err != nil {
			return nil, err
		}
		return []byte{0x70 | r}, nil
	case indReg == "BC" && reg8 == "A":
		return []byte{0x02}, nil
	case indReg == "DE" && reg8 == "A":
		return []byte{0x12}, nil
	case indReg == "":
		if reg8 != "A" {
			return nil, fmt.Errorf("z80enc: only LD (nn),A supported for absolute addressing")
		}
		return []byte{0x32, uint8(addr), uint8(addr >> 8)}, nil
	case indReg == "IX" || indReg == "IY":
		r, err := Reg8(reg8)
		if err != nil {
			return nil, err
		}
		prefix, _ := IndexPrefix(indReg)
		return []byte{prefix, 0x70 | r, byte(displacement)}, nil
	}
	return nil, fmt.Errorf("z80enc: unsupported indirect addressing mode: (%s)", indReg)
}
