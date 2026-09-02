package assembler

import "testing"

// TestDDFDCBRotateShift covers the DDCB/FDCB rotate/shift group
// (RLC/RRC/RL/RR/SLA/SRA/SRL on (IX+d)/(IY+d)), reported missing
// entirely by the zenpoint project (SLL/SLS is not included: it was
// never supported for the (HL) form either, a separate, pre-existing
// gap this fix doesn't touch). The opcode byte is identical to the
// (HL) form's own -- only the DD/FD prefix and displacement byte are
// inserted before it -- so each case is checked against that same
// invariant, not just an opaque expected byte string.
func TestDDFDCBRotateShift(t *testing.T) {
	cases := []struct {
		mnemonic string
		hlOpcode byte // the (HL) form's own opcode byte, for cross-check
	}{
		{"RLC", 0x06},
		{"RRC", 0x0E},
		{"RL", 0x16},
		{"RR", 0x1E},
		{"SLA", 0x26},
		{"SRA", 0x2E},
		{"SRL", 0x3E},
	}
	for _, c := range cases {
		t.Run(c.mnemonic+"_IX", func(t *testing.T) {
			got := assembleOne(t, c.mnemonic+" (IX+5)")
			want := []uint8{0xDD, 0xCB, 0x05, c.hlOpcode}
			assertBytesEqual(t, got, want)
		})
		t.Run(c.mnemonic+"_IY_negative", func(t *testing.T) {
			got := assembleOne(t, c.mnemonic+" (IY-3)")
			want := []uint8{0xFD, 0xCB, 0xFD, c.hlOpcode} // -3 as a signed byte
			assertBytesEqual(t, got, want)
		})
	}
}

// TestDDFDCBBitOps covers the DDCB/FDCB BIT/RES/SET group -- the exact
// instructions the zenpoint report named (flag-preserving BIT/SET/RES
// on (IY+d), used at 21 call sites that previously needed an
// LD-preserves-flags workaround because these had no encoding at all).
func TestDDFDCBBitOps(t *testing.T) {
	cases := []struct {
		src  string
		want []uint8
	}{
		{"BIT 7, (IY+0)", []uint8{0xFD, 0xCB, 0x00, 0x7E}},
		{"RES 3, (IX+10)", []uint8{0xDD, 0xCB, 0x0A, 0x9E}},
		{"SET 0, (IY-5)", []uint8{0xFD, 0xCB, 0xFB, 0xC6}},
		{"BIT 0, (IX+127)", []uint8{0xDD, 0xCB, 0x7F, 0x46}}, // max positive displacement
		{"SET 7, (IY-128)", []uint8{0xFD, 0xCB, 0x80, 0xFE}}, // max negative displacement
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := assembleOne(t, c.src)
			assertBytesEqual(t, got, c.want)
		})
	}
}

// TestDDFDCBDoesNotBreakHLForm is the regression guard: the existing
// (HL) rotate/shift and BIT/RES/SET encodings, which the DDCB/FDCB fix
// extended rather than replaced, must produce exactly the same bytes
// as before.
func TestDDFDCBDoesNotBreakHLForm(t *testing.T) {
	cases := []struct {
		src  string
		want []uint8
	}{
		{"RLC (HL)", []uint8{0xCB, 0x06}},
		{"BIT 7, (HL)", []uint8{0xCB, 0x7E}},
		{"RES 3, (HL)", []uint8{0xCB, 0x9E}},
		{"SET 0, (HL)", []uint8{0xCB, 0xC6}},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := assembleOne(t, c.src)
			assertBytesEqual(t, got, c.want)
		})
	}
}

// assembleOne assembles a single instruction line (org 0, then src,
// then a trailing ret so the assembler always has a well-formed
// program) and returns the encoded bytes for src alone (everything
// but the trailing ret's own single byte).
func assembleOne(t *testing.T, src string) []uint8 {
	t.Helper()
	full := "\torg 0\n\t" + src + "\n\tret\n"
	a := New()
	result, err := a.AssembleString(full)
	if err != nil {
		t.Fatalf("AssembleString(%q): %v", src, err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("assembly errors for %q: %v", src, result.Errors)
	}
	if len(result.MachineCode) < 1 {
		t.Fatalf("AssembleString(%q) produced no output", src)
	}
	return result.MachineCode[:len(result.MachineCode)-1] // drop the trailing ret byte (0xC9)
}

func assertBytesEqual(t *testing.T, got, want []uint8) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got % X (%d bytes), want % X (%d bytes)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got % X, want % X (differs at byte %d)", got, want, i)
		}
	}
}
