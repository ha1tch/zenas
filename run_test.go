package main

import (
	"testing"

	"github.com/ha1tch/zenas/assembler"
)

// runSource assembles source and executes it with the given options, returning
// the run result. callAddr of -1 means whole-program (run to HALT).
func runSource(t *testing.T, source string, opts runOptions, callAddr int) runResult {
	t.Helper()
	asm := assembler.New()
	result, err := asm.AssembleString(source)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, result.Errors)
	}
	rr, _ := execute(result, opts, callAddr)
	return rr
}

func TestRunHaltsAndComputes(t *testing.T) {
	// Sum 5+4+3+2+1 into A via a DJNZ loop.
	src := "        ORG $8000\n        LD A, 0\n        LD B, 5\nloop:   ADD A, B\n        DJNZ loop\n        HALT\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
	if rr.A != 0x0F {
		t.Errorf("A = %#x, want 0x0F", rr.A)
	}
}

func TestRunDeterministicZeroStart(t *testing.T) {
	// LD does not affect flags; with zeroed start, A must be exactly the loaded
	// value and not influenced by power-on garbage.
	src := "        ORG $8000\n        LD A, $42\n        HALT\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.A != 0x42 {
		t.Errorf("A = %#x, want 0x42", rr.A)
	}
}

func TestRunMaxStepsGuard(t *testing.T) {
	// An infinite loop must stop at the cap, not hang.
	src := "        ORG $8000\nspin:   JR spin\n"
	rr := runSource(t, src, runOptions{maxSteps: 50}, -1)
	if rr.Outcome != "max_steps" {
		t.Fatalf("outcome = %q, want max_steps", rr.Outcome)
	}
	if rr.Steps != 50 {
		t.Errorf("steps = %d, want 50", rr.Steps)
	}
}

func TestRunCallSubroutine(t *testing.T) {
	// A wrapper routine loads an input and calls the routine under test; --call
	// it and check the result is returned.
	src := "        ORG $8000\n        HALT\nwrap:   LD A, 7\n        CALL dbl\n        RET\ndbl:    ADD A, A\n        RET\n"
	asm := assembler.New()
	result, err := asm.AssembleString(src)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, result.Errors)
	}
	addr, ok := result.Symbols["wrap"]
	if !ok {
		t.Fatal("label wrap not found")
	}
	rr, _ := execute(result, runOptions{maxSteps: defaultMaxSteps}, int(addr))
	if rr.Outcome != "returned" {
		t.Fatalf("outcome = %q, want returned", rr.Outcome)
	}
	if rr.A != 14 {
		t.Errorf("A = %d, want 14", rr.A)
	}
}

func TestRunShadowRegisters(t *testing.T) {
	// EX AF,AF' then EXX moves values into the shadow set.
	src := "        ORG $8000\n        LD A, $11\n        EX AF, AF'\n        LD BC, $2233\n        EXX\n        HALT\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.AF_>>8 != 0x11 {
		t.Errorf("AF_ high = %#x, want 0x11", rr.AF_>>8)
	}
	if rr.BC_ != 0x2233 {
		t.Errorf("BC_ = %#x, want 0x2233", rr.BC_)
	}
}

func TestParseExpectsRegisterFlagMemory(t *testing.T) {
	exps, err := parseExpects("A=0x42,HL=0x8000,CF=1,(0xC000)=0xAB", nil)
	if err != nil {
		t.Fatalf("parseExpects error: %v", err)
	}
	if len(exps) != 4 {
		t.Fatalf("got %d expectations, want 4", len(exps))
	}
	if exps[0].kind != "reg" || exps[0].target != "A" || exps[0].want != 0x42 {
		t.Errorf("exp[0] = %+v", exps[0])
	}
	if exps[2].kind != "flag" || exps[2].target != "C" || exps[2].want != 1 {
		t.Errorf("exp[2] (CF) = %+v", exps[2])
	}
	if exps[3].kind != "mem" || exps[3].addr != 0xC000 || exps[3].want != 0xAB {
		t.Errorf("exp[3] (mem) = %+v", exps[3])
	}
}

