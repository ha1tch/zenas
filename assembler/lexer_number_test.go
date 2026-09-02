package assembler

import "testing"

// TestHexLiteralWithLeadingLetterDigit covers a real bug found by the
// zenpoint project: a traditional assembler hex literal whose first
// significant digit is itself a letter (0BCH, 0FFh -- the leading 0
// is conventional, to keep it from looking like an identifier) was
// misread as a broken 0b-prefixed binary literal, since both the
// lexer's own dispatch (scanNumber vs scanBinaryNumber0b) and the
// separate, exported ParseNumber checked "does this start with 0b?"
// before ever checking "does this end in H?". Two distinct fixes were
// needed, one per site; this test exercises the full path (Tokenize
// then ParseNumber) so a regression in either shows up here.
func TestHexLiteralWithLeadingLetterDigit(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"0BCH", 0xBC},
		{"0FFh", 0xFF},
		{"0B0H", 0xB0}, // the digit right after "0B" is itself a valid
		// hex digit ('0'), not just a non-hex terminator -- confirms
		// the lookahead scans the whole hex-digit run, not just one
		// character past the "0B".
		{"05h", 0x05},
		{"0DEADh", 0xDEAD},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			l := NewLexer()
			tokens, err := l.Tokenize(c.src)
			if err != nil {
				t.Fatalf("Tokenize(%q): %v", c.src, err)
			}
			if len(tokens) == 0 || tokens[0].Type != TokenNumber {
				t.Fatalf("Tokenize(%q) = %+v, want a single NUMBER token", c.src, tokens)
			}
			if tokens[0].Value != c.src {
				t.Errorf("token value = %q, want %q (the literal split or was misrouted)", tokens[0].Value, c.src)
			}
			got, err := ParseNumber(tokens[0])
			if err != nil {
				t.Fatalf("ParseNumber(%q): %v", c.src, err)
			}
			if got != c.want {
				t.Errorf("ParseNumber(%q) = %d, want %d", c.src, got, c.want)
			}
		})
	}
}

// TestSpacedHexdumpRadixMarkersStillWork is the regression guard for
// the 0d-dispatch fix above: the genuine "0d" radix marker (the spaced
// decimal-hexdump form for .DB/.DW, e.g. ".DB 0d 222 173") must still
// be recognised as a marker, not accidentally routed into the
// hex-with-H-suffix path meant for literals like 0DEADh.
func TestSpacedHexdumpRadixMarkersStillWork(t *testing.T) {
	l := NewLexer()
	tokens, err := l.Tokenize("0d 222 173")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if len(tokens) == 0 || tokens[0].Type != TokenIdentifier || tokens[0].Value != "0d" {
		t.Fatalf("Tokenize(\"0d 222 173\") first token = %+v, want IDENTIFIER \"0d\"", tokens[0])
	}
}

// TestGenuineBinaryLiteralStillWorks is the regression guard for the
// fix above: a real 0b-prefixed binary literal must still be read as
// binary, not accidentally routed into the hex-with-H-suffix path.
func TestGenuineBinaryLiteralStillWorks(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"0b10101010", 0xAA},
		{"0B11110000", 0xF0},
		{"0b1111_0000", 0xF0}, // underscore separator still allowed
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			l := NewLexer()
			tokens, err := l.Tokenize(c.src)
			if err != nil {
				t.Fatalf("Tokenize(%q): %v", c.src, err)
			}
			if len(tokens) == 0 || tokens[0].Type != TokenNumber {
				t.Fatalf("Tokenize(%q) = %+v, want a single NUMBER token", c.src, tokens)
			}
			got, err := ParseNumber(tokens[0])
			if err != nil {
				t.Fatalf("ParseNumber(%q): %v", c.src, err)
			}
			if got != c.want {
				t.Errorf("ParseNumber(%q) = %d, want %d", c.src, got, c.want)
			}
		})
	}
}
