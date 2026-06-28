package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

// runOptions holds the parsed options for the `run` subcommand.
type runOptions struct {
	inputFile string
	maxSteps  int
	trace     bool
	jsonLevel string // "" means human-readable
	hexDump   bool
	dumpStart int // -1 means default (assembled region)
	dumpLen   int
	preloads  []preload
	next      bool

	// callLabel, when non-empty, runs a subroutine: the executor pushes a
	// sentinel return address, sets PC to the label, and stops when the routine
	// RETs back to the sentinel (rather than running to HALT).
	callLabel string
	// expects holds assertions to check against final state (used by `assert`).
	expects []expectation
}

// expectation is a single assertion over post-run state.
type expectation struct {
	raw    string // original text, for reporting
	kind   string // "reg", "flag", "mem"
	target string // register name, flag letter, or address (as text)
	addr   uint16 // for kind=="mem"
	want   int    // expected value
}

// preload is a binary file to load into memory before execution.
type preload struct {
	address uint16
	path    string
}

// runResult is the structured outcome of a run, used for JSON output and to
// drive the human-readable report.
type runResult struct {
	Outcome string `json:"outcome"` // "halted", "max_steps", "error"
	Steps   int    `json:"steps"`
	Cycles  uint64 `json:"cycles"`
	Origin  uint16 `json:"origin"`

	A  uint8  `json:"a"`
	F  uint8  `json:"f"`
	AF uint16 `json:"af"`
	BC uint16 `json:"bc"`
	DE uint16 `json:"de"`
	HL uint16 `json:"hl"`
	IX uint16 `json:"ix"`
	IY uint16 `json:"iy"`
	SP uint16 `json:"sp"`
	PC uint16 `json:"pc"`

	// Alternate (shadow) register pairs, written AF_ BC_ DE_ HL_.
	AF_ uint16 `json:"af_"`
	BC_ uint16 `json:"bc_"`
	DE_ uint16 `json:"de_"`
	HL_ uint16 `json:"hl_"`

	Flags  string     `json:"flags"`
	Halted bool       `json:"halted"`
	Trace  []traceStep `json:"trace,omitempty"`
	Memory *memoryDump `json:"memory,omitempty"`
}

type traceStep struct {
	Step   int    `json:"step"`
	PC     uint16 `json:"pc"`
	Opcode uint8  `json:"opcode"`
	A      uint8  `json:"a"`
	HL     uint16 `json:"hl"`
}

type memoryDump struct {
	Start uint16  `json:"start"`
	Bytes []uint8 `json:"bytes"`
}

const defaultMaxSteps = 1000000

// callSentinel is the fake return address pushed before a --call subroutine, so
// the executor can detect the routine returning (PC == sentinel after a RET).
const callSentinel uint16 = 0xFFFF

