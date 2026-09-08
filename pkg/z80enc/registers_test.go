package z80enc

import "testing"

func TestReg8(t *testing.T) {
	cases := map[string]uint8{
		"B": 0, "C": 1, "D": 2, "E": 3, "H": 4, "L": 5, "A": 7,
		"IXH": 4, "IXL": 5, "IYH": 4, "IYL": 5,
	}
	for name, want := range cases {
		got, err := Reg8(name)
		if err != nil {
			t.Errorf("Reg8(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("Reg8(%q) = %d, want %d", name, got, want)
		}
	}
	if _, err := Reg8("IX"); err == nil {
		t.Error("Reg8(\"IX\") expected an error (not an 8-bit register), got none")
	}
}

func TestReg16(t *testing.T) {
	cases := map[string]uint8{
		"BC": 0, "DE": 1, "HL": 2, "IX": 2, "IY": 2, "SP": 3,
	}
	for name, want := range cases {
		got, err := Reg16(name)
		if err != nil {
			t.Errorf("Reg16(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("Reg16(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestStackReg16(t *testing.T) {
	cases := map[string]uint8{"BC": 0, "DE": 1, "HL": 2, "AF": 3}
	for name, want := range cases {
		got, err := StackReg16(name)
		if err != nil {
			t.Errorf("StackReg16(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("StackReg16(%q) = %d, want %d", name, got, want)
		}
	}
	if _, err := StackReg16("SP"); err == nil {
		t.Error("StackReg16(\"SP\") expected an error (PUSH/POP never take SP), got none")
	}
}

func TestCondition(t *testing.T) {
	cases := map[string]uint8{
		"NZ": 0, "Z": 1, "NC": 2, "C": 3, "PO": 4, "PE": 5, "P": 6, "M": 7,
	}
	for name, want := range cases {
		got, err := Condition(name)
		if err != nil {
			t.Errorf("Condition(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("Condition(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestJRCondition(t *testing.T) {
	cases := map[string]uint8{"NZ": 0, "Z": 1, "NC": 2, "C": 3}
	for name, want := range cases {
		got, err := JRCondition(name)
		if err != nil {
			t.Errorf("JRCondition(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("JRCondition(%q) = %d, want %d", name, got, want)
		}
	}
	// JR has no PO/PE/P/M forms, unlike JP/CALL.
	for _, name := range []string{"PO", "PE", "P", "M"} {
		if _, err := JRCondition(name); err == nil {
			t.Errorf("JRCondition(%q) expected an error (not valid for JR/DJNZ), got none", name)
		}
	}
}

func TestIndexPrefix(t *testing.T) {
	if got, err := IndexPrefix("IX"); err != nil || got != 0xDD {
		t.Errorf("IndexPrefix(\"IX\") = %#02x, %v, want 0xDD, nil", got, err)
	}
	if got, err := IndexPrefix("IY"); err != nil || got != 0xFD {
		t.Errorf("IndexPrefix(\"IY\") = %#02x, %v, want 0xFD, nil", got, err)
	}
	if _, err := IndexPrefix("HL"); err == nil {
		t.Error("IndexPrefix(\"HL\") expected an error, got none")
	}
}
