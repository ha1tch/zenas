package z80enc

import "testing"

func TestEncodeRp16Imm(t *testing.T) {
	cases := []struct {
		rp   string
		val  uint16
		want []byte
	}{
		{"BC", 0x1234, []byte{0x01, 0x34, 0x12}},
		{"IX", 0x8000, []byte{0xDD, 0x21, 0x00, 0x80}},
		{"IY", 0x0001, []byte{0xFD, 0x21, 0x01, 0x00}},
	}
	for _, c := range cases {
		b, err := EncodeRp16Imm(0x01, c.rp, c.val)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeRp16Shift4(t *testing.T) {
	cases := []struct {
		base uint8
		rp   string
		want []byte
	}{
		{0x03, "DE", []byte{0x13}},
		{0x0B, "IY", []byte{0xFD, 0x2B}},
	}
	for _, c := range cases {
		b, err := EncodeRp16Shift4(c.base, c.rp)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeADDHLrp(t *testing.T) {
	cases := []struct {
		dst, src string
		want     []byte
	}{
		{"HL", "DE", []byte{0x19}},
		{"IX", "BC", []byte{0xDD, 0x09}},
		{"IX", "IX", []byte{0xDD, 0x29}},
		{"IY", "IY", []byte{0xFD, 0x29}},
	}
	for _, c := range cases {
		b, err := EncodeADDHLrp(c.dst, c.src)
		assertBytes(t, b, err, c.want...)
	}
	if _, err := EncodeADDHLrp("IX", "IY"); err == nil {
		t.Error("EncodeADDHLrp(\"IX\", \"IY\") expected an error (unencodable), got none")
	}
}

func TestEncodeStackOp(t *testing.T) {
	cases := []struct {
		base uint8
		rp   string
		want []byte
	}{
		{0xC5, "AF", []byte{0xF5}},
		{0xC5, "IX", []byte{0xDD, 0xE5}},
		{0xC1, "IY", []byte{0xFD, 0xE1}},
		{0xC1, "BC", []byte{0xC1}},
	}
	for _, c := range cases {
		b, err := EncodeStackOp(c.base, c.rp)
		assertBytes(t, b, err, c.want...)
	}
	if _, err := EncodeStackOp(0xC5, "SP"); err == nil {
		t.Error("EncodeStackOp with SP expected an error (PUSH/POP never take SP), got none")
	}
}

func TestEncodeLDrpAddr(t *testing.T) {
	cases := []struct {
		rp   string
		want []byte
	}{
		{"HL", []byte{0x2A, 0x00, 0x90}},
		{"BC", []byte{0xED, 0x4B, 0x00, 0x90}},
		{"DE", []byte{0xED, 0x5B, 0x00, 0x90}},
		{"SP", []byte{0xED, 0x7B, 0x00, 0x90}},
	}
	for _, c := range cases {
		b, err := EncodeLDrpAddr(c.rp, 0x9000)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeLDAddrRp(t *testing.T) {
	cases := []struct {
		rp   string
		want []byte
	}{
		{"HL", []byte{0x22, 0x00, 0x90}},
		{"SP", []byte{0xED, 0x73, 0x00, 0x90}},
	}
	for _, c := range cases {
		b, err := EncodeLDAddrRp(c.rp, 0x9000)
		assertBytes(t, b, err, c.want...)
	}
}

func TestEncodeED16(t *testing.T) {
	b, err := EncodeED16(0x4A, "DE")
	assertBytes(t, b, err, 0xED, 0x5A)

	b, err = EncodeED16(0x42, "SP")
	assertBytes(t, b, err, 0xED, 0x72)
}
