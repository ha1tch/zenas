package assembler

import "testing"

// TestLowHighByteExtraction covers a real gap found porting Oakhollow
// (a pasmo-sourced ZX Spectrum game) to zenas: pasmo's LOW/HIGH unary
// byte-extraction operators (LOW x = bits 0-7 of x, HIGH x = bits 8-15)
// were not implemented at all -- not gated behind a dialect mode, simply
// absent from the expression grammar. The failing construct was
// `CP LOW PERIM` / `CP HIGH PERIM` where PERIM is a forward-referenced
// EQU constant, which is exactly what this test exercises: the bare
// instruction-operand path (parseIdentifierOperand), not just the
// directive-argument / parenthesised-expression path
// (parsePrimaryExpression) that a narrower fix might have covered alone.
func TestLowHighByteExtraction(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []byte
	}{
		{
			name: "CP LOW of a forward-referenced constant",
			src:  "\torg 32768\n\tcp low period\nperiod\tequ 0x1234\n",
			want: []byte{0xFE, 0x34}, // CP n ; n = low(0x1234) = 0x34
		},
		{
			name: "CP HIGH of a forward-referenced constant",
			src:  "\torg 32768\n\tcp high period\nperiod\tequ 0x1234\n",
			want: []byte{0xFE, 0x12}, // CP n ; n = high(0x1234) = 0x12
		},
		{
			name: "LOW/HIGH are case-insensitive, like other zenas keywords",
			src:  "\torg 32768\n\tCP High period\n\tcp Low period\nperiod\tequ 0x1234\n",
			want: []byte{0xFE, 0x12, 0xFE, 0x34},
		},
		{
			name: "LOW/HIGH in a DB directive (directive-argument path)",
			src:  "\torg 32768\n\tdb low period, high period\nperiod\tequ 0xBEEF\n",
			want: []byte{0xEF, 0xBE},
		},
		{
			name: "LOW/HIGH of a numeric literal, no symbol involved",
			src:  "\torg 32768\n\tcp low 0x02FF\n",
			want: []byte{0xFE, 0xFF},
		},
		{
			name: "HIGH truncates correctly past a byte boundary",
			src:  "\torg 32768\n\tcp high 0xFF00\n",
			want: []byte{0xFE, 0xFF},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := New()
			result, err := a.AssembleString(c.src)
			if err != nil {
				t.Fatalf("AssembleString(%q): %v", c.src, err)
			}
			if len(result.Errors) > 0 {
				t.Fatalf("assembly errors: %v", result.Errors)
			}
			if len(result.MachineCode) != len(c.want) {
				t.Fatalf("machine code = % X, want % X", result.MachineCode, c.want)
			}
			for i := range c.want {
				if result.MachineCode[i] != c.want[i] {
					t.Errorf("machine code = % X, want % X", result.MachineCode, c.want)
					break
				}
			}
		})
	}
}

// TestLowHighReservedEverywhereOperandsAreParsed guards against a fix that
// only patches one of the two operand-parsing entry points: LOW/HIGH must
// work identically whether the operand starts an instruction argument
// directly (parseIdentifierOperand) or appears after an operator/paren
// (parsePrimaryExpression). This assembles the same logical value both
// ways and checks they agree.
func TestLowHighReservedEverywhereOperandsAreParsed(t *testing.T) {
	src := "\torg 32768\n\tcp low(period+1)\nperiod\tequ 0x12FE\n"
	a := New()
	result, err := a.AssembleString(src)
	if err != nil {
		t.Fatalf("AssembleString(%q): %v", src, err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("assembly errors: %v", result.Errors)
	}
	want := []byte{0xFE, 0xFF} // low(0x12FE + 1) = low(0x12FF) = 0xFF
	if len(result.MachineCode) != len(want) || result.MachineCode[0] != want[0] || result.MachineCode[1] != want[1] {
		t.Errorf("machine code = % X, want % X", result.MachineCode, want)
	}
}
