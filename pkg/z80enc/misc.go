package z80enc

import "fmt"

// EncodeEX encodes EX DE,HL (0xEB) and EX AF,AF' (0x08) -- the only
// two general-purpose EX forms. a and b are the two operand register
// names as written (AF' spelled with the trailing apostrophe).
func EncodeEX(a, b string) ([]byte, error) {
	switch {
	case a == "DE" && b == "HL":
		return []byte{0xEB}, nil
	case a == "AF" && (b == "AF" || b == "AF'"):
		return []byte{0x08}, nil
	default:
		return nil, fmt.Errorf("z80enc: unsupported EX operands: %s,%s", a, b)
	}
}

// EncodeEXSPHL encodes EX (SP),rp: 0xE3 for HL (unprefixed), DD E3
// for IX, FD E3 for IY. IX/IY support was a genuine gap found during
// the clock-count review (confirmed absent, not a design decision)
// -- validating that rp is one of these three is the caller's job.
func EncodeEXSPHL(rp string) ([]byte, error) {
	switch rp {
	case "HL":
		return []byte{0xE3}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(rp)
		return []byte{prefix, 0xE3}, nil
	}
	return nil, fmt.Errorf("z80enc: EX (SP),%s: unsupported register", rp)
}

// EncodeIM encodes IM 0/1/2 (ED-prefixed): 0xED then 0x46/0x56/0x5E.
func EncodeIM(mode int) ([]byte, error) {
	switch mode {
	case 0:
		return []byte{0xED, 0x46}, nil
	case 1:
		return []byte{0xED, 0x56}, nil
	case 2:
		return []byte{0xED, 0x5E}, nil
	default:
		return nil, fmt.Errorf("z80enc: IM mode must be 0, 1, or 2")
	}
}

// EncodeOUT encodes OUT (n),A / OUT (C),r. portReg is "C" for the
// register-addressed port form (0xED, 0x41 | (srcCode<<3)); "" for
// the immediate-port form (0xD3, port), which requires srcReg to be
// "A".
func EncodeOUT(portReg, srcReg string, port uint8) ([]byte, error) {
	if portReg == "C" {
		r, err := Reg8(srcReg)
		if err != nil {
			return nil, err
		}
		return []byte{0xED, 0x41 | (r << 3)}, nil
	}
	if srcReg != "A" {
		return nil, fmt.Errorf("z80enc: OUT (n),A requires A as the source")
	}
	return []byte{0xD3, port}, nil
}

// EncodeIN encodes IN A,(n) / IN r,(C). portReg is "C" for the
// register-addressed port form (0xED, 0x40 | (dstCode<<3)); "" for
// the immediate-port form (0xDB, port), which requires dstReg to be
// "A". dstReg "F" (only meaningful with portReg "C") is the second
// real spelling of the same ED70 opcode EncodeINFC covers under the
// name "IN (C)" -- confirmed the same instruction under both names,
// not two different opcodes, so this is not a duplicate encoding,
// just the same one reachable under either spelling.
func EncodeIN(dstReg, portReg string, port uint8) ([]byte, error) {
	if portReg == "C" {
		if dstReg == "F" {
			return []byte{0xED, 0x70}, nil
		}
		r, err := Reg8(dstReg)
		if err != nil {
			return nil, err
		}
		return []byte{0xED, 0x40 | (r << 3)}, nil
	}
	if dstReg != "A" {
		return nil, fmt.Errorf("z80enc: IN A,(n) requires A as the destination")
	}
	return []byte{0xDB, port}, nil
}