// runDiscoveredTests runs each .EXPECT-annotated test routine in a *_test.asm
// file, go-test style: call the routine, check its expectations, report per-test
// PASS/FAIL and a summary. Exits non-zero if any test fails.
func runDiscoveredTests(result *assembler.AssemblyResult, opts runOptions) {
	// Discover test routines by the test_ prefix in the symbol table. A test_*
	// routine is a test; it must carry at least one .EXPECT/.MATCH and all must
	// pass. (Assertions are guaranteed to attach to test_* labels - the assembler
	// rejects any that do not.)
	var testLabels []string
	for name := range result.Symbols {
		if strings.HasPrefix(name, "test_") {
			testLabels = append(testLabels, name)
		}
	}
	sort.Strings(testLabels)

	if len(testLabels) == 0 {
		fmt.Println("warning: no test_ routines found in a _test.asm file")
		return
	}

	// Group assertions by label.
	byLabel := map[string][]assembler.TestSpec{}
	for _, spec := range result.Tests {
		byLabel[spec.Label] = append(byLabel[spec.Label], spec)
	}

	passed, failed := 0, 0
	for _, label := range testLabels {
		addr := result.Symbols[label]
		specs := byLabel[label]

		// A test_ routine with no assertion cannot pass - it asserts nothing.
		if len(specs) == 0 {
			fmt.Printf("FAIL  %s  (no .EXPECT or .MATCH)\n", label)
			failed++
			continue
		}

		rr, ram := execute(result, opts, int(addr))
		if rr.Outcome == "max_steps" {
			fmt.Printf("FAIL  %s  (exceeded instruction cap of %d)\n", label, opts.maxSteps)
			failed++
			continue
		}

		testPass := true
		var firstFail string
		for _, spec := range specs {
			if spec.Expect != "" {
				exps, err := parseExpects(spec.Expect)
				if err != nil {
					testPass = false
					if firstFail == "" {
						firstFail = fmt.Sprintf("bad .EXPECT: %s", err)
					}
					continue
				}
				for _, e := range exps {
					got := actualValue(e, rr, ram)
					if got != e.want {
						testPass = false
						if firstFail == "" {
							firstFail = fmt.Sprintf("%s (got %s)", e.raw, formatActual(e, got))
						}
					}
				}
			}
			if spec.Match != "" {
				ok, detail := checkMatch(spec.Match, result.Symbols, ram, opts)
				if !ok {
					testPass = false
					if firstFail == "" {
						firstFail = detail
					}
				}
			}
		}

		if testPass {
			fmt.Printf("PASS  %s\n", label)
			passed++
		} else {
			fmt.Printf("FAIL  %s  %s\n", label, firstFail)
			failed++
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// checkMatch verifies that the bytes produced by the data part of a .MATCH are
// present in memory at the given location. Returns ok and, on failure, a detail
// string giving the first differing offset (expected vs actual).
func checkMatch(raw string, symbols map[string]uint16, ram *memory.RAM, opts runOptions) (bool, string) {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return false, fmt.Sprintf(".MATCH needs 'location, data' (got %q)", raw)
	}
	locText := strings.TrimSpace(raw[:comma])
	dataText := strings.TrimSpace(raw[comma+1:])

	// Resolve the location: a symbol, or a numeric address.
	var base uint16
	if a, ok := symbols[locText]; ok {
		base = a
	} else {
		a, err := parseAddress(locText)
		if err != nil {
			return false, fmt.Sprintf(".MATCH location %q is not a symbol or address", locText)
		}
		base = a
	}

	want, err := assembleDataFragment(dataText, opts.next)
	if err != nil {
		return false, fmt.Sprintf(".MATCH data: %s", err)
	}

	for i, b := range want {
		got := ram.Read(base + uint16(i))
		if got != b {
			return false, fmt.Sprintf(".MATCH at %s+%d: expected 0x%02X, got 0x%02X", locText, i, b, got)
		}
	}
	return true, ""
}

// assembleDataFragment assembles a single data directive (e.g. ".db 0x600DF00D")
// to its bytes, reusing the real data-assembly path so the radix forms and all
// data semantics behave identically to normal assembly.
func assembleDataFragment(data string, next bool) ([]uint8, error) {
	src := "        ORG 0\n        " + data + "\n"
	asm := assembler.New()
	if next {
		asm.EnableZ80N()
	}
	res, err := asm.AssembleString(src)
	if err != nil {
		return nil, err
	}
	if res == nil || len(res.Errors) > 0 {
		msg := "assembly error"
		if res != nil && len(res.Errors) > 0 {
			msg = res.Errors[0].Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return res.MachineCode, nil
}

// handleAssert implements `zenas assert`: assemble, run (optionally a --call
// subroutine), then check --expect assertions against final state. It reports
// PASS/FAIL per assertion and exits nonzero if any fail or the run overran.
func handleAssert() {
	opts, err := parseRunArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	isTestFile := strings.HasSuffix(opts.inputFile, "_test.asm")
	if len(opts.expects) == 0 && !isTestFile {
		fmt.Fprintln(os.Stderr, "Error: assert requires --expect, or a *_test.asm file with .EXPECT routines")
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %s\n", opts.inputFile, err)
		os.Exit(1)
	}

	asm := assembler.New()
	if opts.next {
		asm.EnableZ80N()
	}
	asm.SetBaseDir(filepath.Dir(opts.inputFile))
	asm.SetTestFile(isTestFile)
	result, asmErr := asm.AssembleString(string(content))
	if asmErr != nil || result == nil || len(result.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Assembly failed; cannot assert.")
		if result != nil {
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  Line %d: %s\n", e.Line, e.Message)
			}
		}
		if asmErr != nil {
			fmt.Fprintf(os.Stderr, "  %s\n", asmErr)
		}
		os.Exit(1)
	}

	// Discovery mode: a *_test.asm file with .EXPECT-annotated test_* routines and
	// no explicit --expect runs each discovered test, go-test style.
	if isTestFile && len(opts.expects) == 0 {
		runDiscoveredTests(result, opts)
		return
	}

	callAddr := -1
	if opts.callLabel != "" {
		addr, ok := result.Symbols[opts.callLabel]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: --call label %q not found\n", opts.callLabel)
			os.Exit(1)
		}
		callAddr = int(addr)
	}

	rr, ram := execute(result, opts, callAddr)

	// An overrun is a failure regardless of assertions: the code ran away.
	if rr.Outcome == "max_steps" {
		fmt.Printf("FAIL  exceeded instruction cap of %d (possible infinite loop)\n", opts.maxSteps)
		os.Exit(1)
	}

	allPass := true
	for _, e := range opts.expects {
		got := actualValue(e, rr, ram)
		if got == e.want {
			fmt.Printf("PASS  %s\n", e.raw)
		} else {
			allPass = false
			fmt.Printf("FAIL  %s  (got %s)\n", e.raw, formatActual(e, got))
		}
	}

	if !allPass {
		os.Exit(1)
	}
}

// actualValue extracts the value an expectation refers to from the run result
// or memory.
func actualValue(e expectation, rr runResult, ram *memory.RAM) int {
	switch e.kind {
	case "reg":
		switch e.target {
		case "A":
			return int(rr.A)
		case "F":
			return int(rr.F)
		case "B":
			return int(rr.BC >> 8)
		case "C":
			return int(rr.BC & 0xFF)
		case "D":
			return int(rr.DE >> 8)
		case "E":
			return int(rr.DE & 0xFF)
		case "H":
			return int(rr.HL >> 8)
		case "L":
			return int(rr.HL & 0xFF)
		case "AF":
			return int(rr.AF)
		case "BC":
			return int(rr.BC)
		case "DE":
			return int(rr.DE)
		case "HL":
			return int(rr.HL)
		case "IX":
			return int(rr.IX)
		case "IY":
			return int(rr.IY)
		case "SP":
			return int(rr.SP)
		case "PC":
			return int(rr.PC)
		case "AF_":
			return int(rr.AF_)
		case "BC_":
			return int(rr.BC_)
		case "DE_":
			return int(rr.DE_)
		case "HL_":
			return int(rr.HL_)
		}
	case "flag":
		// target is a single flag letter; want is 0 or 1.
		bit := flagBit(e.target)
		if rr.F&bit != 0 {
			return 1
		}
		return 0
	case "mem":
		return int(ram.Read(e.addr))
	}
	return -1
}

func formatActual(e expectation, got int) string {
	if e.kind == "flag" {
		return fmt.Sprintf("%sF=%d", e.target, got)
	}
	return fmt.Sprintf("0x%X", got)
}

// flagBit maps a flag letter to its F-register bit.
func flagBit(letter string) uint8 {
	switch strings.ToUpper(letter) {
	case "S":
		return 0x80
	case "Z":
		return 0x40
	case "H":
		return 0x10
	case "P", "V":
		return 0x04
	case "N":
		return 0x02
	case "C":
		return 0x01
	}
	return 0
}

// parseExpects parses a comma-separated list of assertions:
//   register: A=0x42, HL=0x8000, SP=65000
//   flag:     CF=1, ZF=0  (CF SF ZF HF PF NF; value 0 or 1)
//   memory:   (0xC000)=0x42
var registerNames = map[string]bool{
	"A": true, "F": true, "B": true, "C": true, "D": true, "E": true,
	"H": true, "L": true, "AF": true, "BC": true, "DE": true, "HL": true,
	"IX": true, "IY": true, "SP": true, "PC": true,
	"AF_": true, "BC_": true, "DE_": true, "HL_": true,
}

func parseExpects(s string) ([]expectation, error) {
	var out []expectation
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			return nil, fmt.Errorf("--expect %q must be NAME=VALUE", part)
		}
		lhs := strings.TrimSpace(part[:eq])
		rhs := strings.TrimSpace(part[eq+1:])

		// Memory: (ADDR)=VALUE
		if strings.HasPrefix(lhs, "(") && strings.HasSuffix(lhs, ")") {
			addr, err := parseAddress(lhs[1 : len(lhs)-1])
			if err != nil {
				return nil, fmt.Errorf("--expect memory address: %s", err)
			}
			val, err := parseAddress(rhs)
			if err != nil {
				return nil, fmt.Errorf("--expect memory value: %s", err)
			}
			out = append(out, expectation{raw: part, kind: "mem", addr: addr, want: int(val)})
			continue
		}

		upper := strings.ToUpper(lhs)

		// Flags are written with an F suffix to avoid the C/H ambiguity with
		// register names: CF (carry), ZF (zero), SF (sign), HF (half-carry),
		// PF (parity/overflow), NF (add/subtract). Value is 0 or 1.
		if len(upper) == 2 && upper[1] == 'F' && flagBit(upper[:1]) != 0 {
			val, err := strconv.Atoi(rhs)
			if err != nil || (val != 0 && val != 1) {
				return nil, fmt.Errorf("--expect flag %s must be 0 or 1", upper)
			}
			out = append(out, expectation{raw: part, kind: "flag", target: upper[:1], want: val})
			continue
		}

		if registerNames[upper] {
			val, err := parseAddress(rhs)
			if err != nil {
				return nil, fmt.Errorf("--expect %s value: %s", upper, err)
			}
			out = append(out, expectation{raw: part, kind: "reg", target: upper, want: int(val)})
			continue
		}

		return nil, fmt.Errorf("--expect: unknown target %q (use a register, (addr), or a flag like CF)", lhs)
	}
	return out, nil
}

