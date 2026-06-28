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
	exps, err := parseExpects("A=0x42,HL=0x8000,CF=1,(0xC000)=0xAB")
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
	exps, err := parseExpects("C=7")
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

