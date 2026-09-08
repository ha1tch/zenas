package z80enc

import "testing"

func assertBytes(t *testing.T, got []byte, err error, want ...byte) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got % X, want % X (length)", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got % X, want % X", got, want)
		}
	}
}

// TestEncodeDDFDCBCopy_WorkedExamples checks against the exact worked
// examples multiple independent Z80 documentation sources agree on,
// not just internally-consistent opcode arithmetic.
func TestEncodeDDFDCBCopy_WorkedExamples(t *testing.T) {
	b, err := EncodeDDFDCBCopy(0x00, "IX", 1, "B")
	assertBytes(t, b, err, 0xDD, 0xCB, 0x01, 0x00)

	b, err = EncodeDDFDCBBitCopy(0xC0, 7, "IX", 2, "A")
	assertBytes(t, b, err, 0xDD, 0xCB, 0x02, 0xFF)
}

func TestEncodeCBIndirect(t *testing.T) {
	cases := []struct {
		base uint8
		reg  string
		disp int8
		want []byte
	}{
		{0x00, "HL", 0, []byte{0xCB, 0x06}},
		{0x00, "IX", 5, []byte{0xDD, 0xCB, 0x05, 0x06}},
		{0x08, "IY", -3, []byte{0xFD, 0xCB, 0xFD, 0x0E}},
	}
	for _, c := range cases {
		b, err := EncodeCBIndirect(c.base, c.reg, c.disp)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeCBBitIndirect(t *testing.T) {
	cases := []struct {
		base, bit uint8
		reg       string
		disp      int8
		want      []byte
	}{
		{0x40, 7, "HL", 0, []byte{0xCB, 0x7E}},
		{0x40, 7, "IY", 0, []byte{0xFD, 0xCB, 0x00, 0x7E}},
		{0x80, 3, "IX", 10, []byte{0xDD, 0xCB, 0x0A, 0x9E}},
		{0xC0, 0, "IY", -5, []byte{0xFD, 0xCB, 0xFB, 0xC6}},
	}
	for _, c := range cases {
		b, err := EncodeCBBitIndirect(c.base, c.bit, c.reg, c.disp)
		assertBytes(t, b, err, c.want...)
	}
}

// TestRejectsHalfIndexRegisters confirms IXH/IXL/IYH/IYL are rejected
// as copy destinations, even though they'd map to the same register
// code as plain H/L -- real hardware always copies into plain H/L
// here, so accepting the half-index names would be misleading
// notation for identical bytes.
func TestRejectsHalfIndexRegisters(t *testing.T) {
	for _, reg := range []string{"IXH", "IXL", "IYH", "IYL"} {
		if _, err := EncodeDDFDCBCopy(0x00, "IX", 1, reg); err == nil {
			t.Errorf("expected an error using %s as a copy destination, got none", reg)
		}
	}
}

// TestRejectsInvalidIndexRegister confirms a register other than
// HL/IX/IY is rejected, not silently misencoded.
func TestRejectsInvalidIndexRegister(t *testing.T) {
	if _, err := EncodeCBIndirect(0x00, "BC", 0); err == nil {
		t.Error("expected an error for register BC, got none")
	}
	if _, err := EncodeDDFDCBCopy(0x00, "HL", 0, "B"); err == nil {
		t.Error("expected an error using HL as the index register for a DDCB/FDCB copy form, got none")
	}
}

// TestRejectsOutOfRangeBit confirms a bit number outside 0-7 is
// rejected.
func TestRejectsOutOfRangeBit(t *testing.T) {
	if _, err := EncodeCBBitIndirect(0x40, 8, "HL", 0); err == nil {
		t.Error("expected an error for bit number 8, got none")
	}
	if _, err := EncodeDDFDCBBitCopy(0xC0, 8, "IX", 0, "A"); err == nil {
		t.Error("expected an error for bit number 8, got none")
	}
}
