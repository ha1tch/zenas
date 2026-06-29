// file: build_cmd.go
//
// The `zenas build` command. Where `assemble` produces a raw binary (or JSON),
// `build` is the packaging step: it assembles the source and then emits one or
// more loadable artifacts — tape images (.tap, .tzx) and snapshots (.sna, .z80)
// — via the shared zentools format library.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zenas/pkg/build"
)

func handleBuild() {
	if len(os.Args) < 3 {
		fmt.Println("Error: No input file specified")
		fmt.Println("Usage: zenas build <input.asm> [--tap] [--tzx] [--sna] [--z80]")
		fmt.Println("                   [--loader] [--start <addr|symbol>] [--sp <addr>]")
		fmt.Println("                   [--model 48k|128k|plus2|plus2a|plus3] [-o <basename>]")
		fmt.Println("  Snapshot formats (.sna/.z80) load and run immediately; best for")
		fmt.Println("  development testing (use .z80 v3 or .sna). Tape formats (.tap/.tzx)")
		fmt.Println("  are the primary distribution format. --loader prepends a BASIC")
		fmt.Println("  auto-run loader to a tape; --start sets the entry point (required")
		fmt.Println("  for snapshots and for --loader).")
		return
	}

	inputFile := os.Args[2]
	var (
		wantTAP, wantTZX, wantSNA, wantZ80 bool
		startSpec                          string
		spSpec                             string
		modelSpec                          = "48k"
		outBase                            string
		haveSP                             bool
		wantLoader                         bool
	)

	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--tap":
			wantTAP = true
		case arg == "--tzx":
			wantTZX = true
		case arg == "--loader":
			wantLoader = true
		case arg == "--sna":
			wantSNA = true
		case arg == "--z80":
			wantZ80 = true
		case strings.HasPrefix(arg, "--start="):
			startSpec = strings.TrimPrefix(arg, "--start=")
		case arg == "--start":
			i++
			if i >= len(os.Args) {
				fail("--start needs an address or symbol")
			}
			startSpec = os.Args[i]
		case strings.HasPrefix(arg, "--sp="):
			spSpec = strings.TrimPrefix(arg, "--sp=")
			haveSP = true
		case arg == "--sp":
			i++
			if i >= len(os.Args) {
				fail("--sp needs an address")
			}
			spSpec = os.Args[i]
			haveSP = true
		case strings.HasPrefix(arg, "--model="):
			modelSpec = strings.TrimPrefix(arg, "--model=")
		case arg == "--model":
			i++
			if i >= len(os.Args) {
				fail("--model needs a value")
			}
			modelSpec = os.Args[i]
		case strings.HasPrefix(arg, "-o="):
			outBase = strings.TrimPrefix(arg, "-o=")
		case arg == "-o":
			i++
			if i >= len(os.Args) {
				fail("-o needs a basename")
			}
			outBase = os.Args[i]
		default:
			fail(fmt.Sprintf("unknown build option %q", arg))
		}
	}

	if !wantTAP && !wantTZX && !wantSNA && !wantZ80 {
		fail("no output format requested (use --tap, --tzx, --sna, and/or --z80)")
	}
	wantSnapshot := wantSNA || wantZ80

	// Assemble.
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fail(fmt.Sprintf("failed to read input file: %v", err))
	}
	asm := assembler.New()
	asm.SetBaseDir(filepath.Dir(inputFile))
	result, err := asm.AssembleString(string(content))
	if err != nil || (result != nil && len(result.Errors) > 0) {
		n := 0
		if result != nil {
			n = len(result.Errors)
		}
		fail(fmt.Sprintf("assembly failed with %d error(s)", n))
	}

	model := build.Model(strings.ToLower(modelSpec))

	// Resolve the entry point. Required for snapshots and for a loader tape
	// (the loader's USR target); ignored for a bare CODE tape.
	var start uint16
	if wantSnapshot || wantLoader {
		if startSpec == "" {
			fail("--start <addr|symbol> is required for snapshot output (--sna/--z80) and for --loader")
		}
		start, err = resolveStart(startSpec, result)
		if err != nil {
			fail(err.Error())
		}
	}

	// Stack pointer: explicit override, else the default.
	sp := build.DefaultSP
	if haveSP {
		v, perr := parseAddr(spSpec)
		if perr != nil {
			fail(fmt.Sprintf("invalid --sp value %q: %v", spSpec, perr))
		}
		sp = v
	}

	req := build.Request{
		Name:   programName(inputFile),
		Code:   result.MachineCode,
		Origin: result.Origin,
		Start:  start,
		SP:     sp,
		Model:  model,
	}

	if wantSnapshot {
		if w := req.SPWarning(); w != "" {
			fmt.Printf("Warning: %s\n", w)
		}
	}

	base := outBase
	if base == "" {
		base = strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
	}

	// Emit each requested format. With --loader, tape formats get a BASIC
	// auto-run loader prepended (so LOAD "" runs the code); without it, a bare
	// CODE block.
	if wantTAP {
		if wantLoader {
			img, e := build.EncodeTAPWithLoader(req)
			writeArtifact(base+".tap", img, e)
		} else {
			writeArtifact(base+".tap", build.EncodeTAP(req), nil)
		}
	}
	if wantTZX {
		var tapImg []byte
		var e error
		if wantLoader {
			tapImg, e = build.EncodeTAPWithLoader(req)
		} else {
			tapImg = build.EncodeTAP(req)
		}
		if e != nil {
			fail(fmt.Sprintf("encoding .tzx loader: %v", e))
		}
		img, te := build.EncodeTZXFromTAP(tapImg)
		writeArtifact(base+".tzx", img, te)
	}
	if wantSNA {
		img, e := build.EncodeSNA(req)
		writeArtifact(base+".sna", img, e)
	}
	if wantZ80 {
		img, e := build.EncodeZ80(req)
		writeArtifact(base+".z80", img, e)
	}
}

// resolveStart turns a --start value into an address. It accepts a numeric
// literal (hex 0x.., $.., or decimal) or a symbol name defined in the source.
func resolveStart(spec string, result *assembler.AssemblyResult) (uint16, error) {
	if addr, err := parseAddr(spec); err == nil {
		return addr, nil
	}
	if result.Symbols != nil {
		if addr, ok := result.Symbols[spec]; ok {
			return addr, nil
		}
		// case-insensitive fall-back, since labels are often upper-cased.
		for name, addr := range result.Symbols {
			if strings.EqualFold(name, spec) {
				return addr, nil
			}
		}
	}
	return 0, fmt.Errorf("--start %q is neither a valid address nor a defined symbol", spec)
}

// parseAddr parses a 16-bit address in hex (0x.., $..), or decimal.
func parseAddr(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	base := 10
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		s = s[2:]
		base = 16
	case strings.HasPrefix(s, "$"):
		s = s[1:]
		base = 16
	}
	v, err := strconv.ParseUint(s, base, 16)
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

// programName derives a tape/snapshot program name (<= 10 chars) from the input
// file name.
func programName(inputFile string) string {
	name := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	if len(name) > 10 {
		name = name[:10]
	}
	return name
}

func writeArtifact(path string, data []byte, encErr error) {
	if encErr != nil {
		fail(fmt.Sprintf("encoding %s: %v", path, encErr))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fail(fmt.Sprintf("writing %s: %v", path, err))
	}
	fmt.Printf("Wrote %s (%d bytes)\n", path, len(data))
}

func fail(msg string) {
	fmt.Printf("Error: %s\n", msg)
	os.Exit(1)
}
