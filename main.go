// Z80 Assembler Command-Line Tool
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	
	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zenas/pkg/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	
	switch os.Args[1] {
	case "assemble", "asm":
		handleAssemble()
	case "build":
		handleBuild()
	case "run":
		handleRun()
	case "assert":
		handleAssert()
	case "test":
		handleTest()
	case "help", "-h", "--help":
		if len(os.Args) > 2 && (os.Args[2] == "--all" || os.Args[2] == "--full" || os.Args[2] == "-a") {
			printUsageFull()
		} else {
			printUsage()
		}
	case "version", "-v", "--version":
		fmt.Printf("zenas %s\n", version.Version)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
	}
}

func printUsage() {
	fmt.Printf("zenas %s - a Z80 / Z80N assembler\n", version.Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  zenas assemble <input.asm> [output.bin] [options]")
	fmt.Println("  zenas build <input.asm> [--tap] [--tzx] [--sna] [--z80] [--loader] [options]")
	fmt.Println("  zenas version")
	fmt.Println("  zenas help [--all]")
	fmt.Println()
	fmt.Println("Common options:")
	fmt.Println("  --next                Enable the Z80N (ZX Spectrum Next) instruction set")
	fmt.Println("  --define=NAME[=VAL]   Pre-define a symbol; drives IF/IFDEF")
	fmt.Println("  --tag NAME            Select a build tag (defines ZENAS_TAG_NAME)")
	fmt.Println("  --sym[=path]          Write a pasmo-format symbol file")
	fmt.Println("  --hex                 Always show a hex dump of the output")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  zenas assemble game.asm game.bin")
	fmt.Println("  zenas assemble game.asm --next")
	fmt.Println("  zenas assemble game.asm out.bin --tag debug --tag plus3")
	fmt.Println()
	fmt.Println("Run 'zenas help --all' for the full option, charset and output reference.")
}

// printUsageFull prints the complete reference: every option, the character
// sets, and the JSON output levels. Shown by 'zenas help --all'.
func printUsageFull() {
	fmt.Printf("zenas %s - a Z80 / Z80N assembler\n", version.Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  zenas assemble <input.asm> [output.bin] [options]")
	fmt.Println("  zenas version          Show the version (also -v, --version)")
	fmt.Println("  zenas test             Run the built-in self-tests")
	fmt.Println("  zenas help [--all]     Show help; --all for this full reference")
	fmt.Println()
	fmt.Println("Output options:")
	fmt.Println("  output.bin            Output file (default: input name with .bin)")
	fmt.Println("  --hex                 Always show a hex dump of the output")
	fmt.Println("  --no-hex              Never show a hex dump")
	fmt.Println("  --sym[=path]          Write a pasmo-format symbol file (default: <output>.sym)")
	fmt.Println("  --json=LEVEL          Emit a JSON report instead of a binary (see below)")
	fmt.Println()
	fmt.Println("Assembly options:")
	fmt.Println("  --next                Enable the Z80N (ZX Spectrum Next) instruction set")
	fmt.Println("  --cpu=Z80|Z80N        Select CPU target (default Z80); --cpu=Z80N == --next")
	fmt.Println("  --define=NAME[=VAL]   Pre-define a symbol (default value 1); drives IF/IFDEF")
	fmt.Println("  --tag NAME            Select a build tag. Defines ZENAS_TAG_NAME and")
	fmt.Println("                        ZENAS_TAGBIT_NAME, and sets a bit in ZENAS_TAGS.")
	fmt.Println("  --charset=NAME        Character encoding for strings (see below)")
	fmt.Println("  --no-warnings         Suppress character-replacement warnings")
	fmt.Println()
	fmt.Println("Character sets (--charset=NAME):")
	fmt.Println("  ascii                 Standard ASCII (default)")
	fmt.Println("  spectrum-uk           ZX Spectrum (UK)")
	fmt.Println("  spectrum-tk90x        Brazilian TK90X/TK95")
	fmt.Println("  spectrum-inves        Spanish Investronica Spectrum")
	fmt.Println("  spectrum-czech        Czech/Slovak Didaktik")
	fmt.Println("  spectrum-polish       Polish Spectrum clones")
	fmt.Println("  msx-jp                MSX (Japanese Katakana)")
	fmt.Println("  msx-eu                MSX (European)")
	fmt.Println("  cpc-uk                Amstrad CPC (UK)")
	fmt.Println("  cpc-fr                Amstrad CPC (French)")
	fmt.Println("  When a character is missing from the target set it is replaced with the")
	fmt.Println("  closest available one (for example an accented letter to its base letter)")
	fmt.Println("  and a warning is issued unless --no-warnings is given.")
	fmt.Println()
	fmt.Println("JSON output levels (--json=LEVEL):")
	fmt.Println("  basic                 Machine code and symbols only")
	fmt.Println("  standard              Add errors and assembly info")
	fmt.Println("  detailed              Include an instruction breakdown")
	fmt.Println("  full                  Complete metadata with timing")
	fmt.Println()
	fmt.Println("See docs/MANUAL.md for the full language and directive reference.")
}

func handleAssemble() {
	if len(os.Args) < 3 {
		fmt.Println("Error: No input file specified")
		fmt.Println("Usage: zenas assemble <input.asm> [output.bin] [options]  (see 'zenas help --all')")
		return
	}
	
	inputFile := os.Args[2]
	outputFile := ""
	jsonLevel := ""
	hexMode := "auto" // auto, force, never
	charset := "ascii" // default charset
	showWarnings := true
	symFile := ""     // path for the symbol file; "" means none
	symRequested := false
	defines := map[string]uint16{}
	tags := []string{} // tag names in command-line order, for the ZENAS_TAGS bitmask
	cpuTarget := "Z80" // CPU target: Z80 (default) or Z80N
	
	// Parse remaining arguments
	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--json=") {
			jsonLevel = strings.TrimPrefix(arg, "--json=")
		} else if arg == "--hex" {
			hexMode = "force"
		} else if arg == "--no-hex" {
			hexMode = "never"
		} else if strings.HasPrefix(arg, "--charset=") {
			charset = strings.TrimPrefix(arg, "--charset=")
		} else if arg == "--no-warnings" {
			showWarnings = false
		} else if strings.HasPrefix(arg, "--sym=") {
			symFile = strings.TrimPrefix(arg, "--sym=")
			symRequested = true
		} else if arg == "--sym" {
			symRequested = true // path defaulted below from the output name
		} else if arg == "--next" {
			cpuTarget = "Z80N"
		} else if strings.HasPrefix(arg, "--cpu=") {
			cpuTarget = strings.ToUpper(strings.TrimPrefix(arg, "--cpu="))
			if cpuTarget != "Z80" && cpuTarget != "Z80N" {
				fmt.Printf("Error: unknown CPU target '%s' (expected Z80 or Z80N)\n", cpuTarget)
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "--define=") {
			name, val, err := parseDefine(strings.TrimPrefix(arg, "--define="))
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defines[name] = val
		} else if arg == "--tag" || strings.HasPrefix(arg, "--tag=") {
			// --tag <name> (or --tag=<name>) selects a build tag, the way Go build
			// tags select variants. Tags are collected in order and turned into
			// defines after parsing: ZENAS_TAG_<name>=1 (presence), a per-tag bit
			// constant ZENAS_TAGBIT_<name>, and a composite bitmask ZENAS_TAGS.
			var tag string
			if arg == "--tag" {
				if i+1 >= len(os.Args) {
					fmt.Println("Error: --tag requires a tag name")
					os.Exit(1)
				}
				i++
				tag = os.Args[i]
			} else {
				tag = strings.TrimPrefix(arg, "--tag=")
			}
			if _, err := tagSymbol(tag); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			tags = append(tags, strings.TrimSpace(tag))
		} else if outputFile == "" {
			outputFile = arg
		}
	}
	
	// Expand the collected build tags into symbols. Each distinct tag, in the
	// order it first appears, gets the next bit. A tag named NAME produces:
	//   ZENAS_TAG_NAME    = 1            (presence; for IFDEF and AND/OR/NOT)
	//   ZENAS_TAGBIT_NAME = 1 << bit     (this tag's bit, for order-independent masks)
	// and the composite ZENAS_TAGS is the OR of all set tags' bits. Symbols are
	// 16-bit, so at most 16 distinct tags can occupy bits. ZENAS_TAGS is always
	// defined (0 when no tags are set) so source that references it compiles in
	// every configuration.
	var mask uint16
	bit := 0
	seen := map[string]bool{}
	for _, tag := range tags {
		sym, _ := tagSymbol(tag) // already validated during parsing
		defines[sym] = 1
		if seen[tag] {
			continue
		}
		seen[tag] = true
		if bit >= 16 {
			fmt.Println("Error: too many distinct --tag values (maximum 16 occupy bits in ZENAS_TAGS)")
			os.Exit(1)
		}
		bitVal := uint16(1) << uint(bit)
		defines["ZENAS_TAGBIT_"+tag] = bitVal
		mask |= bitVal
		bit++
	}
	defines["ZENAS_TAGS"] = mask

	// Generate output filename if not specified
	if outputFile == "" && jsonLevel == "" {
		ext := filepath.Ext(inputFile)
		outputFile = strings.TrimSuffix(inputFile, ext) + ".bin"
	}

	// Default the symbol-file path to the output basename + .sym when --sym was
	// given without an explicit path.
	if symRequested && symFile == "" {
		ext := filepath.Ext(outputFile)
		symFile = strings.TrimSuffix(outputFile, ext) + ".sym"
	}
	
	// Handle JSON output
	if jsonLevel != "" {
		err := assembleToJSON(inputFile, jsonLevel, charset, showWarnings)
		if err != nil {
			fmt.Printf("Assembly failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	
	// Normal binary output
	err := assembleFile(inputFile, outputFile, symFile, hexMode, charset, showWarnings, defines, cpuTarget == "Z80N")
	if err != nil {
		fmt.Printf("Assembly failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Assembly successful: %s -> %s (charset: %s)\n", inputFile, outputFile, charset)
}

func handleTest() {
	fmt.Println("Running Z80 Assembler Tests...")
	fmt.Println()
	
	tests := []struct {
		name    string
		source  string
		expect  []uint8
		wantErr bool // when true, the test passes if assembly reports an error
	}{
		{
			name:   "Simple LD instructions",
			source: "LD A, B\nLD C, 42\n",
			expect: []uint8{0x78, 0x0E, 0x2A},
		},
		{
			name:   "ADD and arithmetic",
			source: "ADD A, C\nSUB B\nINC A\n",
			expect: []uint8{0x81, 0x90, 0x3C},
		},
		{
			name:   "Jump instructions",
			source: "JP $1234\nJR 10\nCALL $ABCD\n",
			expect: []uint8{0xC3, 0x34, 0x12, 0x18, 0x05, 0xCD, 0xCD, 0xAB}, // JR 10 = abs addr 10, disp 0x05 (matches pasmo)
		},
		{
			name:   "Labels and constants",
			source: ".ORG $8000\nSTART:\n  LD A, 42\n  JP START\n",
			expect: []uint8{0x3E, 0x2A, 0xC3, 0x00, 0x80},
		},
		{
			name:   "Data directives",
			source: ".DB $FF, 42\n.DW $1234\n",
			expect: []uint8{0xFF, 0x2A, 0x34, 0x12},
		},
		{
			name:   "CB prefix instructions",
			source: "RLC A\nBIT 7, B\nSET 3, C\n",
			expect: []uint8{0xCB, 0x07, 0xCB, 0x78, 0xCB, 0xD9},
		},
		{
			name:   "Character replacement test",
			source: ".DB \"José\", 0\n",
			expect: []uint8{'J', 'o', 's', '?', 0}, // ascii charset replaces 'é' with '?'
		},
		{
			name:   "Macro: single argument",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO ONE(val)\n  LD A,val\nENDMACRO\n  ONE(5)\n",
			expect: []uint8{0x3E, 0x05}, // LD A,5
		},
		{
			name:   "Macro: multiple arguments",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO TWO(x,y)\n  LD A,x\n  LD B,y\nENDMACRO\n  TWO(5,6)\n",
			expect: []uint8{0x3E, 0x05, 0x06, 0x06}, // LD A,5 / LD B,6
		},
		{
			name:   "Macro: zero arguments",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO Z()\n  NOP\nENDMACRO\n  Z()\n",
			expect: []uint8{0x00}, // NOP
		},
		{
			name:   "Macro: local label unique per expansion",
			source: ".MACRO_STYLE TRADITIONAL\n.ORG $8000\nMACRO DELAY(n)\n  LD B,n\nLP:\n  DJNZ LP\nENDMACRO\n  DELAY(5)\n  DELAY(7)\n",
			// LD B,5 / DJNZ -2, then LD B,7 / DJNZ -2 - each LP resolves independently
			expect: []uint8{0x06, 0x05, 0x10, 0xFE, 0x06, 0x07, 0x10, 0xFE},
		},
		{
			name:   "Macro: nested call with multiple arguments",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO INNER(aa,bb)\n  LD A,aa\n  ADD A,bb\nENDMACRO\nMACRO OUTER(vv)\n  INNER(vv,3)\n  INC A\nENDMACRO\n  OUTER(5)\n",
			// OUTER(5) -> INNER(5,3) -> LD A,5 / ADD A,3, then INC A
			expect: []uint8{0x3E, 0x05, 0xC6, 0x03, 0x3C},
		},
		{
			name:   "Macro: nested call with zero arguments",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO PLAIN()\n  NOP\nENDMACRO\nMACRO WRAP()\n  PLAIN()\n  PLAIN()\nENDMACRO\n  WRAP()\n",
			expect: []uint8{0x00, 0x00}, // two NOPs
		},
		{
			name:   "Macro: width marker matches (uint8, byte arg)",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO M(uint8_t v)\n  LD A,v\nENDMACRO\n  M(5)\n",
			expect: []uint8{0x3E, 0x05}, // LD A,5
		},
		{
			name:    "Macro: width mismatch is an error (uint16 param, 8-bit arg)",
			source:  ".MACRO_STYLE TRADITIONAL\nMACRO M(uint16_t v)\n  LD HL,v\nENDMACRO\n  M(5)\n",
			wantErr: true, // 8-bit argument into a 16-bit signature must be rejected
		},
		{
			name:   "Macro: untyped param is not width-checked",
			source: ".MACRO_STYLE TRADITIONAL\nMACRO M(v)\n  LD HL,v\nENDMACRO\n  M(5)\n",
			expect: []uint8{0x21, 0x05, 0x00}, // LD HL,5 - allowed, no width contract
		},
		{
			name:   "C-style: function and call on a single line",
			source: ".MACRO_STYLE C\nvoid f() { asm { LD A,1; } }\nvoid main() { f(); }\n.END\n",
			expect: []uint8{0x3E, 0x01}, // LD A,1 - semicolon is a terminator inside braces
		},
		{
			name:   "C-style: typed return places value in A, no RET when inlined",
			source: ".MACRO_STYLE C\nuint8_t g() { asm { LD A,5; } return 42; }\nvoid main() { g(); }\n.END\n",
			expect: []uint8{0x3E, 0x05, 0x3E, 0x2A}, // LD A,5 / LD A,42 (return literal); no RET
		},
		{
			name:    "C-style: return width mismatch is an error",
			source:  ".MACRO_STYLE C\nuint8_t g() { asm { LD A,5; } return 300; }\nvoid main() { g(); }\n.END\n",
			wantErr: true, // 16-bit value returned from an 8-bit function
		},
		{
			name:    "C-style: typed function without return is an error",
			source:  ".MACRO_STYLE C\nuint8_t g() { asm { LD A,5; } }\nvoid main() { g(); }\n.END\n",
			wantErr: true, // declares uint8_t but never returns
		},
		{
			name:    "C-style: value returned from void is an error",
			source:  ".MACRO_STYLE C\nvoid g() { asm { NOP; } return 5; }\nvoid main() { g(); }\n.END\n",
			wantErr: true,
		},
		{
			name:   "C-style: void bare return emits no RET when inlined",
			source: ".MACRO_STYLE C\nvoid g() { asm { NOP; } return; }\nvoid main() { g(); }\n.END\n",
			expect: []uint8{0x00}, // NOP; bare return emits nothing (inlined)
		},
		{
			name:   "Package: qualified calls disambiguate same-named macros",
			source: ".MACRO_STYLE TRADITIONAL\n.PACKAGE math\nMACRO add(aa, bb)\n  LD A, aa\n  ADD A, bb\nENDMACRO\n.PACKAGE counter\nMACRO add(nn)\n  LD A, nn\n  INC A\nENDMACRO\n.PACKAGE main\n  ORG $8000\n  math.add(10, 5)\n  counter.add(7)\n",
			expect: []uint8{0x3E, 0x0A, 0xC6, 0x05, 0x3E, 0x07, 0x3C}, // math then counter
		},
		{
			name:   "Package: qualified macro coexists with same-named instruction",
			source: ".MACRO_STYLE TRADITIONAL\n.PACKAGE math\nMACRO add(aa, bb)\n  LD A, aa\n  ADD A, bb\nENDMACRO\n.PACKAGE main\n  ORG $8000\n  math.add(10, 20)\n  ADD A, 5\n",
			expect: []uint8{0x3E, 0x0A, 0xC6, 0x14, 0xC6, 0x05}, // macro, then ADD instruction
		},
		{
			name:    "Package: ambiguous bare call is an error",
			source:  ".MACRO_STYLE TRADITIONAL\n.PACKAGE math\nMACRO twice(nn)\n  LD A, nn\nENDMACRO\n.PACKAGE counter\nMACRO twice(nn)\n  LD B, nn\nENDMACRO\n.PACKAGE main\n  ORG $8000\n  twice(5)\n",
			wantErr: true, // defined in two packages; must qualify
		},
		{
			name:   "Package: bare call resolves when unambiguous",
			source: ".MACRO_STYLE TRADITIONAL\n.PACKAGE math\nMACRO quintuple(nn)\n  LD A, nn\nENDMACRO\n.PACKAGE main\n  ORG $8000\n  quintuple(9)\n",
			expect: []uint8{0x3E, 0x09}, // LD A,9
		},
		{
			name:   "Pasmo dialect: $ is the location counter",
			source: ".pasmo\n  ORG $8000\n  DW $\n  DW $\n.zenas\n",
			expect: []uint8{0x00, 0x80, 0x02, 0x80}, // $ at 8000 and 8002
		},
		{
			name:   "Pasmo dialect: DEFM string, then native resumes",
			source: ".pasmo\n  ORG $8000\n  DEFM \"Hi\"\n.zenas\n  LD HL, $1234\n",
			expect: []uint8{0x48, 0x69, 0x21, 0x34, 0x12}, // "Hi" then LD HL,$1234 (native hex)
		},
		{
			name:   "Char literal arithmetic in expressions",
			source: "        ORG $8000\n        DEFB 'A'+1, 'Z'-'A', 'A'\n",
			expect: []uint8{0x42, 0x19, 0x41}, // 'A'+1=0x42, 'Z'-'A'=0x19, 'A'=0x41
		},
		{
			name:   "Pasmo dialect: column-1 labels without colons",
			source: ".pasmo\n        ORG $8000\nloop    NOP\n        DJNZ loop\ndone    NOP\n.zenas\n",
			expect: []uint8{0x00, 0x10, 0xFD, 0x00}, // loop:NOP / DJNZ loop / done:NOP (matches pasmo)
		},
		{
			name:   "Pasmo dialect: column-1 mnemonic stays an instruction",
			source: ".pasmo\n        ORG $8000\nADD A, 5\n.zenas\n",
			expect: []uint8{0xC6, 0x05}, // ADD in column 1 is the instruction, not a label
		},
		{
			name:   "Pasmo dialect: # hex prefix",
			source: ".pasmo\n        ORG $8000\n        LD A, #80\n        LD B, #FF\n.zenas\n",
			expect: []uint8{0x3E, 0x80, 0x06, 0xFF}, // #80 == 0x80, #FF == 0xFF
		},
		{
			name:   "Hexdump: glued 0x in DB (bytes, written order)",
			source: "        ORG 0\n        DB 0xDEADBEEF\n",
			expect: []uint8{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:   "Hexdump: glued 0x in DW (little-endian words)",
			source: "        ORG 0\n        DW 0xDEADBEEF\n",
			expect: []uint8{0xAD, 0xDE, 0xEF, 0xBE},
		},
		{
			name:   "Hexdump: spaced 0x in DB",
			source: "        ORG 0\n        DB 0x DE AD BE EF\n",
			expect: []uint8{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:   "Hexdump: spaced 0d (decimal) equals string in DEFM",
			source: "        ORG 0\n        DEFM 0d 66 65 66 69\n",
			expect: []uint8{0x42, 0x41, 0x42, 0x45}, // "BABE"
		},
	}
	
	passed := 0
	total := len(tests)
	
	for i, test := range tests {
		fmt.Printf("Test %d: %s\n", i+1, test.name)
		
		asm := assembler.New()
		result, err := asm.AssembleString(test.source)
		
		// Tests that expect an error pass when assembly reports one.
		if test.wantErr {
			if err != nil || (result != nil && len(result.Errors) > 0) {
				fmt.Printf("  PASS: Passed (expected error)\n")
				passed++
			} else {
				fmt.Printf("  FAIL: expected an error but assembly succeeded\n")
			}
			continue
		}
		
		if err != nil {
			fmt.Printf("  FAIL: Assembly failed: %v\n", err)
			continue
		}
		
		if len(result.Errors) > 0 {
			fmt.Printf("  FAIL: Assembly errors:\n")
			for _, e := range result.Errors {
				fmt.Printf("    Line %d: %s\n", e.Line, e.Message)
			}
			continue
		}
		
		if !bytesEqual(result.MachineCode, test.expect) {
			fmt.Printf("  FAIL: Output mismatch\n")
			fmt.Printf("    Expected: %s\n", formatBytes(test.expect))
			fmt.Printf("    Got:      %s\n", formatBytes(result.MachineCode))
			continue
		}
		
		// Show warnings if any
		if len(result.Warnings) > 0 {
			fmt.Printf("  WARN: Warnings:\n")
			for _, w := range result.Warnings {
				fmt.Printf("    Line %d: %s\n", w.Line, w.Message)
			}
		}
		
		fmt.Printf("  PASS: Passed\n")
		passed++
	}
	
	fmt.Printf("\nTest Results: %d/%d passed\n", passed, total)
	
	if passed == total {
		fmt.Println("All tests passed!")
	} else {
		fmt.Printf("%d test(s) failed\n", total-passed)
		os.Exit(1)
	}
}

// JSON output structures for different detail levels

type BasicJSON struct {
	Success     bool              `json:"success"`
	MachineCode []string          `json:"machineCode"`
	ByteCount   int               `json:"byteCount"`
	Symbols     map[string]uint16 `json:"symbols"`
	Warnings    []WarningJSON     `json:"warnings,omitempty"`
}

type StandardJSON struct {
	BasicJSON
	InputFile   string                  `json:"inputFile"`
	Errors      []assembler.AssemblyError `json:"errors,omitempty"`
	Timestamp   string                  `json:"timestamp"`
	Assembler   string                  `json:"assembler"`
	Version     string                  `json:"version"`
}

type WarningJSON struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type InstructionInfo struct {
	Address     uint16   `json:"address"`
	Bytes       []string `json:"bytes"`
	Source      string   `json:"source"`
	Mnemonic    string   `json:"mnemonic,omitempty"`
	Operands    []string `json:"operands,omitempty"`
	ByteCount   int      `json:"byteCount"`
}

type DetailedJSON struct {
	StandardJSON
	Instructions []InstructionInfo `json:"instructions"`
	DataAreas    []DataArea        `json:"dataAreas"`
	CodeSize     int               `json:"codeSize"`
	DataSize     int               `json:"dataSize"`
}

type DataArea struct {
	StartAddress uint16   `json:"startAddress"`
	EndAddress   uint16   `json:"endAddress"`
	Size         int      `json:"size"`
	Type         string   `json:"type"` // "code", "data", "string"
	Content      []string `json:"content,omitempty"`
}

type CycleInfo struct {
	MinCycles int `json:"minCycles"`
	MaxCycles int `json:"maxCycles"`
	Note      string `json:"note,omitempty"`
}

type FullJSON struct {
	DetailedJSON
	Performance struct {
		TotalCycles  CycleInfo `json:"totalCycles"`
		Instructions int       `json:"instructionCount"`
		Efficiency   float64   `json:"efficiency"` // bytes per instruction
	} `json:"performance"`
	Metadata struct {
		EntryPoint  uint16 `json:"entryPoint,omitempty"`
		CodeBlocks  int    `json:"codeBlocks"`
		DataBlocks  int    `json:"dataBlocks"`
		LabelCount  int    `json:"labelCount"`
		ConstCount  int    `json:"constantCount"`
	} `json:"metadata"`
	BuildInfo struct {
		Host        string `json:"host"`
		BuildTime   string `json:"buildTime"`
		SourceLines int    `json:"sourceLines"`
		SourceSize  int    `json:"sourceSize"`
	} `json:"buildInfo"`
}

func assembleToJSON(inputFile, level, charset string, showWarnings bool) error {
	// Validate JSON level
	validLevels := map[string]bool{
		"basic": true, "standard": true, "detailed": true, "full": true,
	}
	if !validLevels[level] {
		return fmt.Errorf("invalid JSON level: %s (must be basic, standard, detailed, or full)", level)
	}
	
	// Read input file
	content, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}
	
	// Assemble
	asm := assembler.New()
	
	// Set character set
	if err := asm.SetCharset(charset); err != nil {
		return fmt.Errorf("invalid charset '%s': %v", charset, err)
	}
	
	asm.SetBaseDir(filepath.Dir(inputFile))
	result, err := asm.AssembleString(string(content))
	
	// Create appropriate JSON structure based on level
	switch level {
	case "basic":
		jsonData := createBasicJSON(result, err, showWarnings)
		return outputJSON(jsonData)
		
	case "standard":
		jsonData := createStandardJSON(result, err, inputFile, showWarnings)
		return outputJSON(jsonData)
		
	case "detailed":
		jsonData := createDetailedJSON(result, err, inputFile, string(content), showWarnings)
		return outputJSON(jsonData)
		
	case "full":
		jsonData := createFullJSON(result, err, inputFile, string(content), showWarnings)
		return outputJSON(jsonData)
	}
	
	return nil
}

func createBasicJSON(result *assembler.AssemblyResult, err error, showWarnings bool) BasicJSON {
	success := err == nil && (result == nil || len(result.Errors) == 0)
	
	var machineCode []string
	var symbols map[string]uint16
	var warnings []WarningJSON
	
	if result != nil {
		// Convert bytes to hex strings
		for _, b := range result.MachineCode {
			machineCode = append(machineCode, fmt.Sprintf("%02X", b))
		}
		symbols = result.Symbols
		
		// Include warnings if requested
		if showWarnings {
			for _, w := range result.Warnings {
				warnings = append(warnings, WarningJSON{
					Line:    w.Line,
					Message: w.Message,
					Type:    warningTypeToString(w.Type),
				})
			}
		}
	}
	
	return BasicJSON{
		Success:     success,
		MachineCode: machineCode,
		ByteCount:   len(machineCode),
		Symbols:     symbols,
		Warnings:    warnings,
	}
}

func createStandardJSON(result *assembler.AssemblyResult, err error, inputFile string, showWarnings bool) StandardJSON {
	basic := createBasicJSON(result, err, showWarnings)
	
	standard := StandardJSON{
		BasicJSON: basic,
		InputFile: inputFile,
		Timestamp: "2024-09-16T18:30:00Z", // You'd use time.Now() here
		Assembler: "zenas",
		Version:   version.Version,
	}
	
	if result != nil {
		standard.Errors = result.Errors
	}
	
	return standard
}

func createDetailedJSON(result *assembler.AssemblyResult, err error, inputFile, source string, showWarnings bool) DetailedJSON {
	standard := createStandardJSON(result, err, inputFile, showWarnings)
	
	detailed := DetailedJSON{
		StandardJSON: standard,
		Instructions: []InstructionInfo{},
		DataAreas:    []DataArea{},
		CodeSize:     0,
		DataSize:     0,
	}
	
	if result != nil && len(result.MachineCode) > 0 {
		// Analyze the assembled code (simplified analysis)
		detailed.Instructions = analyzeInstructions(result.MachineCode, source)
		detailed.DataAreas = analyzeDataAreas(result.MachineCode)
		detailed.CodeSize = len(result.MachineCode)
	}
	
	return detailed
}

func createFullJSON(result *assembler.AssemblyResult, err error, inputFile, source string, showWarnings bool) FullJSON {
	detailed := createDetailedJSON(result, err, inputFile, source, showWarnings)
	
	full := FullJSON{
		DetailedJSON: detailed,
	}
	
	if result != nil {
		// Calculate performance metrics
		full.Performance.Instructions = len(detailed.Instructions)
		if full.Performance.Instructions > 0 {
			full.Performance.Efficiency = float64(detailed.CodeSize) / float64(full.Performance.Instructions)
		}
		full.Performance.TotalCycles = estimateCycles(detailed.Instructions)
		
		// Calculate metadata
		full.Metadata.LabelCount = countLabels(result.Symbols)
		full.Metadata.ConstCount = countConstants(result.Symbols)
		full.Metadata.CodeBlocks = len(detailed.DataAreas)
		
		// Build info
		full.BuildInfo.Host = "zenas-assembler"
		full.BuildInfo.BuildTime = "2024-09-16T18:30:00Z"
		full.BuildInfo.SourceLines = strings.Count(source, "\n") + 1
		full.BuildInfo.SourceSize = len(source)
	}
	
	return full
}

func warningTypeToString(wt assembler.WarningType) string {
	// Convert warning type enum to string
	// This requires knowing the assembler package's WarningType values
	return "character_replacement" // Simplified for now
}

// Helper functions for analysis

func analyzeInstructions(machineCode []uint8, source string) []InstructionInfo {
	// Simplified instruction analysis
	instructions := []InstructionInfo{}
	
	// Parse source to correlate with machine code
	lines := strings.Split(source, "\n")
	addr := uint16(0)
	byteIndex := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		
		// Skip directives for now (would need more sophisticated parsing)
		if strings.HasPrefix(line, ".") {
			continue
		}
		
		// Simple heuristic: assume each non-empty, non-comment line is an instruction
		if byteIndex < len(machineCode) {
			// Guess instruction length (simplified)
			instrLen := guessInstructionLength(machineCode, byteIndex)
			
			bytes := []string{}
			for i := 0; i < instrLen && byteIndex+i < len(machineCode); i++ {
				bytes = append(bytes, fmt.Sprintf("%02X", machineCode[byteIndex+i]))
			}
			
			instructions = append(instructions, InstructionInfo{
				Address:   addr,
				Bytes:     bytes,
				Source:    line,
				ByteCount: instrLen,
			})
			
			addr += uint16(instrLen)
			byteIndex += instrLen
		}
	}
	
	return instructions
}

func guessInstructionLength(machineCode []uint8, index int) int {
	if index >= len(machineCode) {
		return 0
	}
	
	opcode := machineCode[index]
	
	// Simple length determination based on opcode patterns
	switch {
	case opcode == 0xCB: // CB prefix
		return 2
	case opcode == 0xDD || opcode == 0xFD: // DD/FD prefix
		return 2 // Simplified
	case opcode == 0xED: // ED prefix
		return 2
	case (opcode & 0xC7) == 0x06: // LD r,n
		return 2
	case (opcode & 0xCF) == 0x01: // LD rp,nn
		return 3
	case opcode == 0xC3 || opcode == 0xCD: // JP nn, CALL nn
		return 3
	default:
		return 1
	}
}

func analyzeDataAreas(machineCode []uint8) []DataArea {
	// Simplified data area analysis
	return []DataArea{
		{
			StartAddress: 0x8000,
			EndAddress:   uint16(0x8000 + len(machineCode) - 1),
			Size:         len(machineCode),
			Type:         "mixed",
		},
	}
}

func estimateCycles(instructions []InstructionInfo) CycleInfo {
	// Simplified cycle estimation
	totalMin := 0
	totalMax := 0
	
	for _, instr := range instructions {
		// Very rough cycle estimates based on byte count
		cycles := instr.ByteCount * 4 // Rough average
		totalMin += cycles
		totalMax += cycles + 3 // Add some variation for conditional instructions
	}
	
	return CycleInfo{
		MinCycles: totalMin,
		MaxCycles: totalMax,
		Note:      "Estimated based on instruction analysis",
	}
}

func countLabels(symbols map[string]uint16) int {
	// Count symbols that look like labels (non-constant addresses)
	count := 0
	for name, value := range symbols {
		if value >= 0x8000 || (value > 0xFF && !isConstantName(name)) {
			count++
		}
	}
	return count
}

func countConstants(symbols map[string]uint16) int {
	// Count symbols that look like constants
	count := 0
	for name, value := range symbols {
		if value <= 0xFF || isConstantName(name) {
			count++
		}
	}
	return count
}

func isConstantName(name string) bool {
	// Simple heuristic for constant names
	return strings.Contains(name, "COUNT") || 
		   strings.Contains(name, "CHAR") || 
		   strings.Contains(name, "SIZE") ||
		   strings.HasSuffix(name, "_CONST")
}

func outputJSON(data interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}
	
	fmt.Println(string(jsonBytes))
	return nil
}

// writeSymFile writes a symbol table in the pasmo .sym format:
//
//	NAME\t\tEQU 0XXXXH
//
// one line per symbol, sorted by name. Values are 16-bit, rendered as
// zero-padded uppercase hex with a leading 0 and trailing H (pasmo's
// convention, e.g. 08000H). This is the format the downstream tooling
// (gen_memmap.py, release.sh) parses.
func writeSymFile(path string, symbols map[string]uint16) error {
	names := make([]string, 0, len(symbols))
	for name := range symbols {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s\t\tEQU 0%04XH\n", name, symbols[name])
	}
	return ioutil.WriteFile(path, []byte(b.String()), 0644)
}

// tagSymbol turns a --tag name into the symbol it defines: ZENAS_TAG_<name>.
// The name must be a valid symbol identifier (letters, digits, underscore; not
// starting with a digit) so the resulting symbol is usable in IF/IFDEF.
func tagSymbol(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("--tag requires a non-empty tag name")
	}
	for i, r := range tag {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if r == '_' || isLetter || (isDigit && i > 0) {
			continue
		}
		return "", fmt.Errorf("invalid --tag name %q: tags must be identifiers (letters, digits, underscore; not starting with a digit)", tag)
	}
	return "ZENAS_TAG_" + tag, nil
}

// parseDefine parses a --define value of the form NAME or NAME=VALUE.
// A bare NAME defines the symbol as 1. VALUE accepts decimal, 0x hex, or $ hex.
func parseDefine(s string) (string, uint16, error) {
	name := s
	valStr := "1"
	if idx := strings.IndexByte(s, '='); idx >= 0 {
		name = s[:idx]
		valStr = s[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, fmt.Errorf("--define requires a symbol name")
	}
	valStr = strings.TrimSpace(valStr)
	var v int64
	var err error
	switch {
	case strings.HasPrefix(valStr, "0x"), strings.HasPrefix(valStr, "0X"):
		v, err = strconv.ParseInt(valStr[2:], 16, 32)
	case strings.HasPrefix(valStr, "$"):
		v, err = strconv.ParseInt(valStr[1:], 16, 32)
	default:
		v, err = strconv.ParseInt(valStr, 10, 32)
	}
	if err != nil {
		return "", 0, fmt.Errorf("invalid --define value %q: %v", valStr, err)
	}
	return name, uint16(v), nil
}

func assembleFile(inputFile, outputFile, symFile, hexMode, charset string, showWarnings bool, defines map[string]uint16, z80n bool) error {
	// Read input file
	content, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}
	
	// Assemble
	asm := assembler.New()

	// Enable the Z80N (ZX Spectrum Next) extended instruction set if requested.
	if z80n {
		asm.EnableZ80N()
	}
	
	// Set character set
	if err := asm.SetCharset(charset); err != nil {
		return fmt.Errorf("invalid charset '%s': %v", charset, err)
	}

	// Apply command-line defines (build-tag style) before assembly.
	for name, val := range defines {
		asm.Define(name, val)
	}
	
	asm.SetBaseDir(filepath.Dir(inputFile))
	result, err := asm.AssembleString(string(content))
	
	// Show assembly errors first if they exist
	if result != nil && len(result.Errors) > 0 {
		fmt.Printf("Assembly errors:\n")
		for _, e := range result.Errors {
			fmt.Printf("  Line %d: %s\n", e.Line, e.Message)
		}
		return fmt.Errorf("assembly failed with %d error(s)", len(result.Errors))
	}
	
	// Show warnings if requested and they exist
	if showWarnings && result != nil && len(result.Warnings) > 0 {
		fmt.Printf("Assembly warnings:\n")
		for _, w := range result.Warnings {
			fmt.Printf("  Line %d: %s\n", w.Line, w.Message)
		}
		fmt.Println()
	}
	
	// Check for other errors
	if err != nil {
		return err
	}
	
	// Write output file
	err = ioutil.WriteFile(outputFile, result.MachineCode, 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	// Write symbol file if requested
	if symFile != "" {
		if err := writeSymFile(symFile, result.Symbols); err != nil {
			return fmt.Errorf("failed to write symbol file: %v", err)
		}
	}
	
	// Print assembly summary
	fmt.Printf("Assembled %d bytes", len(result.MachineCode))
	if showWarnings && result != nil && len(result.Warnings) > 0 {
		fmt.Printf(" (with %d warning(s))", len(result.Warnings))
	}
	fmt.Println()
	
	if len(result.Symbols) > 0 {
		fmt.Println("\nSymbol Table:")
		for symbol, addr := range result.Symbols {
			fmt.Printf("  %-12s = $%04X (%d)\n", symbol, addr, addr)
		}
	}
	
	// Print hex dump based on mode
	shouldShowHex := false
	switch hexMode {
	case "force":
		shouldShowHex = len(result.MachineCode) > 0
	case "never":
		shouldShowHex = false
	case "auto":
		shouldShowHex = len(result.MachineCode) > 0 && len(result.MachineCode) <= 64
	}
	
	if shouldShowHex {
		fmt.Printf("\nHex Dump:\n")
		for i := 0; i < len(result.MachineCode); i += 8 {
			fmt.Printf("%04X: ", i)
			end := i + 8
			if end > len(result.MachineCode) {
				end = len(result.MachineCode)
			}
			for j := i; j < end; j++ {
				fmt.Printf("%02X ", result.MachineCode[j])
			}
			fmt.Println()
		}
	}
	
	return nil
}

// Helper functions

func bytesEqual(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatBytes(bytes []uint8) string {
	var parts []string
	for _, b := range bytes {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return "[" + strings.Join(parts, " ") + "]"
}