package z80enc

import "testing"

func TestEncodeAddr16(t *testing.T) {
	assertBytes(t, EncodeAddr16(0xC3, 0x9000), nil, 0xC3, 0x00, 0x90)
	assertBytes(t, EncodeAddr16(0xCD, 0x9000), nil, 0xCD, 0x00, 0x90)
}

func TestEncodeCondAddr16(t *testing.T) {
	b, err := EncodeCondAddr16(0xC2, "Z", 0x9000)
	assertBytes(t, b, err, 0xCA, 0x00, 0x90)

	b, err = EncodeCondAddr16(0xC4, "PE", 0x9000)
	assertBytes(t, b, err, 0xEC, 0x00, 0x90)
}

func TestEncodeRelBranch(t *testing.T) {
	assertBytes(t, EncodeRelBranch(0x18, -5), nil, 0x18, 0xFB)
	assertBytes(t, EncodeRelBranch(0x10, 10), nil, 0x10, 0x0A)
}

func TestEncodeCondRelBranch(t *testing.T) {
	b, err := EncodeCondRelBranch(0x20, "NZ", -5)
	assertBytes(t, b, err, 0x20, 0xFB)

	if _, err := EncodeCondRelBranch(0x20, "PE", 0); err == nil {
		t.Error("EncodeCondRelBranch(..., \"PE\", ...) expected an error (JR has no PO/PE/P/M forms), got none")
	}
}

func TestEncodeJPIndirect(t *testing.T) {
	b, err := EncodeJPIndirect("HL")
	assertBytes(t, b, err, 0xE9)

	b, err = EncodeJPIndirect("IX")
	assertBytes(t, b, err, 0xDD, 0xE9)

	b, err = EncodeJPIndirect("IY")
	assertBytes(t, b, err, 0xFD, 0xE9)

	if _, err := EncodeJPIndirect("BC"); err == nil {
		t.Error("EncodeJPIndirect(\"BC\") expected an error, got none")
	}
}

func TestEncodeCondOnly(t *testing.T) {
	b, err := EncodeCondOnly(0xC0, "C")
	assertBytes(t, b, err, 0xD8)
}

func TestEncodeRST(t *testing.T) {
	for _, n := range []int{0x00, 0x08, 0x10, 0x18, 0x20, 0x28, 0x30, 0x38} {
		b, err := EncodeRST(n)
		assertBytes(t, b, err, 0xC7|uint8(n))
	}
	if _, err := EncodeRST(0x05); err == nil {
		t.Error("EncodeRST(0x05) expected an error (not a valid restart vector), got none")
	}
}