func TestParseExpectsRegisterCNotFlag(t *testing.T) {
	// Bare C must be register C, not the carry flag (the ambiguity CF resolves).
	exps, err := parseExpects("C=7", nil)
	if err != nil {
		t.Fatalf("parseExpects error: %v", err)
	}
	if exps[0].kind != "reg" || exps[0].target != "C" {
		t.Errorf("C should parse as register, got %+v", exps[0])
	}
}

func TestAssembleDataFragmentRadixForms(t *testing.T) {
	// The .MATCH data path must reuse the real data-assembly path, including the
	// glued and spaced radix forms.
	cases := []struct {
		data string
		want []uint8
	}{
		{".db 0x600DF00D", []uint8{0x60, 0x0D, 0xF0, 0x0D}},
		{".db 0x 60 0D F0 0D", []uint8{0x60, 0x0D, 0xF0, 0x0D}},
		{".dw 0x 600D F00D", []uint8{0x0D, 0x60, 0x0D, 0xF0}},
		{".db 0d 96 13 240 13", []uint8{0x60, 0x0D, 0xF0, 0x0D}},
	}
	for _, c := range cases {
		got, err := assembleDataFragment(c.data, false)
		if err != nil {
			t.Errorf("%q: error %v", c.data, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%q: got %d bytes, want %d", c.data, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: byte %d = %#x, want %#x", c.data, i, got[i], c.want[i])
			}
		}
	}
}

func TestExpectRejectedOutsideTestFile(t *testing.T) {
	src := "        ORG $8000\ntest_x:\n        LD A, 5\n        RET\n.EXPECT A=5\n"
	asm := assembler.New()
	asm.SetTestFile(false) // not a _test.asm file
	result, err := asm.AssembleString(src)
	if err == nil && (result == nil || len(result.Errors) == 0) {
		t.Fatal(".EXPECT should be rejected outside a _test.asm file")
	}
}

func TestExpectMustAttachToTestRoutine(t *testing.T) {
	// An assertion on a non-test_ label is an assembly error.
	src := "        ORG $8000\nhelper:\n        LD A, 5\n        RET\n.EXPECT A=5\n"
	asm := assembler.New()
	asm.SetTestFile(true)
	result, err := asm.AssembleString(src)
	if err == nil && (result == nil || len(result.Errors) == 0) {
		t.Fatal(".EXPECT on a non-test_ label should be an assembly error")
	}
}

func TestExpectCollectedInTestFile(t *testing.T) {
	src := "        ORG $8000\ntest_a:\n        LD A, 5\n        RET\n.EXPECT A=5\ntest_b:\n        LD A, 9\n        RET\n.EXPECT A=9\n"
	asm := assembler.New()
	asm.SetTestFile(true)
	result, err := asm.AssembleString(src)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, result.Errors)
	}
	if len(result.Tests) != 2 {
		t.Fatalf("got %d test specs, want 2", len(result.Tests))
	}
	if result.Tests[0].Label != "test_a" || result.Tests[0].Expect != "A=5" {
		t.Errorf("spec[0] = %+v", result.Tests[0])
	}
	if result.Tests[1].Label != "test_b" || result.Tests[1].Expect != "A=9" {
		t.Errorf("spec[1] = %+v", result.Tests[1])
	}
}


// TestReservedMacroParamRejected verifies that a macro parameter whose name
// collides with a register or condition code is rejected at definition time,
// rather than silently assembling as that register inside the body.
func TestReservedMacroParamRejected(t *testing.T) {
	reserved := []string{"a", "b", "c", "d", "e", "h", "l", "hl", "bc", "de",
		"sp", "af", "ix", "iy", "nz", "z", "nc", "po", "pe", "p", "m", "i", "r"}
	for _, name := range reserved {
		src := "MACRO m(uint8_t " + name + ")\n    LD A, " + name + "\nENDMACRO\n" +
			"    ORG 0\nstart:\n    m(1)\n    RET\n"
		asm := assembler.New()
		res, err := asm.AssembleString(src)
		ok := err != nil || (res != nil && len(res.Errors) > 0)
		if !ok {
			t.Errorf("parameter %q was accepted; expected a reserved-name error", name)
		}
	}
}

