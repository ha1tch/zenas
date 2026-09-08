package z80enc

import "testing"

func TestEncodeLDRegFromIndirect(t *testing.T) {
	cases := []struct {
		reg8, indReg string
		addr         uint16
		disp         int8
		want         []byte
	}{
		{"B", "HL", 0, 0, []byte{0x46}},
		{"A", "BC", 0, 0, []byte{0x0A}},
		{"A", "DE", 0, 0, []byte{0x1A}},
		{"A", "", 0x9000, 0, []byte{0x3A, 0x00, 0x90}},
		{"C", "IX", 0, 5, []byte{0xDD, 0x4E, 0x05}},
	}
	for _, c := range cases {
		b, err := EncodeLDRegFromIndirect(c.reg8, c.indReg, c.addr, c.disp)
		assertBytes(t, b, err, c.want...)
	}
	if _, err := EncodeLDRegFromIndirect("B", "", 0x9000, 0); err == nil {
		t.Error("EncodeLDRegFromIndirect(\"B\", \"\", ...) expected an error (only A,(nn) is valid), got none")
	}
}

func TestEncodeLDIndirectFromReg(t *testing.T) {
	cases := []struct {
		indReg string
		addr   uint16
		reg8   string
		disp   int8
		want   []byte
	}{
		{"HL", 0, "B", 0, []byte{0x70}},
		{"BC", 0, "A", 0, []byte{0x02}},
		{"", 0x9000, "A", 0, []byte{0x32, 0x00, 0x90}},
		{"IY", 0, "L", -3, []byte{0xFD, 0x75, 0xFD}},
	}
	for _, c := range cases {
		b, err := EncodeLDIndirectFromReg(c.indReg, c.addr, c.reg8, c.disp)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeIndirectImm8(t *testing.T) {
	b, err := EncodeIndirectImm8("HL", 0, 0x42)
	assertBytes(t, b, err, 0x36, 0x42)

	b, err = EncodeIndirectImm8("IX", 5, 0x42)
	assertBytes(t, b, err, 0xDD, 0x36, 0x05, 0x42)
}

func TestEncodeIndirectByte(t *testing.T) {
	b, err := EncodeIndirectByte(0x34, "HL", 0) // INC (HL)
	assertBytes(t, b, err, 0x34)

	b, err = EncodeIndirectByte(0x34, "IX", 5) // INC (IX+5)
	assertBytes(t, b, err, 0xDD, 0x34, 0x05)

	b, err = EncodeIndirectByte(0x86, "HL", 0) // ADD A,(HL)
	assertBytes(t, b, err, 0x86)
}
