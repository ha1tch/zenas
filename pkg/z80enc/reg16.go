package z80enc

import "fmt"

// EncodeRp16Imm encodes "LD rp,nn": base | (rpCode << 4), then the
// 16-bit immediate little-endian. rp is BC/DE/HL/SP/IX/IY; IX/IY get
// their own DD/FD prefix before the opcode (they share HL's own rp
// code, 2 -- the prefix is what distinguishes them, not a different
// code).
func EncodeRp16Imm(base uint8, rp string, value uint16) ([]byte, error) {
	code, err := Reg16(rp)
	if err != nil {
		return nil, err
	}
	opcode := base | (code << 4)
	lo, hi := uint8(value), uint8(value>>8)
	if rp == "IX" || rp == "IY" {
		prefix, _ := IndexPrefix(rp)
		return []byte{prefix, opcode, lo, hi}, nil
	}
	return []byte{opcode, lo, hi}, nil
}

// EncodeRp16Shift4 encodes "base | (rpCode << 4)" with no further
// operand -- INC rp and DEC rp's own shape. rp is BC/DE/HL/SP/IX/IY,
// with the same IX/IY prefix handling as EncodeRp16Imm.
func EncodeRp16Shift4(base uint8, rp string) ([]byte, error) {
	code, err := Reg16(rp)
	if err != nil {
		return nil, err
	}
	opcode := base | (code << 4)
	if rp == "IX" || rp == "IY" {
		prefix, _ := IndexPrefix(rp)
		return []byte{prefix, opcode}, nil
	}
	return []byte{opcode}, nil
}

// EncodeADDHLrp encodes ADD HL,rp / ADD IX,rp / ADD IY,rp (0x09 |
// (rpCode << 4), with a DD/FD prefix for the IX/IY destination forms).
// dst must be HL, IX, or IY. When dst is IX or IY, src may be BC, DE,
// SP, or dst itself (ADD IX,IX / ADD IY,IY, using HL's own rp code, 2
// -- confirmed real hardware behaviour, not a guess: the index
// register only has one code slot in this encoding, shared with HL,
// and using itself as the source is exactly that slot); mixing IX and
// IY in one ADD is rejected, since it isn't encodable at all (only one
// index prefix can be active per instruction).
func EncodeADDHLrp(dst, src string) ([]byte, error) {
	if dst != "HL" && dst != "IX" && dst != "IY" {
		return nil, fmt.Errorf("z80enc: ADD destination must be HL, IX, or IY, got %q", dst)
	}
	if dst == "IX" || dst == "IY" {
		var code uint8
		switch {
		case src == dst:
			code = 2
		case src == "IX" || src == "IY":
			return nil, fmt.Errorf("z80enc: cannot mix IX and IY in ADD")
		default:
			c, err := Reg16(src)
			if err != nil {
				return nil, err
			}
			code = c
		}
		prefix, _ := IndexPrefix(dst)
		return []byte{prefix, 0x09 | (code << 4)}, nil
	}
	code, err := Reg16(src)
	if err != nil {
		return nil, err
	}
	return []byte{0x09 | (code << 4)}, nil
}

// EncodeStackOp encodes PUSH rp / POP rp: base | (rpCode << 4), using
// PUSH/POP's own register-pair encoding (StackReg16: BC/DE/HL/AF, not
// Reg16's SP-instead-of-AF encoding). rp may also be IX or IY, which
// get their own DD/FD prefix and a fixed opcode (0xE5 for PUSH, 0xE1
// for POP) rather than the base|code form -- confirmed real hardware
// behaviour: PUSH IX/POP IX are single fixed 2-byte forms, not a
// parameterised rp slot the way PUSH BC/DE/HL/AF are.
func EncodeStackOp(base uint8, rp string) ([]byte, error) {
	if rp == "IX" || rp == "IY" {
		prefix, _ := IndexPrefix(rp)
		fixedOpcode := uint8(0xE5) // PUSH
		if base == 0xC1 {
			fixedOpcode = 0xE1 // POP
		}
		return []byte{prefix, fixedOpcode}, nil
	}
	code, err := StackReg16(rp)
	if err != nil {
		return nil, err
	}
	return []byte{base | (code << 4)}, nil
}

// EncodeLDrpAddr encodes "LD rp,(nn)": rp is BC, DE, HL, SP, IX, or
// IY. HL uses the short, unprefixed 0x2A form; BC/DE/SP need an ED
// prefix (0xED, then 0x4B/0x5B/0x7B respectively); IX/IY use the same
// 0x2A opcode as HL, but with their own DD/FD prefix instead (DD 2A
// nn nn / FD 2A nn nn) -- confirmed a real, genuine gap found during
// the clock-count review, not a design decision: these two forms had
// no encoding at all before this fix, not even an unreachable case,
// simply missing.
func EncodeLDrpAddr(rp string, addr uint16) ([]byte, error) {
	lo, hi := uint8(addr), uint8(addr>>8)
	if rp == "HL" {
		return []byte{0x2A, lo, hi}, nil
	}
	if rp == "IX" || rp == "IY" {
		prefix, _ := IndexPrefix(rp)
		return []byte{prefix, 0x2A, lo, hi}, nil
	}
	var sub uint8
	switch rp {
	case "BC":
		sub = 0x4B
	case "DE":
		sub = 0x5B
	case "SP":
		sub = 0x7B
	default:
		return nil, fmt.Errorf("z80enc: LD rp,(nn): unsupported register %q", rp)
	}
	return []byte{0xED, sub, lo, hi}, nil
}

// EncodeLDAddrRp encodes "LD (nn),rp": rp is BC, DE, HL, SP, IX, or
// IY. HL uses the short, unprefixed 0x22 form; BC/DE/SP need an ED
// prefix (0xED, then 0x43/0x53/0x73 respectively); IX/IY use the same
// 0x22 opcode as HL, but with their own DD/FD prefix instead. Same
// genuine gap as EncodeLDrpAddr, fixed the same way.
func EncodeLDAddrRp(rp string, addr uint16) ([]byte, error) {
	lo, hi := uint8(addr), uint8(addr>>8)
	if rp == "HL" {
		return []byte{0x22, lo, hi}, nil
	}
	if rp == "IX" || rp == "IY" {
		prefix, _ := IndexPrefix(rp)
		return []byte{prefix, 0x22, lo, hi}, nil
	}
	var sub uint8
	switch rp {
	case "BC":
		sub = 0x43
	case "DE":
		sub = 0x53
	case "SP":
		sub = 0x73
	default:
		return nil, fmt.Errorf("z80enc: LD (nn),rp: unsupported register %q", rp)
	}
	return []byte{0xED, sub, lo, hi}, nil
}

// EncodeED16 encodes an ED-prefixed 16-bit rp-parameterised op: 0xED,
// base | (rpCode << 4). ADC HL,rp (base 0x4A) and SBC HL,rp (base
// 0x42) are the only two real instructions in this shape; rp uses
// Reg16's own encoding (BC/DE/HL/SP -- these two never take IX/IY).
func EncodeED16(base uint8, rp string) ([]byte, error) {
	code, err := Reg16(rp)
	if err != nil {
		return nil, err
	}
	return []byte{0xED, base | (code << 4)}, nil
}