// TestSafeMacroParamAccepted verifies that ordinary parameter names still work.
func TestSafeMacroParamAccepted(t *testing.T) {
	for _, name := range []string{"val", "mask", "addr", "count", "x", "n", "value"} {
		src := "MACRO m(uint8_t " + name + ")\n    LD A, " + name + "\nENDMACRO\n" +
			"    ORG 0\nstart:\n    m(1)\n    RET\n"
		asm := assembler.New()
		res, err := asm.AssembleString(src)
		if err != nil || (res != nil && len(res.Errors) > 0) {
			t.Errorf("parameter %q was rejected; expected it to assemble (err=%v)", name, err)
		}
	}
}

// TestCStyleProgramRunsAndComputes verifies that a C-style program with function
// calls runs to completion and computes the right result - i.e. that inlined
// returns do not abort the caller and call arguments reach the body.
func TestCStyleProgramRunsAndComputes(t *testing.T) {
	// triple(7): body computes A*3 via B; main stores A at 0xC000 and halts.
	src := ".MACRO_STYLE C\n" +
		"uint8_t triple(uint8_t value) { asm { LD B, A; ADD A, B; ADD A, B; } return value; }\n" +
		"void main() { triple(7); asm { LD (0xC000), A; HALT; } }\n" +
		".END\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted (a C-style program must run to HALT, not RET into garbage)", rr.Outcome)
	}
	if rr.A != 21 {
		t.Errorf("A = %d, want 21 (7*3)", rr.A)
	}
}

// TestCStyleMultiCallChain verifies several calls in sequence each take effect,
// the failure mode that an inlined RET previously caused.
func TestCStyleMultiCallChain(t *testing.T) {
	// add_five(10) -> 15, then add_two(15,25) -> 40, stored and halted.
	src := ".MACRO_STYLE C\n" +
		"uint8_t add_five(uint8_t value) { asm { ADD A, 5; } return value; }\n" +
		"uint8_t add_two(uint8_t first, uint8_t second) { asm { ADD A, B; } return first; }\n" +
		"void main() { add_five(10); add_two(15, 25); asm { LD (0xC000), A; HALT; } }\n" +
		".END\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
	if rr.A != 40 {
		t.Errorf("A = %d, want 40 (15+25)", rr.A)
	}
}

// TestCStyleIdentifierCollisionsRejected verifies that C-style function names,
// variable names, and parameter names cannot collide with registers/conditions,
// and that a mnemonic-named function with no package is rejected.
func TestCStyleIdentifierCollisionsRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"function named after register", ".MACRO_STYLE C\nuint8_t hl(uint8_t value) { asm { ADD A,1; } return value; }\nvoid main() { asm { HALT; } }\n.END\n"},
		{"body variable named after register", ".MACRO_STYLE C\nvoid main() { uint8_t a; asm { HALT; } }\n.END\n"},
		{"parameter named after register", ".MACRO_STYLE C\nuint8_t f(uint8_t b) { asm { ADD A,1; } return b; }\nvoid main() { asm { HALT; } }\n.END\n"},
		{"function named after mnemonic, no package", ".MACRO_STYLE C\nuint8_t add(uint8_t first, uint8_t second) { asm { ADD A,B; } return first; }\nvoid main() { asm { HALT; } }\n.END\n"},
	}
	for _, tc := range cases {
		asm := assembler.New()
		res, err := asm.AssembleString(tc.src)
		if err == nil && (res == nil || len(res.Errors) == 0) {
			t.Errorf("%s: accepted; expected a collision error", tc.name)
		}
	}
}

// TestCStyleMnemonicNameInPackageWorks verifies that a mnemonic-named function
// is usable when placed in a package and called qualified.
func TestCStyleMnemonicNameInPackageWorks(t *testing.T) {
	src := ".MACRO_STYLE C\n.PACKAGE arith\n" +
		"uint8_t add(uint8_t first, uint8_t second) { asm { ADD A, B; } return first; }\n" +
		"void main() { arith.add(15, 25); asm { LD (0xC000), A; HALT; } }\n.END\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
}

