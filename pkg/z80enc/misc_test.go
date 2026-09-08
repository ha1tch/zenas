package z80enc

import "testing"

func TestEncodeEX(t *testing.T) {
	b, err := EncodeEX("DE", "HL")
	assertBytes(t, b, err, 0xEB)

	b, err = EncodeEX("AF", "AF'")
	assertBytes(t, b, err, 0x08)

	if _, err := EncodeEX("BC", "DE"); err == nil {
		t.Error("EncodeEX(\"BC\", \"DE\") expected an error, got none")
	}
}

func TestEncodeEXSPHL(t *testing.T) {
	b, err := EncodeEXSPHL("HL")
	assertBytes(t, b, err, 0xE3)

	// IX/IY support: a genuine gap found during the clock-count
	// review, not previously reachable at all.
	b, err = EncodeEXSPHL("IX")
	assertBytes(t, b, err, 0xDD, 0xE3)

	b, err = EncodeEXSPHL("IY")
	assertBytes(t, b, err, 0xFD, 0xE3)

	if _, err := EncodeEXSPHL("BC"); err == nil {
		t.Error("EncodeEXSPHL(\"BC\") expected an error, got none")
	}
}

func TestEncodeIM(t *testing.T) {
	cases := map[int][]byte{
		0: {0xED, 0x46},
		1: {0xED, 0x56},
		2: {0xED, 0x5E},
	}
	for mode, want := range cases {
		b, err := EncodeIM(mode)
		assertBytes(t, b, err, want...)
	}
	if _, err := EncodeIM(3); err == nil {
		t.Error("EncodeIM(3) expected an error, got none")
	}
}

func TestEncodeOUT(t *testing.T) {
	b, err := EncodeOUT("", "A", 0xFE)
	assertBytes(t, b, err, 0xD3, 0xFE)

	b, err = EncodeOUT("C", "B", 0)
	assertBytes(t, b, err, 0xED, 0x41)

	if _, err := EncodeOUT("", "B", 0xFE); err == nil {
		t.Error("EncodeOUT(\"\", \"B\", ...) expected an error (only A is valid for immediate port), got none")
	}
}

func TestEncodeIN(t *testing.T) {
	b, err := EncodeIN("A", "", 0xFE)
	assertBytes(t, b, err, 0xDB, 0xFE)

	b, err = EncodeIN("D", "C", 0)
	assertBytes(t, b, err, 0xED, 0x50)

	if _, err := EncodeIN("B", "", 0xFE); err == nil {
		t.Error("EncodeIN(\"B\", \"\", ...) expected an error (only A is valid for immediate port), got none")
	}
}

// TestEncodeIN_FAlias confirms "IN F,(C)" produces byte-identical
// output to "IN (C)" -- the same ED70 opcode reachable under either
// spelling, not two different encodings.
func TestEncodeIN_FAlias(t *testing.T) {
	viaF, err := EncodeIN("F", "C", 0)
	assertBytes(t, viaF, err, 0xED, 0x70)

	viaBare, err := EncodeINFC("C")
	assertBytes(t, viaBare, err, 0xED, 0x70)

	if string(viaF) != string(viaBare) {
		t.Errorf("IN F,(C) = % X, IN (C) = % X -- want identical bytes", viaF, viaBare)
	}
}

func TestApplyHalfIndexPrefix(t *testing.T) {
	// LD IXH,A: half-index register, no indirect operand.
	b, err := ApplyHalfIndexPrefix([]byte{0x67}, []string{"IXH", "A"}, nil)
	assertBytes(t, b, err, 0xDD, 0x67)

	// Already-prefixed code (indexed-operand encoder added it) is
	// returned unchanged.
	b, err = ApplyHalfIndexPrefix([]byte{0xDD, 0x46, 0x05}, nil, []string{"IX"})
	assertBytes(t, b, err, 0xDD, 0x46, 0x05)

	// No half-index register at all: unchanged.
	b, err = ApplyHalfIndexPrefix([]byte{0x78}, []string{"A", "B"}, nil)
	assertBytes(t, b, err, 0x78)

	// Illegal: mixing IX and IY halves in one instruction.
	if _, err := ApplyHalfIndexPrefix([]byte{0x00}, []string{"IXH", "IYL"}, nil); err == nil {
		t.Error("mixing IXH and IYL expected an error, got none")
	}

	// Illegal: half register plus an indexed memory operand.
	if _, err := ApplyHalfIndexPrefix([]byte{0x00}, []string{"IXH"}, []string{"IX"}); err == nil {
		t.Error("half-index register with an indexed memory operand expected an error, got none")
	}

	// Real bug found during the clock-count review: DD CB / FD CB
	// always addresses (IX+d)/(IY+d) memory -- there is no valid
	// encoding for a CB-prefixed op directly on a half-index register
	// (confirmed against multiple independent sources). Before this
	// fix, code already encoded as if the operand were plain H/L (CB
	// 04, e.g. "RLC H") got a bare DD/FD silently prepended (DD CB
	// 04) -- a truncated, invalid instruction missing the mandatory
	// displacement byte, not rejected the way it must be.
	if _, err := ApplyHalfIndexPrefix([]byte{0xCB, 0x04}, []string{"IXH"}, nil); err == nil {
		t.Error("CB-prefixed op on a half-index register expected an error, got none")
	}
}

func TestEncodeSimple(t *testing.T) {
	assertBytes(t, EncodeSimple(0, 0x00), nil, 0x00)          // NOP
	assertBytes(t, EncodeSimple(0, 0x76), nil, 0x76)          // HALT
	assertBytes(t, EncodeSimple(0xED, 0xB0), nil, 0xED, 0xB0) // LDIR
}

func TestEncodeLDSPHL(t *testing.T) {
	b, err := EncodeLDSPHL("HL")
	assertBytes(t, b, err, 0xF9)

	b, err = EncodeLDSPHL("IX")
	assertBytes(t, b, err, 0xDD, 0xF9)

	if _, err := EncodeLDSPHL("BC"); err == nil {
		t.Error("EncodeLDSPHL(\"BC\") expected an error, got none")
	}
}

func TestEncodeINFC(t *testing.T) {
	b, err := EncodeINFC("C")
	assertBytes(t, b, err, 0xED, 0x70)

	if _, err := EncodeINFC("B"); err == nil {
		t.Error("EncodeINFC(\"B\") expected an error, got none")
	}
}

func TestEncodeOUTC0(t *testing.T) {
	b, err := EncodeOUTC0("C", 0)
	assertBytes(t, b, err, 0xED, 0x71)

	if _, err := EncodeOUTC0("C", 5); err == nil {
		t.Error("EncodeOUTC0(\"C\", 5) expected an error (only a literal 0 is valid), got none")
	}
}
