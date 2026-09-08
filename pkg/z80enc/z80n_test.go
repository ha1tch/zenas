package z80enc

import "testing"

func TestEncodeFixedOperand(t *testing.T) {
	b, err := EncodeFixedOperand(0xED, 0x24, "MIRROR", "A", "A")
	assertBytes(t, b, err, 0xED, 0x24)

	if _, err := EncodeFixedOperand(0xED, 0x24, "MIRROR", "B", "A"); err == nil {
		t.Error("EncodeFixedOperand with mismatched operand expected an error, got none")
	}
}

func TestEncodeFixedOperandPair(t *testing.T) {
	b, err := EncodeFixedOperandPair(0xED, 0x30, "MUL", "D", "D", "E", "E")
	assertBytes(t, b, err, 0xED, 0x30)

	if _, err := EncodeFixedOperandPair(0xED, 0x30, "MUL", "D", "D", "B", "E"); err == nil {
		t.Error("EncodeFixedOperandPair with mismatched second operand expected an error, got none")
	}
}

func TestEncodeZ80NJPc(t *testing.T) {
	b, err := EncodeZ80NJPc("C")
	assertBytes(t, b, err, 0xED, 0x98)

	if _, err := EncodeZ80NJPc("HL"); err == nil {
		t.Error("EncodeZ80NJPc(\"HL\") expected an error (belongs to the base JP indirect encoder), got none")
	}
}

func TestEncodeZ80NTEST(t *testing.T) {
	assertBytes(t, EncodeZ80NTEST(0x42), nil, 0xED, 0x27, 0x42)
}

func TestEncodeZ80NADDrrA(t *testing.T) {
	cases := map[string][]byte{
		"HL": {0xED, 0x31},
		"DE": {0xED, 0x32},
		"BC": {0xED, 0x33},
	}
	for dst, want := range cases {
		b, err := EncodeZ80NADDrrA(dst, "A")
		assertBytes(t, b, err, want...)
	}
	if _, err := EncodeZ80NADDrrA("HL", "B"); err == nil {
		t.Error("EncodeZ80NADDrrA(\"HL\", \"B\") expected an error (src must be A), got none")
	}
	if _, err := EncodeZ80NADDrrA("SP", "A"); err == nil {
		t.Error("EncodeZ80NADDrrA(\"SP\", \"A\") expected an error (dst must be HL/DE/BC), got none")
	}
}

func TestEncodeZ80NADDrrnn(t *testing.T) {
	b, err := EncodeZ80NADDrrnn("HL", 0x1234)
	assertBytes(t, b, err, 0xED, 0x34, 0x34, 0x12)

	b, err = EncodeZ80NADDrrnn("BC", 0x1234)
	assertBytes(t, b, err, 0xED, 0x36, 0x34, 0x12)
}

func TestEncodeZ80NPUSHnn(t *testing.T) {
	// PUSH nn is big-endian -- unique among Z80(N) 16-bit immediates.
	assertBytes(t, EncodeZ80NPUSHnn(0x1234), nil, 0xED, 0x8A, 0x12, 0x34)
}

func TestEncodeZ80NNEXTREGnn(t *testing.T) {
	assertBytes(t, EncodeZ80NNEXTREGnn(5, 10), nil, 0xED, 0x91, 0x05, 0x0A)
}

func TestEncodeZ80NNEXTREGnA(t *testing.T) {
	b, err := EncodeZ80NNEXTREGnA(5, "A")
	assertBytes(t, b, err, 0xED, 0x92, 0x05)

	if _, err := EncodeZ80NNEXTREGnA(5, "B"); err == nil {
		t.Error("EncodeZ80NNEXTREGnA(5, \"B\") expected an error, got none")
	}
}
