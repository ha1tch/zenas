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
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zenas/pkg/artifact"
	"github.com/ha1tch/zentools/pkg/build"
)

func handleBuild() {
	if len(os.Args) < 3 {
		fmt.Println("Error: No input file specified")
		fmt.Println("Usage: zenas build <input.asm> [--tap] [--tzx] [--pzx] [--sna] [--z80] [--szx] [--nex]")
		fmt.Println("                   [--loader] [--start <addr|symbol>] [--sp <addr>]")
		fmt.Println("                   [--model 48k|128k|plus2|plus2a|plus3] [-o <basename>]")
		fmt.Println("                   [--border <0-7>] [--core <a.b.c>]")
		fmt.Println("                   [--next | --cpu=Z80|Z80N] [--define=NAME[=VAL]] [--tag NAME]")
		fmt.Println("                   [--sym[=path]] [--map[=path]] [--symbols=FILE]")
		fmt.Println("  Snapshot formats (.sna/.z80/.szx) load and run immediately; best for")
		fmt.Println("  development testing (use .z80 v3 or .sna). Tape formats (.tap/.tzx/.pzx)")
		fmt.Println("  are the primary distribution format. --loader prepends a BASIC")
		fmt.Println("  auto-run loader to a tape. --nex writes a Spectrum Next executable:")
		fmt.Println("  the code must lie from 0x4000 up; --border and --core set its border")
		fmt.Println("  colour and the lowest core version it asks for (default 2.0.28).")
		fmt.Println("  --start sets the entry point (required for snapshots, --nex and --loader).")
		fmt.Println("  --next, --define and --tag are as for 'zenas assemble'. --nex turns the")
		fmt.Println("  Z80N instructions on by itself; --cpu=Z80 keeps them off.")
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
		symFile                            string
		symRequested                       bool
		symOpts                            = newSymbolOptions()
		wantSZX, wantPZX, wantNEX          bool
		borderSpec, coreSpec               string
		z80n                               bool
		cpuChosen                          bool
		defines                            = map[string]uint16{}
		tags                               []string
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
		case arg == "--szx":
			wantSZX = true
		case arg == "--pzx":
			wantPZX = true
		case arg == "--nex":
			wantNEX = true
		case strings.HasPrefix(arg, "--border="):
			borderSpec = strings.TrimPrefix(arg, "--border=")
		case arg == "--border":
			i++
			if i >= len(os.Args) {
				fail("--border needs a colour, 0 to 7")
			}
			borderSpec = os.Args[i]
		case strings.HasPrefix(arg, "--core="):
			coreSpec = strings.TrimPrefix(arg, "--core=")
		case arg == "--core":
			i++
			if i >= len(os.Args) {
				fail("--core needs a version, a.b.c")
			}
			coreSpec = os.Args[i]
		case arg == "--next", arg == "--zxnext":
			z80n = true
			cpuChosen = true
		case strings.HasPrefix(arg, "--cpu="):
			cpuChosen = true
			switch strings.ToUpper(strings.TrimPrefix(arg, "--cpu=")) {
			case "Z80":
				z80n = false
			case "Z80N":
				z80n = true
			default:
				fail(fmt.Sprintf("unknown CPU target %q (expected Z80 or Z80N)", strings.TrimPrefix(arg, "--cpu=")))
			}
		case strings.HasPrefix(arg, "--define="):
			name, val, derr := assembler.ParseDefine(strings.TrimPrefix(arg, "--define="))
			if derr != nil {
				fail(derr.Error())
			}
			defines[name] = val
		case arg == "--tag", strings.HasPrefix(arg, "--tag="):
			tag := strings.TrimPrefix(arg, "--tag=")
			if arg == "--tag" {
				i++
				if i >= len(os.Args) {
					fail("--tag requires a tag name")
				}
				tag = os.Args[i]
			}
			if _, terr := assembler.TagSymbol(tag); terr != nil {
				fail(terr.Error())
			}
			tags = append(tags, strings.TrimSpace(tag))
		case arg == "--sym":
			symRequested = true
		case strings.HasPrefix(arg, "--sym="):
			symFile, symRequested = strings.TrimPrefix(arg, "--sym="), true
		default:
			if handled, err := symOpts.parse(arg); handled {
				if err != nil {
					fail(err.Error())
				}
				continue
			}
			fail(fmt.Sprintf("unknown build option %q", arg))
		}
	}

	if !wantTAP && !wantTZX && !wantPZX && !wantSNA && !wantZ80 && !wantSZX && !wantNEX {
		fail("no output format requested (use --tap, --tzx, --pzx, --sna, --z80, --szx and/or --nex)")
	}
	wantSnapshot := wantSNA || wantZ80 || wantSZX
	// A .nex runs on a Next only, and a Next's CPU is a Z80N: --nex turns
	// the Z80N instructions on unless the command line chose a CPU.
	if wantNEX && !cpuChosen {
		z80n = true
	}
	tagDefines, err := assembler.TagDefines(tags)
	if err != nil {
		fail(err.Error())
	}
	for name, val := range tagDefines {
		defines[name] = val
	}
	nexOpts, err := artifact.ParseNEXOptions(borderSpec, coreSpec)
	if err != nil {
		fail("--" + err.Error())
	}

	// Assemble.
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fail(fmt.Sprintf("failed to read input file: %v", err))
	}
	asm := assembler.New()
	if z80n {
		asm.EnableZ80N()
	}
	for name, val := range defines {
		asm.Define(name, val)
	}
	asm.SetBaseDir(filepath.Dir(inputFile))
	if _, ierr := symOpts.importSymbols(asm); ierr != nil {
		fail(fmt.Sprintf("--symbols: %v", ierr))
	}
	result, err := asm.AssembleString(string(content))
	if err != nil || (result != nil && len(result.Errors) > 0) {
		n := 0
		if result != nil {
			n = len(result.Errors)
			for _, e := range result.Errors {
				fmt.Printf("  Line %d: %s\n", e.Line, e.Message)
			}
		}
		if n == 0 && err != nil {
			fail(fmt.Sprintf("assembly failed: %v", err))
		}
		fail(fmt.Sprintf("assembly failed with %d error(s)", n))
	}

	// What the files need besides the code. The entry point is required
	// for snapshots, for a NEX file and for a loader tape (the loader's
	// USR target); a bare CODE tape has none.
	opts := artifact.Options{
		Name:   programName(inputFile),
		Model:  build.Model(strings.ToLower(modelSpec)),
		Loader: wantLoader,
		NEX:    nexOpts,
	}
	if wantSnapshot || wantLoader || wantNEX {
		if startSpec == "" {
			fail("--start <addr|symbol> is required for snapshot output (--sna/--z80/--szx), for --nex and for --loader")
		}
		if _, err := artifact.ResolveStart(startSpec, result); err != nil {
			fail("--" + err.Error())
		}
		opts.Start = startSpec
	}
	if haveSP {
		v, perr := artifact.ParseAddress(spSpec)
		if perr != nil {
			fail(fmt.Sprintf("invalid --sp value %q: %v", spSpec, perr))
		}
		opts.SP = &v
	}

	if wantSnapshot || wantNEX {
		req, rerr := artifact.Request(result, opts, true)
		if rerr != nil {
			fail(rerr.Error())
		}
		if w := req.SPWarning(); w != "" {
			fmt.Printf("Warning: %s\n", w)
		}
	}

	base := outBase
	if base == "" {
		base = strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
	}

	// Symbol files, beside the other outputs.
	if symRequested {
		if symFile == "" {
			symFile = base + ".sym"
		}
		if err := writeSymFile(symFile, result); err != nil {
			fail(fmt.Sprintf("failed to write symbol file: %v", err))
		}
		fmt.Printf("Wrote %s\n", symFile)
	}
	if mapFile := symOpts.mapPath(base); mapFile != "" {
		if err := writeMapFile(mapFile, result, symOpts.paging); err != nil {
			fail(fmt.Sprintf("failed to write map file: %v", err))
		}
		fmt.Printf("Wrote %s\n", mapFile)
	}

	// Emit each requested format. With --loader, tape formats get a BASIC
	// auto-run loader prepended (so LOAD "" runs the code); without it, a bare
	// CODE block.
	want := map[artifact.Format]bool{
		artifact.TAP: wantTAP, artifact.TZX: wantTZX, artifact.PZX: wantPZX,
		artifact.SNA: wantSNA, artifact.Z80: wantZ80, artifact.SZX: wantSZX, artifact.NEX: wantNEX,
	}
	for _, format := range artifact.Formats() {
		if !want[format] {
			continue
		}
		img, e := artifact.Encode(result, format, opts)
		writeArtifact(base+format.Ext(), img, e)
	}
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
