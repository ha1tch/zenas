package assembler

import "testing"

// TestCyclesFor is the systematic regression test for the clock-count
// review: every case here was verified against a reference Z80 timing
// table (map.grauw.nl/resources/z80instr.php) and represents a real
// bug found and fixed, not a case chosen because it was already
// correct. Each pair contrasts the value a shared template used to
// report (wrong for one of the two cases) against the now-correct
// value for both.
func TestCyclesFor(t *testing.T) {
	cases := []struct {
		name     string
		mnemonic string
		operands []*ResolvedOperand
		want     int
	}{
		// LD r,r': plain vs half-index register.
		{"LD B,A", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "B"}, {Type: OperandRegister8, Register: "A"}}, 4},
		{"LD IXH,A", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "IXH"}, {Type: OperandRegister8, Register: "A"}}, 8},
		{"LD A,I", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandRegister8, Register: "I"}}, 9},

		// LD r,n: plain vs half-index.
		{"LD B,n", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "B"}, {Type: OperandImmediate8, Value: 5}}, 7},
		{"LD IXH,n", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "IXH"}, {Type: OperandImmediate8, Value: 5}}, 11},

		// LD r,(...): (HL) vs (IX+d) vs (BC) vs absolute.
		{"LD B,(HL)", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "B"}, {Type: OperandIndirect, Register: "HL"}}, 7},
		{"LD B,(IX+d)", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "B"}, {Type: OperandIndirect, Register: "IX", Displacement: 5}}, 19},
		{"LD A,(BC)", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandIndirect, Register: "BC"}}, 7},
		{"LD A,(nn)", "LD", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandIndirect, Value: 0x9000}}, 13},

		// LD (...),r: (HL) vs (IX+d).
		{"LD (HL),B", "LD", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}, {Type: OperandRegister8, Register: "B"}}, 7},
		{"LD (IX+d),B", "LD", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX", Displacement: 5}, {Type: OperandRegister8, Register: "B"}}, 19},

		// LD (...),n: (HL) vs (IX+d).
		{"LD (HL),n", "LD", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}, {Type: OperandImmediate8, Value: 0x42}}, 10},
		{"LD (IX+d),n", "LD", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX", Displacement: 5}, {Type: OperandImmediate8, Value: 0x42}}, 19},

		// 16-bit LD rp,nn: BC vs IX.
		{"LD BC,nn", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "BC"}, {Type: OperandImmediate16, Value: 0x1234}}, 10},
		{"LD IX,nn", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}, {Type: OperandImmediate16, Value: 0x1234}}, 14},

		// LD rp,(nn): HL vs BC vs IX.
		{"LD HL,(nn)", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "HL"}, {Type: OperandIndirect, Value: 0x9000}}, 16},
		{"LD BC,(nn)", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "BC"}, {Type: OperandIndirect, Value: 0x9000}}, 20},
		{"LD IX,(nn)", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}, {Type: OperandIndirect, Value: 0x9000}}, 20},

		// LD (nn),rp: HL vs BC.
		{"LD (nn),HL", "LD", []*ResolvedOperand{{Type: OperandIndirect, Value: 0x9000}, {Type: OperandRegister16, Register: "HL"}}, 16},
		{"LD (nn),BC", "LD", []*ResolvedOperand{{Type: OperandIndirect, Value: 0x9000}, {Type: OperandRegister16, Register: "BC"}}, 20},

		// LD SP,HL vs LD SP,IX.
		{"LD SP,HL", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "SP"}, {Type: OperandRegister16, Register: "HL"}}, 6},
		{"LD SP,IX", "LD", []*ResolvedOperand{{Type: OperandRegister16, Register: "SP"}, {Type: OperandRegister16, Register: "IX"}}, 10},

		// ALU A,r: plain vs half-index.
		{"ADD A,B", "ADD", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandRegister8, Register: "B"}}, 4},
		{"ADD A,IXH", "ADD", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandRegister8, Register: "IXH"}}, 8},

		// ALU A,(...): (HL) vs (IX+d).
		{"AND (HL)", "AND", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}}, 7},
		{"AND (IX+d)", "AND", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX", Displacement: 5}}, 19},

		// INC/DEC (...): (HL) vs (IX+d).
		{"INC (HL)", "INC", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}}, 11},
		{"INC (IX+d)", "INC", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX", Displacement: 5}}, 23},

		// INC/DEC r: plain vs half-index.
		{"INC B", "INC", []*ResolvedOperand{{Type: OperandRegister8, Register: "B"}}, 4},
		{"INC IXH", "INC", []*ResolvedOperand{{Type: OperandRegister8, Register: "IXH"}}, 8},

		// ADD HL,rr vs ADD IX,rr.
		{"ADD HL,DE", "ADD", []*ResolvedOperand{{Type: OperandRegister16, Register: "HL"}, {Type: OperandRegister16, Register: "DE"}}, 11},
		{"ADD IX,DE", "ADD", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}, {Type: OperandRegister16, Register: "DE"}}, 15},

		// INC/DEC rp: BC vs IX.
		{"INC BC", "INC", []*ResolvedOperand{{Type: OperandRegister16, Register: "BC"}}, 6},
		{"INC IX", "INC", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}}, 10},

		// PUSH/POP: BC vs IX.
		{"PUSH BC", "PUSH", []*ResolvedOperand{{Type: OperandRegister16, Register: "BC"}}, 11},
		{"PUSH IX", "PUSH", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}}, 15},
		{"POP BC", "POP", []*ResolvedOperand{{Type: OperandRegister16, Register: "BC"}}, 10},
		{"POP IX", "POP", []*ResolvedOperand{{Type: OperandRegister16, Register: "IX"}}, 14},

		// JP (HL) vs JP (IX).
		{"JP (HL)", "JP", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}}, 4},
		{"JP (IX)", "JP", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX"}}, 8},

		// OUT/IN immediate-port vs register-port.
		{"OUT (n),A", "OUT", []*ResolvedOperand{{Type: OperandIndirect, Value: 0xFE}, {Type: OperandRegister8, Register: "A"}}, 11},
		{"OUT (C),B", "OUT", []*ResolvedOperand{{Type: OperandIndirect, Register: "C"}, {Type: OperandRegister8, Register: "B"}}, 12},
		{"IN A,(n)", "IN", []*ResolvedOperand{{Type: OperandRegister8, Register: "A"}, {Type: OperandIndirect, Value: 0xFE}}, 11},
		{"IN D,(C)", "IN", []*ResolvedOperand{{Type: OperandRegister8, Register: "D"}, {Type: OperandIndirect, Register: "C"}}, 12},

		// BIT b,(...): (HL) vs (IX+d).
		{"BIT 0,(HL)", "BIT", []*ResolvedOperand{{Type: OperandImmediate8, Value: 0}, {Type: OperandIndirect, Register: "HL"}}, 12},
		{"BIT 0,(IX+d)", "BIT", []*ResolvedOperand{{Type: OperandImmediate8, Value: 0}, {Type: OperandIndirect, Register: "IX", Displacement: 5}}, 20},

		// RES/SET b,(...): (HL) vs (IX+d).
		{"RES 0,(HL)", "RES", []*ResolvedOperand{{Type: OperandImmediate8, Value: 0}, {Type: OperandIndirect, Register: "HL"}}, 15},
		{"RES 0,(IX+d)", "RES", []*ResolvedOperand{{Type: OperandImmediate8, Value: 0}, {Type: OperandIndirect, Register: "IX", Displacement: 5}}, 23},

		// Rotate/shift (...): (HL) vs (IX+d).
		{"RLC (HL)", "RLC", []*ResolvedOperand{{Type: OperandIndirect, Register: "HL"}}, 15},
		{"RLC (IX+d)", "RLC", []*ResolvedOperand{{Type: OperandIndirect, Register: "IX", Displacement: 5}}, 23},

		// EX (SP),HL vs EX (SP),IX -- IX/IY were entirely unsupported
		// before this review (a genuine gap, not a cycle-count bug).
		{"EX (SP),HL", "EX", []*ResolvedOperand{{Type: OperandIndirect, Register: "SP"}, {Type: OperandRegister16, Register: "HL"}}, 19},
		{"EX (SP),IX", "EX", []*ResolvedOperand{{Type: OperandIndirect, Register: "SP"}, {Type: OperandRegister16, Register: "IX"}}, 23},

		// The 3 isolated wrong-value fixes.
		{"RET C (taken)", "RET", []*ResolvedOperand{{Type: OperandCondition, Condition: "C"}}, 11},
		{"INIR", "INIR", nil, 21},
		{"LDIR", "LDIR", nil, 21},
		{"CPIR", "CPIR", nil, 21},
		{"OTIR", "OTIR", nil, 21},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewEncoder()
			_, cycles, err := e.EncodeWithCycles(c.mnemonic, c.operands)
			if err != nil {
				t.Fatalf("EncodeWithCycles(%q, ...): %v", c.mnemonic, err)
			}
			if cycles != c.want {
				t.Errorf("cycles = %d, want %d", cycles, c.want)
			}
		})
	}
}