func handleRun() {
	opts, err := parseRunArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %s\n", opts.inputFile, err)
		os.Exit(1)
	}

	// Assemble in-process.
	asm := assembler.New()
	if opts.next {
		asm.EnableZ80N()
	}
	asm.SetBaseDir(filepath.Dir(opts.inputFile))
	result, asmErr := asm.AssembleString(string(content))
	if asmErr != nil || result == nil || len(result.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Assembly failed; cannot run.")
		if result != nil {
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  Line %d: %s\n", e.Line, e.Message)
			}
		}
		if asmErr != nil && (result == nil || len(result.Errors) == 0) {
			fmt.Fprintf(os.Stderr, "  %s\n", asmErr)
		}
		os.Exit(1)
	}

	callAddr := -1
	if opts.callLabel != "" {
		addr, ok := result.Symbols[opts.callLabel]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: --call label %q not found\n", opts.callLabel)
			os.Exit(1)
		}
		callAddr = int(addr)
	}

	rr, _ := execute(result, opts, callAddr)
	emitRunResult(rr, opts)

	if rr.Outcome == "max_steps" {
		os.Exit(2) // distinct exit code: ran past the instruction cap
	}
}

// execute loads the assembled bytes (and any preloads) into a fresh 64K RAM,
// sets PC to the origin (or to callAddr in subroutine mode), and steps the CPU
// until HALT, the sentinel return, or the instruction cap. It returns the result
// and the RAM, so callers can read memory for assertions.
func execute(result *assembler.AssemblyResult, opts runOptions, callAddr int) (runResult, *memory.RAM) {
	ram := memory.NewRAM()

	// Preloads first, so the assembled program can sit on top of them if the
	// addresses overlap (the program under test usually wins).
	for _, pl := range opts.preloads {
		data, err := os.ReadFile(pl.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot read preload %s: %s\n", pl.path, err)
			os.Exit(1)
		}
		ram.Load(pl.address, data)
	}

	ram.Load(result.Origin, result.MachineCode)

	cpu := z80.New(ram, nil)
	cpu.Reset()
	// Start from a known, zeroed register state so runs are deterministic and
	// assertions are reproducible. Reset() only clears control state, not the
	// main registers, so clear them explicitly here.
	cpu.A, cpu.F = 0, 0
	cpu.B, cpu.C, cpu.D, cpu.E, cpu.H, cpu.L = 0, 0, 0, 0, 0, 0
	cpu.IXH, cpu.IXL, cpu.IYH, cpu.IYL = 0, 0, 0, 0
	cpu.A_, cpu.F_, cpu.B_, cpu.C_, cpu.D_, cpu.E_, cpu.H_, cpu.L_ = 0, 0, 0, 0, 0, 0, 0, 0

	if callAddr >= 0 {
		// Subroutine mode: push the sentinel return address and enter the routine.
		// When the routine RETs, PC becomes the sentinel and the loop stops.
		cpu.SP = 0xFF00
		cpu.SP -= 2
		ram.Write(cpu.SP, uint8(callSentinel&0xFF))
		ram.Write(cpu.SP+1, uint8((callSentinel>>8)&0xFF))
		cpu.PC = uint16(callAddr)
	} else {
		cpu.PC = result.Origin
	}

	rr := runResult{Origin: result.Origin}

	steps := 0
	for {
		if callAddr >= 0 && cpu.PC == callSentinel {
			rr.Outcome = "returned"
			break
		}
		if cpu.Halted {
			rr.Outcome = "halted"
			break
		}
		if steps >= opts.maxSteps {
			rr.Outcome = "max_steps"
			break
		}
		if opts.trace {
			rr.Trace = append(rr.Trace, traceStep{
				Step:   steps,
				PC:     cpu.PC,
				Opcode: ram.Read(cpu.PC),
				A:      cpu.A,
				HL:     cpu.HL(),
			})
		}
		cpu.Step()
		steps++
	}

	rr.Steps = steps
	rr.Cycles = cpu.Cycles
	rr.A, rr.F = cpu.A, cpu.F
	rr.AF = cpu.AF()
	rr.BC, rr.DE, rr.HL = cpu.BC(), cpu.DE(), cpu.HL()
	rr.IX = uint16(cpu.IXH)<<8 | uint16(cpu.IXL)
	rr.IY = uint16(cpu.IYH)<<8 | uint16(cpu.IYL)
	rr.SP, rr.PC = cpu.SP, cpu.PC
	rr.AF_ = uint16(cpu.A_)<<8 | uint16(cpu.F_)
	rr.BC_ = uint16(cpu.B_)<<8 | uint16(cpu.C_)
	rr.DE_ = uint16(cpu.D_)<<8 | uint16(cpu.E_)
	rr.HL_ = uint16(cpu.H_)<<8 | uint16(cpu.L_)
	rr.Flags = formatFlags(cpu.F)
	rr.Halted = cpu.Halted

	if opts.hexDump {
		start := result.Origin
		length := len(result.MachineCode)
		if opts.dumpStart >= 0 {
			start = uint16(opts.dumpStart)
			length = opts.dumpLen
		}
		bytes := make([]uint8, length)
		for i := 0; i < length; i++ {
			bytes[i] = ram.Read(start + uint16(i))
		}
		rr.Memory = &memoryDump{Start: start, Bytes: bytes}
	}

	return rr, ram
}

