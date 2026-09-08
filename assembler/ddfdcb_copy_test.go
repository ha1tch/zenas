package assembler

import "testing"

// TestDDFDCBCopyToRegister covers the undocumented DDCB/FDCB
// copy-to-register forms (e.g. "RLC (IX+d),B"), added on top of the
// plain DDCB/FDCB group. Two cases are checked against the exact
// worked examples multiple independent Z80 documentation sources
// agree on (DDCB0100 = RLC (IX+1),B; DDCB02FF = SET 7,(IX+2),A), not
// just internally-consistent opcode arithmetic.
func TestDDFDCBCopyToRegister(t *testing.T) {
	cases := []struct {
		src  string
		want []uint8
	}{
		{"RLC (IX+1), B", []uint8{0xDD, 0xCB, 0x01, 0x00}},    // worked example from research
		{"SET 7, (IX+2), A", []uint8{0xDD, 0xCB, 0x02, 0xFF}}, // worked example from research
		{"RES 0, (IY-5), H", []uint8{0xFD, 0xCB, 0xFB, 0x84}},
		{"SRL (IX+0), C", []uint8{0xDD, 0xCB, 0x00, 0x39}},
		{"SLL (IY+10), L", []uint8{0xFD, 0xCB, 0x0A, 0x35}},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := assembleOne(t, c.src)
			assertBytesEqual(t, got, c.want)
		})
	}
}

// TestDDFDCBCopyRejectsHalfIndexRegisters confirms IXH/IXL/IYH/IYL are
// rejected as copy-to-register destinations, even though they map to
// the same reg field as plain H/L: real hardware always copies into
// plain H/L here, so accepting the half-index names would be
// misleading notation for identical bytes.
func TestDDFDCBCopyRejectsHalfIndexRegisters(t *testing.T) {
	for _, reg := range []string{"IXH", "IXL", "IYH", "IYL"} {
		t.Run(reg, func(t *testing.T) {
			a := New()
			src := "\torg 0\n\tRLC (IX+1), " + reg + "\n"
			result, err := a.AssembleString(src)
			if err == nil && len(result.Errors) == 0 {
				t.Errorf("expected an error using %s as a DDCB copy destination, got none", reg)
			}
		})
	}
}

// TestDDFDCBBitHasNoCopyForm confirms BIT n,(IX+d),r is rejected: BIT
// never writes a result anywhere but the flags, so all 8 register-field
// values of a DDCB/FDCB BIT instruction are identical -- there is
// nothing to copy, and no three-operand form was registered for it.
func TestDDFDCBBitHasNoCopyForm(t *testing.T) {
	a := New()
	result, err := a.AssembleString("\torg 0\n\tBIT 7, (IX+1), A\n")
	if err == nil && len(result.Errors) == 0 {
		t.Error("expected an error for BIT n,(IX+d),r (no such instruction), got none")
	}
}

// TestSLLAllForms covers the undocumented 8th shift (SLL/SLI/SL1,
// synonyms for the same opcode) across every operand form: register,
// (HL), (IX+d), (IY+d).
func TestSLLAllForms(t *testing.T) {
	cases := []struct {
		src  string
		want []uint8
	}{
		{"SLL B", []uint8{0xCB, 0x30}},
		{"SLI (HL)", []uint8{0xCB, 0x36}},
		{"SL1 (IX+5)", []uint8{0xDD, 0xCB, 0x05, 0x36}},
		{"SLL (IY-3)", []uint8{0xFD, 0xCB, 0xFD, 0x36}},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := assembleOne(t, c.src)
			assertBytesEqual(t, got, c.want)
		})
	}
}

// TestBareINOUT covers the undocumented "IN (C)" (ED70: reads the
// port, sets flags, discards the byte), "IN F,(C)" (the same ED70
// opcode under its second real spelling -- previously rejected by
// this codebase; a documented gap, not a design decision, closed
// here rather than left open), and "OUT (C),0" (ED71: outputs a
// literal zero, no source register).
func TestBareINOUT(t *testing.T) {
	cases := []struct {
		src  string
		want []uint8
	}{
		{"IN (C)", []uint8{0xED, 0x70}},
		{"IN F, (C)", []uint8{0xED, 0x70}},
		{"OUT (C), 0", []uint8{0xED, 0x71}},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := assembleOne(t, c.src)
			assertBytesEqual(t, got, c.want)
		})
	}
}

// TestFRegisterOnlyValidForINFC confirms F is accepted only in
// "IN F,(C)" and rejected everywhere else an 8-bit register could
// appear -- it is not a general-purpose register, and the parser
// change that let "IN F,(C)" through must not have widened anything
// beyond that one instruction.
func TestFRegisterOnlyValidForINFC(t *testing.T) {
	for _, src := range []string{"LD F, A", "LD A, F", "INC F", "AND F"} {
		t.Run(src, func(t *testing.T) {
			a := New()
			result, err := a.AssembleString("\torg 0\n\t" + src + "\n")
			if err == nil && len(result.Errors) == 0 {
				t.Errorf("expected an error for %q (F is not a general register), got none", src)
			}
		})
	}
}

// TestCBOpRejectsHalfIndexRegister is the full-pipeline regression
// test for a real bug found during the clock-count review: DD CB /
// FD CB always addresses (IX+d)/(IY+d) memory, so a CB-prefixed
// rotate/shift/bit operation directly on IXH/IXL/IYH/IYL has no valid
// encoding at all. Before the fix, these silently assembled to
// truncated, invalid bytes (e.g. "RLC IXH" -> DD CB 04, missing the
// mandatory displacement byte) instead of being rejected.
func TestCBOpRejectsHalfIndexRegister(t *testing.T) {
	for _, src := range []string{"RLC IXH", "BIT 0, IXH", "SET 0, IYL", "RES 7, IXL", "SRL IYH"} {
		t.Run(src, func(t *testing.T) {
			a := New()
			result, err := a.AssembleString("\torg 0\n\t" + src + "\n")
			if err == nil && len(result.Errors) == 0 {
				t.Errorf("expected an error for %q (no valid DD/FD CB encoding for a half-index register), got none", src)
			}
		})
	}
}