// TestCyclesForZ80N covers the Z80N-specific cycle-count errors found
// during the same review, cross-checked against a source that
// measured every Z80N timing against a T80N FPGA core simulation
// rather than quoting published figures (docs.rs/crate/hg80, Z80N.md)
// -- OUTINB, LDIX, LDWS, LDDX, LDIRX, LDPIRX, and LDDRX were all
// wrongly sharing SWAPNIB/PIXELDN/PIXELAD/SETAE's 8T-state value.
// LDIRSCALE is also covered here since it surfaced in the same pass:
// not a wrong cycle count but a genuinely missing instruction (ED B6,
// a plain repeating copy despite its name), added alongside the
// timing fixes rather than left for a separate pass.
func TestCyclesForZ80N(t *testing.T) {
	cases := []struct {
		name     string
		mnemonic string
		want     int
	}{
		{"SWAPNIB", "SWAPNIB", 8},
		{"OUTINB", "OUTINB", 16},
		{"PIXELDN", "PIXELDN", 8},
		{"LDIX", "LDIX", 16},
		{"LDWS", "LDWS", 14},
		{"LDDX", "LDDX", 16},
		{"LDIRX", "LDIRX", 21},
		{"LDIRSCALE", "LDIRSCALE", 21},
		{"LDPIRX", "LDPIRX", 21},
		{"LDDRX", "LDDRX", 21},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewEncoder()
			e.EnableZ80N()
			_, cycles, err := e.EncodeWithCycles(c.mnemonic, nil)
			if err != nil {
				t.Fatalf("EncodeWithCycles(%q, ...): %v", c.mnemonic, err)
			}
			if cycles != c.want {
				t.Errorf("cycles = %d, want %d", cycles, c.want)
			}
		})
	}
}