// TestIODoesNotCrash verifies that running code using OUT/IN does not panic the
// harness when no I/O device is attached.
func TestIODoesNotCrash(t *testing.T) {
	src := "        ORG $8000\n        LD A, $42\n        OUT ($90), A\n        IN A, ($90)\n        HALT\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted (IN/OUT must not crash the run harness)", rr.Outcome)
	}
}

// TestCStyleDirectAssignmentStores verifies that a C-style variable assignment
// inside a function body actually allocates storage and stores the value, so it
// can be read back.
func TestCStyleDirectAssignmentStores(t *testing.T) {
	src := ".MACRO_STYLE C\n" +
		"void main() {\n" +
		"    uint8_t total;\n" +
		"    total = 42;\n" +
		"    asm { LD A, 0; LD A, (total); HALT; }\n" +
		"}\n.END\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
	if rr.A != 42 {
		t.Errorf("A = %d, want 42 (variable store/load failed)", rr.A)
	}
}

// TestCStyleAssignmentCapturesCallResult verifies that assigning a packaged
// call's result to a variable stores the computed value.
func TestCStyleAssignmentCapturesCallResult(t *testing.T) {
	src := ".MACRO_STYLE C\n.PACKAGE math\n" +
		"uint8_t add(uint8_t first, uint8_t second) { asm { ADD A, B; } return first; }\n" +
		"void main() {\n" +
		"    uint8_t total;\n" +
		"    total = math.add(20, 22);\n" +
		"    asm { LD A, 0; LD A, (total); HALT; }\n" +
		"}\n.END\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
	if rr.A != 42 {
		t.Errorf("A = %d, want 42 (20+22 via captured call result)", rr.A)
	}
}

// TestExpectAcceptsSymbolAddress verifies that --expect / .EXPECT memory targets
// accept a symbol name, resolved against the assembled symbol table, in addition
// to numeric addresses.
func TestExpectAcceptsSymbolAddress(t *testing.T) {
	// With a symbol table available, a symbol address resolves at parse time.
	symbols := map[string]uint16{"total": 0xC000}
	exps, err := parseExpects("(total)=42", symbols)
	if err != nil {
		t.Fatalf("parseExpects with symbol: %v", err)
	}
	if len(exps) != 1 || exps[0].kind != "mem" || exps[0].addr != 0xC000 || exps[0].want != 42 {
		t.Fatalf("unexpected expectation: %+v", exps)
	}

	// Without a table (CLI path), the symbol is deferred then resolved.
	exps2, err := parseExpects("(total)=42", nil)
	if err != nil {
		t.Fatalf("parseExpects deferred: %v", err)
	}
	if exps2[0].addrSym != "total" {
		t.Fatalf("expected deferred symbol 'total', got %q", exps2[0].addrSym)
	}
	if err := resolveExpectSymbols(exps2, symbols); err != nil {
		t.Fatalf("resolveExpectSymbols: %v", err)
	}
	if exps2[0].addr != 0xC000 || exps2[0].addrSym != "" {
		t.Fatalf("symbol not resolved: %+v", exps2[0])
	}

	// A numeric address still works.
	exps3, err := parseExpects("(0xC000)=42", nil)
	if err != nil || exps3[0].addr != 0xC000 {
		t.Fatalf("numeric address regressed: %+v %v", exps3, err)
	}

	// An unknown symbol is an error on resolution.
	bad, _ := parseExpects("(nope)=1", nil)
	if err := resolveExpectSymbols(bad, symbols); err == nil {
		t.Fatalf("expected error for unknown symbol")
	}
}

// TestSingletonModeRunsViaCall verifies that .MACRO_MODE SINGLETON emits a
// parameterless macro body once and reaches it by CALL from each instantiation,
// producing the same runtime effect as inlining.
func TestSingletonModeRunsViaCall(t *testing.T) {
	src := ".MACRO_MODE SINGLETON\n" +
		"MACRO step()\n    INC HL\n    INC HL\nENDMACRO\n" +
		"    ORG 0x8000\n" +
		"    LD HL, 0xC000\n    step()\n    step()\n    step()\n    HALT\n"
	rr := runSource(t, src, runOptions{maxSteps: defaultMaxSteps}, -1)
	if rr.Outcome != "halted" {
		t.Fatalf("outcome = %q, want halted", rr.Outcome)
	}
	if rr.HL != 0xC006 {
		t.Errorf("HL = %#x, want 0xC006 (3 calls x INC HL x2)", rr.HL)
	}
}

