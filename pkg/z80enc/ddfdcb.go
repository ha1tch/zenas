// Package z80enc is zenas's pure, dependency-free Z80 opcode-encoding
// knowledge: byte-level arithmetic for specific instruction groups,
// taking plain Go primitives (register names as strings, displacements
// as int8, bit numbers as uint8) rather than any parser- or
// assembler-specific operand type. This is deliberately the part of
// zenas that is shareable across different front ends -- a fork with
// its own lexer/parser (a genuinely different operand representation)
// can still reuse the actual opcode arithmetic here directly, rather
// than re-deriving it from zenas's own release notes each time a fix
// lands, the way the DDCB/FDCB fix had to be manually rewritten instead
// of copied.
//
// The lexer/parser/macro layer itself is NOT extracted here and isn't
// meant to be -- that's a genuinely different design concern per front
// end (e.g. one lexer serving multiple target architectures uniformly),
// not friction this package is trying to eliminate. A front end still
// needs its own thin adapter converting its own resolved-operand
// representation into the primitives these functions take; what this
// package removes is needing to re-derive the actual byte arithmetic
// too.
package z80enc

import "fmt"

// CopyRegister returns the 3-bit register-field code for a DDCB/FDCB
// copy-to-register destination: B/C/D/E/H/L/A only (codes 0,1,2,3,4,5,7
// -- 6 is reserved for the indexed-memory-only form and is not a valid
// destination here). IXH/IXL/IYH/IYL are deliberately rejected, even
// though Reg8 maps them to the same code as plain H/L: real hardware
// always copies into plain H/L in this context, regardless of which
// DD/FD prefix is active for the indexed memory operand itself, so
// accepting the half-index names would be misleading notation for
// identical bytes -- confirmed against real hardware behaviour
// (multiple independent Z80 documentation sources), not assumed.
func CopyRegister(regName string) (uint8, error) {
	switch regName {
	case "B":
		return 0, nil
	case "C":
		return 1, nil
	case "D":
		return 2, nil
	case "E":
		return 3, nil
	case "H":
		return 4, nil
	case "L":
		return 5, nil
	case "A":
		return 7, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid copy-to-register destination %q (want B, C, D, E, H, L, or A)", regName)
	}
}

// EncodeCBIndirect encodes a CB-prefixed rotate/shift on (HL)/(IX+d)/
// (IY+d): RLC/RRC/RL/RR/SLA/SRA/SLL/SRL. reg is "HL", "IX", or "IY";
// displacement is only meaningful (and only consulted) for "IX"/"IY".
// baseOpcode is the group's own base value (e.g. 0x00 for RLC, 0x20
// for SLA, 0x30 for SLL) -- the same constant zenas's own encoder
// table already uses to register each mnemonic.
func EncodeCBIndirect(baseOpcode uint8, reg string, displacement int8) ([]byte, error) {
	switch reg {
	case "HL":
		return []byte{0xCB, baseOpcode | 6}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(reg) // already checked against IX/IY above
		return []byte{prefix, 0xCB, byte(displacement), baseOpcode | 6}, nil
	default:
		return nil, fmt.Errorf("z80enc: expected HL, IX, or IY, got %q", reg)
	}
}

// EncodeCBBitIndirect encodes a CB-prefixed BIT/RES/SET n,(HL)/(IX+d)/
// (IY+d). bit must be 0-7. reg and displacement are as EncodeCBIndirect.
// baseOpcode is 0x40 (BIT), 0x80 (RES), or 0xC0 (SET).
func EncodeCBBitIndirect(baseOpcode, bit uint8, reg string, displacement int8) ([]byte, error) {
	if bit > 7 {
		return nil, fmt.Errorf("z80enc: bit number must be 0-7, got %d", bit)
	}
	switch reg {
	case "HL":
		return []byte{0xCB, baseOpcode | (bit << 3) | 6}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(reg)
		return []byte{prefix, 0xCB, byte(displacement), baseOpcode | (bit << 3) | 6}, nil
	default:
		return nil, fmt.Errorf("z80enc: expected HL, IX, or IY, got %q", reg)
	}
}

// EncodeDDFDCBCopy encodes the undocumented DDCB/FDCB copy-to-register
// rotate/shift forms (e.g. "RLC (IX+d),B"): the operation runs on the
// indexed memory operand exactly as the documented (IX+d)/(IY+d) form,
// and the same result is also stored in destReg. indexReg must be "IX"
// or "IY"; destReg must be one of B/C/D/E/H/L/A (see CopyRegister's own
// comment for why IXH/IXL/IYH/IYL are rejected). Confirmed real, "quite
// commonly used" behaviour (multiple independent Z80 documentation
// sources), not a theoretical opcode -- verified against the exact
// worked example those sources use:
// EncodeDDFDCBCopy(0x00, "IX", 1, "B") == DD CB 01 00.
func EncodeDDFDCBCopy(baseOpcode uint8, indexReg string, displacement int8, destReg string) ([]byte, error) {
	prefix, err := IndexPrefix(indexReg)
	if err != nil {
		return nil, err
	}
	reg, err := CopyRegister(destReg)
	if err != nil {
		return nil, err
	}
	return []byte{prefix, 0xCB, byte(displacement), baseOpcode | reg}, nil
}

// EncodeDDFDCBBitCopy encodes the undocumented DDCB/FDCB copy-to-register
// RES/SET forms (e.g. "SET n,(IX+d),A"). BIT has no equivalent and has
// no function here: confirmed (multiple independent sources) that all 8
// register-field values of a DDCB/FDCB BIT instruction are identical,
// since BIT never writes a result anywhere but the flags -- there is
// nothing to copy. Verified against the exact worked example those
// sources use: EncodeDDFDCBBitCopy(0xC0, 7, "IX", 2, "A") == DD CB 02 FF.
func EncodeDDFDCBBitCopy(baseOpcode, bit uint8, indexReg string, displacement int8, destReg string) ([]byte, error) {
	if bit > 7 {
		return nil, fmt.Errorf("z80enc: bit number must be 0-7, got %d", bit)
	}
	prefix, err := IndexPrefix(indexReg)
	if err != nil {
		return nil, err
	}
	reg, err := CopyRegister(destReg)
	if err != nil {
		return nil, err
	}
	return []byte{prefix, 0xCB, byte(displacement), baseOpcode | (bit << 3) | reg}, nil
}