// formatFlags renders the F register as the conventional SZ5H3PNC string, with a
// dash where a flag is clear.
func formatFlags(f uint8) string {
	names := []struct {
		bit  uint8
		char byte
	}{
		{0x80, 'S'}, {0x40, 'Z'}, {0x20, '5'}, {0x10, 'H'},
		{0x08, '3'}, {0x04, 'P'}, {0x02, 'N'}, {0x01, 'C'},
	}
	var b strings.Builder
	for _, n := range names {
		if f&n.bit != 0 {
			b.WriteByte(n.char)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func emitRunResult(rr runResult, opts runOptions) {
	if opts.jsonLevel != "" {
		emitRunJSON(rr, opts.jsonLevel)
		return
	}

	// Human-readable report.
	if len(rr.Trace) > 0 {
		fmt.Println("Trace:")
		for _, t := range rr.Trace {
			fmt.Printf("  %5d  PC=%04X  op=%02X  A=%02X  HL=%04X\n",
				t.Step, t.PC, t.Opcode, t.A, t.HL)
		}
		fmt.Println()
	}

	switch rr.Outcome {
	case "halted":
		fmt.Printf("Halted after %d instructions (%d cycles).\n", rr.Steps, rr.Cycles)
	case "returned":
		fmt.Printf("Returned after %d instructions (%d cycles).\n", rr.Steps, rr.Cycles)
	case "max_steps":
		fmt.Printf("Stopped: reached instruction cap of %d (possible infinite loop).\n", opts.maxSteps)
	}
	fmt.Printf("  A=%02X  F=%02X [%s]   AF=%04X\n", rr.A, rr.F, rr.Flags, rr.AF)
	fmt.Printf("  BC=%04X  DE=%04X  HL=%04X\n", rr.BC, rr.DE, rr.HL)
	fmt.Printf("  IX=%04X  IY=%04X  SP=%04X  PC=%04X\n", rr.IX, rr.IY, rr.SP, rr.PC)
	fmt.Printf("  AF_=%04X  BC_=%04X  DE_=%04X  HL_=%04X\n", rr.AF_, rr.BC_, rr.DE_, rr.HL_)

	if rr.Memory != nil {
		fmt.Printf("\nMemory @ %04X:\n", rr.Memory.Start)
		fmt.Print(hexDumpRegion(rr.Memory.Start, rr.Memory.Bytes))
	}
}

func emitRunJSON(rr runResult, level string) {
	// Levels trim the structure: basic = outcome + core registers; standard adds
	// flags/cycles; detailed adds the memory window; full keeps the trace too.
	switch level {
	case "basic":
		rr.Trace = nil
		rr.Memory = nil
		rr.Flags = ""
		rr.Cycles = 0
	case "standard":
		rr.Trace = nil
		rr.Memory = nil
	case "detailed":
		rr.Trace = nil
	case "full":
		// keep everything
	}
	out, _ := json.MarshalIndent(rr, "", "  ")
	fmt.Println(string(out))
}

// hexDumpRegion renders bytes as an address-prefixed hex+ascii dump.
func hexDumpRegion(start uint16, data []uint8) string {
	var b strings.Builder
	for i := 0; i < len(data); i += 16 {
		fmt.Fprintf(&b, "  %04X  ", start+uint16(i))
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < i+16; j++ {
			if j < end {
				fmt.Fprintf(&b, "%02X ", data[j])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" ")
		for j := i; j < end; j++ {
			c := data[j]
			if c >= 0x20 && c < 0x7f {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func parseRunArgs(args []string) (runOptions, error) {
	opts := runOptions{
		maxSteps:  defaultMaxSteps,
		dumpStart: -1,
	}
	for _, arg := range args {
		switch {
		case arg == "--trace":
			opts.trace = true
		case arg == "--hex":
			opts.hexDump = true
		case arg == "--next":
			opts.next = true
		case arg == "--json":
			opts.jsonLevel = "standard"
		case strings.HasPrefix(arg, "--json="):
			opts.jsonLevel = strings.TrimPrefix(arg, "--json=")
		case strings.HasPrefix(arg, "--max-steps="):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-steps="))
			if err != nil || n <= 0 {
				return opts, fmt.Errorf("--max-steps requires a positive integer")
			}
			opts.maxSteps = n
		case strings.HasPrefix(arg, "--call="):
			opts.callLabel = strings.TrimPrefix(arg, "--call=")
			if opts.callLabel == "" {
				return opts, fmt.Errorf("--call requires a label name")
			}
		case strings.HasPrefix(arg, "--expect="):
			exps, err := parseExpects(strings.TrimPrefix(arg, "--expect="))
			if err != nil {
				return opts, err
			}
			opts.expects = append(opts.expects, exps...)
		case strings.HasPrefix(arg, "--preload="):
			pl, err := parsePreload(strings.TrimPrefix(arg, "--preload="))
			if err != nil {
				return opts, err
			}
			opts.preloads = append(opts.preloads, pl)
		case strings.HasPrefix(arg, "--dump="):
			start, length, err := parseDumpRange(strings.TrimPrefix(arg, "--dump="))
			if err != nil {
				return opts, err
			}
			opts.dumpStart, opts.dumpLen = start, length
			opts.hexDump = true
		case strings.HasPrefix(arg, "--"):
			return opts, fmt.Errorf("unknown option: %s", arg)
		default:
			if opts.inputFile != "" {
				return opts, fmt.Errorf("unexpected argument: %s", arg)
			}
			opts.inputFile = arg
		}
	}
	if opts.inputFile == "" {
		return opts, fmt.Errorf("no input file given")
	}
	return opts, nil
}

// parsePreload parses "ADDR,PATH" where ADDR may be decimal or 0x/hex.
func parsePreload(s string) (preload, error) {
	comma := strings.IndexByte(s, ',')
	if comma < 0 {
		return preload{}, fmt.Errorf("--preload requires ADDRESS,FILE (got %q)", s)
	}
	addr, err := parseAddress(s[:comma])
	if err != nil {
		return preload{}, fmt.Errorf("--preload address: %s", err)
	}
	path := s[comma+1:]
	if path == "" {
		return preload{}, fmt.Errorf("--preload requires a file path")
	}
	return preload{address: addr, path: path}, nil
}

// parseDumpRange parses "START:LEN" where both may be decimal or 0x/hex.
func parseDumpRange(s string) (int, int, error) {
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return 0, 0, fmt.Errorf("--dump requires START:LEN (got %q)", s)
	}
	start, err := parseAddress(s[:colon])
	if err != nil {
		return 0, 0, fmt.Errorf("--dump start: %s", err)
	}
	length, err := parseAddress(s[colon+1:])
	if err != nil {
		return 0, 0, fmt.Errorf("--dump length: %s", err)
	}
	return int(start), int(length), nil
}

// parseAddress accepts 0xNNNN, NNNNh, $NNNN, or decimal.
func parseAddress(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	var val int64
	var err error
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		val, err = strconv.ParseInt(s[2:], 16, 32)
	case strings.HasPrefix(s, "$"):
		val, err = strconv.ParseInt(s[1:], 16, 32)
	case strings.HasSuffix(s, "h"), strings.HasSuffix(s, "H"):
		val, err = strconv.ParseInt(s[:len(s)-1], 16, 32)
	default:
		val, err = strconv.ParseInt(s, 10, 32)
	}
	if err != nil {
		return 0, fmt.Errorf("invalid address %q", s)
	}
	if val < 0 || val > 0xFFFF {
		return 0, fmt.Errorf("address %q out of 16-bit range", s)
	}
	return uint16(val), nil
}