// TestSingletonEmitsBodyOnce verifies SINGLETON produces smaller output than
// INLINE when a macro is instantiated several times (body shared, not repeated).
func TestSingletonEmitsBodyOnce(t *testing.T) {
	body := "MACRO big()\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\n    NOP\nENDMACRO\n" +
		"    ORG 0x8000\n    big()\n    big()\n    big()\n    big()\n    HALT\n"
	asmI := assembler.New()
	resI, errI := asmI.AssembleString(".MACRO_MODE INLINE\n" + body)
	asmS := assembler.New()
	resS, errS := asmS.AssembleString(".MACRO_MODE SINGLETON\n" + body)
	if errI != nil || errS != nil {
		t.Fatalf("assembly error: inline=%v singleton=%v", errI, errS)
	}
	if len(resS.MachineCode) >= len(resI.MachineCode) {
		t.Errorf("singleton (%d bytes) should be smaller than inline (%d bytes)",
			len(resS.MachineCode), len(resI.MachineCode))
	}
}

// TestSingletonParameterizedViaSlots verifies that a parameterised macro under
// SINGLETON passes its arguments through fixed memory slots: the body is emitted
// once, each call writes its argument to the slot and CALLs, so distinct calls
// produce distinct results.
func TestSingletonParameterizedViaSlots(t *testing.T) {
	src := ".MACRO_MODE SINGLETON\n" +
		"MACRO emit(uint8_t val)\n    LD A, (val)\n    LD (HL), A\n    INC HL\nENDMACRO\n" +
		"    ORG 0x8000\n    LD HL, 0xC000\n    emit(0x11)\n    emit(0x22)\n    emit(0x33)\n    HALT\n"
	asm := assembler.New()
	res, err := asm.AssembleString(src)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, res.Errors)
	}
	_, ram := execute(res, runOptions{maxSteps: defaultMaxSteps}, -1)
	for i, want := range []uint8{0x11, 0x22, 0x33} {
		if got := ram.Read(0xC000 + uint16(i)); got != want {
			t.Errorf("(0xC000+%d) = %#x, want %#x", i, got, want)
		}
	}
}

// TestSingletonRejectsRecursiveMacro verifies that a recursive macro is refused
// under SINGLETON, because its fixed argument slots are not re-entrant. A
// non-recursive macro that calls a different macro is still allowed.
func TestSingletonRejectsRecursiveMacro(t *testing.T) {
	// Direct self-recursion: rejected.
	rec := ".MACRO_MODE SINGLETON\n" +
		"MACRO countdown(uint8_t n)\n    LD A, (n)\n    DEC A\n    countdown(0)\nENDMACRO\n" +
		"    ORG 0x8000\n    countdown(5)\n    HALT\n"
	asm := assembler.New()
	res, err := asm.AssembleString(rec)
	if err == nil && (res == nil || len(res.Errors) == 0) {
		t.Errorf("recursive macro accepted under SINGLETON; expected rejection")
	}

	// A non-recursive macro calling a different macro (a chain, not a cycle) is
	// allowed.
	chain := ".MACRO_MODE SINGLETON\n" +
		"MACRO helper(uint8_t x)\n    LD A, (x)\n    INC A\nENDMACRO\n" +
		"MACRO caller(uint8_t y)\n    LD A, (y)\n    helper(0)\nENDMACRO\n" +
		"    ORG 0x8000\n    caller(5)\n    HALT\n"
	asm2 := assembler.New()
	res2, err2 := asm2.AssembleString(chain)
	if err2 != nil || (res2 != nil && len(res2.Errors) > 0) {
		t.Errorf("non-recursive chain rejected under SINGLETON: %v %v", err2, res2.Errors)
	}
}
