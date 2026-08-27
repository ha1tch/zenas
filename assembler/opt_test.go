package assembler

import (
	"strings"
	"testing"
)

// TestOptZxnextEnablesZ80N checks that "OPT ZXNEXT" in the source has the
// same effect as EnableZ80N() / the command line's --next: a Z80N-only
// instruction (ADD HL,A, opcode ED 31) assembles without needing the
// caller to remember to enable Next mode externally.
func TestOptZxnextEnablesZ80N(t *testing.T) {
	src := "OPT ZXNEXT\n\torg 32768\n\tadd hl,a\n"
	a := New()
	result, err := a.AssembleString(src)
	if err != nil {
		t.Fatalf("AssembleString: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("assembly errors: %v", result.Errors)
	}
	want := []byte{0xED, 0x31}
	if len(result.MachineCode) != len(want) {
		t.Fatalf("machine code = % X, want % X", result.MachineCode, want)
	}
	for i := range want {
		if result.MachineCode[i] != want[i] {
			t.Errorf("machine code = % X, want % X", result.MachineCode, want)
			break
		}
	}
}

// TestZ80NInstructionWithoutOptOrFlagFails confirms the negative case: a
// Z80N-only instruction with neither OPT ZXNEXT in the source nor
// EnableZ80N() called externally is rejected, the same as it always was --
// OPT ZXNEXT is an additional way to opt in, not a change to the default.
func TestZ80NInstructionWithoutOptOrFlagFails(t *testing.T) {
	src := "\torg 32768\n\tadd hl,a\n"
	a := New()
	result, err := a.AssembleString(src)
	if err == nil && len(result.Errors) == 0 {
		t.Fatal("expected an error assembling a Z80N instruction with Z80N mode not enabled, got none")
	}
}

// TestOptZxnextIsCaseInsensitive checks "opt zxnext" (lower case, as a
// human might actually type it) works the same as the canonical upper-case
// form -- directive names and this option name are both matched
// case-insensitively elsewhere in the assembler, and OPT should be
// consistent with that.
func TestOptZxnextIsCaseInsensitive(t *testing.T) {
	src := "opt zxnext\n\torg 32768\n\tadd hl,a\n"
	a := New()
	result, err := a.AssembleString(src)
	if err != nil {
		t.Fatalf("AssembleString: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("assembly errors: %v", result.Errors)
	}
	if len(result.MachineCode) != 2 {
		t.Fatalf("machine code = % X, want 2 bytes", result.MachineCode)
	}
}

// TestOptUnknownOptionRejected checks that OPT with an option name other
// than ZXNEXT is a clear assembly error, not silently ignored -- a typo in
// an OPT line should not produce a binary that quietly assembled without
// the Z80N support the source actually needed.
func TestOptUnknownOptionRejected(t *testing.T) {
	src := "OPT NOTAREALOPTION\n\torg 32768\n\tnop\n"
	a := New()
	result, err := a.AssembleString(src)
	if err == nil && len(result.Errors) == 0 {
		t.Fatal("expected an error for an unrecognised OPT option, got none")
	}
	// Check the error actually names the problem, not just that one occurred.
	msg := errString(err, result)
	if !strings.Contains(msg, "NOTAREALOPTION") {
		t.Errorf("error message %q does not mention the bad option name", msg)
	}
}

// TestOptRequiresArgument checks a bare "OPT" with no option name is
// rejected at parse time rather than panicking or being silently ignored.
func TestOptRequiresArgument(t *testing.T) {
	src := "OPT\n\torg 32768\n\tnop\n"
	a := New()
	result, err := a.AssembleString(src)
	if err == nil && len(result.Errors) == 0 {
		t.Fatal("expected an error for OPT with no argument, got none")
	}
}

// errString collects a readable message from every source a failure could
// surface through. AssembleString's own returned error is a generic "N
// errors" wrapper; the actual per-error detail lives in result.Errors
// regardless of whether the wrapper error is also non-nil, so both are
// always checked rather than returning early on the wrapper alone (which
// would silently hide the real message this helper exists to surface).
func errString(err error, result *AssemblyResult) string {
	var b strings.Builder
	if err != nil {
		b.WriteString(err.Error())
		b.WriteString("; ")
	}
	if result != nil {
		for _, e := range result.Errors {
			b.WriteString(e.Message)
			b.WriteString("; ")
		}
	}
	return b.String()
}