// ApplyHalfIndexPrefix inspects an already-encoded instruction's
// operand register names and prepends the DD (IX) or FD (IY) prefix
// if any operand used an undocumented half-index register (IXH, IXL,
// IYH, IYL) -- these encode as the corresponding H/L instruction
// (Reg8 maps them to the same codes as H/L) plus that prefix.
// reg8Regs is the register name of every 8-bit-register-typed operand
// in the instruction (their real names, e.g. "IXH", not resolved to
// H); indirectRegs is the register name of every indirect-memory
// operand (e.g. "IX" for an (IX+d) operand).
//
// The prefix governs the entire instruction, which constrains what is
// legal:
//   - All index halves in one instruction must be the same index
//     register; mixing an IX half and an IY half (LD IXH,IYL) is
//     impossible -- it would need two prefixes.
//   - A half register cannot coexist with an indexed memory operand
//     (IX+d)/(IY+d) in the same instruction, since the prefix can
//     only mean one thing.
//
// Both illegal cases are rejected rather than silently producing a
// different instruction. If code already starts with 0xDD/0xFD (the
// indexed-operand encoder already added its own prefix), it's
// returned unchanged.
func ApplyHalfIndexPrefix(code []byte, reg8Regs, indirectRegs []string) ([]byte, error) {
	var prefix uint8
	var sawHalf, sawIndexedMem bool
	for _, r := range indirectRegs {
		if r == "IX" || r == "IY" {
			sawIndexedMem = true
		}
	}
	for _, r := range reg8Regs {
		var p uint8
		switch r {
		case "IXH", "IXL":
			p = 0xDD
		case "IYH", "IYL":
			p = 0xFD
		default:
			continue
		}
		if prefix != 0 && prefix != p {
			return nil, fmt.Errorf("z80enc: cannot mix IX and IY half registers in one instruction")
		}
		prefix = p
		sawHalf = true
	}
	if prefix == 0 {
		return code, nil
	}
	// DD CB / FD CB always addresses (IX+d)/(IY+d) memory -- confirmed
	// (multiple independent sources): there is no valid Z80 encoding
	// for a CB-prefixed rotate/shift/bit operation applied directly to
	// a half-index register. Without this check, code (already
	// encoded as if the operand were plain H/L, since Reg8 maps the
	// half registers to the same codes) would get a prefix silently
	// prepended -- producing a truncated, invalid instruction missing
	// the mandatory displacement byte DD CB/FD CB always requires,
	// rather than being rejected.
	if sawHalf && len(code) > 0 && code[0] == 0xCB {
		return nil, fmt.Errorf("z80enc: CB-prefixed rotate/shift/bit operations cannot be applied directly to a half-index register (IXH/IXL/IYH/IYL) -- DD CB/FD CB always addresses (IX+d)/(IY+d) memory, never a register directly")
	}
	if sawHalf && sawIndexedMem {
		return nil, fmt.Errorf("z80enc: cannot combine an index half register with an (IX+d)/(IY+d) operand")
	}
	if len(code) > 0 && (code[0] == 0xDD || code[0] == 0xFD) {
		return code, nil
	}
	return append([]byte{prefix}, code...), nil
}

// EncodeSimple encodes a fixed, no-operand instruction: prefix+opcode
// if a prefix is set, otherwise just opcode. This is the shape behind
// every zero-operand instruction with a fixed encoding (NOP, HALT,
// RET, DI, EI, LDIR, and similar) -- prefix 0 means "no prefix byte".
func EncodeSimple(prefix, opcode uint8) []byte {
	if prefix != 0 {
		return []byte{prefix, opcode}
	}
	return []byte{opcode}
}

// EncodeLDSPHL encodes LD SP,HL (0xF9) / LD SP,IX / LD SP,IY (DD/FD
// F9). src must be HL, IX, or IY.
func EncodeLDSPHL(src string) ([]byte, error) {
	switch src {
	case "HL":
		return []byte{0xF9}, nil
	case "IX", "IY":
		prefix, _ := IndexPrefix(src)
		return []byte{prefix, 0xF9}, nil
	default:
		return nil, fmt.Errorf("z80enc: unsupported LD SP,%s", src)
	}
}

// EncodeINFC encodes the undocumented "IN (C)" (also widely known as
// "IN F,(C)"): ED 70. Reads port C, sets flags exactly like the
// documented IN r,(C) forms, discards the byte read. portReg must be
// "C".
func EncodeINFC(portReg string) ([]byte, error) {
	if portReg != "C" {
		return nil, fmt.Errorf("z80enc: expected (C)")
	}
	return []byte{0xED, 0x70}, nil
}

// EncodeOUTC0 encodes the undocumented "OUT (C),0": ED 71. Outputs a
// literal zero byte to the port in C, no source register. portReg
// must be "C" and value must be 0.
func EncodeOUTC0(portReg string, value int) ([]byte, error) {
	if portReg != "C" {
		return nil, fmt.Errorf("z80enc: expected (C)")
	}
	if value != 0 {
		return nil, fmt.Errorf("z80enc: OUT (C),0 requires a literal 0")
	}
	return []byte{0xED, 0x71}, nil
}
