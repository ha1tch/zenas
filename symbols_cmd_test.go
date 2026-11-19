package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenas/assembler"
)

const mapSource = `BORDER  EQU 0x229B
COUNT   EQU 5
        ORG 0x8000
start:  ld b,COUNT
loop:   call BORDER
        djnz loop
        jp upper
table:  db 1,2,3
        ORG 0xC000
upper:  ret
        ORG 0x4000
screen: nop
        ORG 0xE100
top:    nop
        ORG 0x0100
lower:  nop
`

// What sjasmplus 1.24.1 wrote with CSPECTMAP for mapSource under DEVICE
// ZXSPECTRUMNEXT, but for the names' case: sjasmplus writes them in
// capitals and zenas as the source has them.
const mapWanted = `0000229B 0000029B 01 BORDER
00000005 00000005 01 COUNT
00008002 00008002 00 loop
00000100 0001C100 00 lower
00004000 00014000 00 screen
00008000 00008000 00 start
0000800A 0000800A 00 table
0000E100 00002100 00 top
0000C000 00000000 00 upper
`

func assembleForMap(t *testing.T, source string, setup func(*assembler.Assembler)) *assembler.AssemblyResult {
	t.Helper()
	asm := assembler.New()
	if setup != nil {
		setup(asm)
	}
	result, err := asm.AssembleString(source)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, result.Errors)
	}
	return result
}

func TestMapIsTheOneSjasmplusWrites(t *testing.T) {
	result := assembleForMap(t, mapSource, nil)
	path := filepath.Join(t.TempDir(), "p.map")
	if err := writeMapFile(path, result, assembler.DefaultNextPaging); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != mapWanted {
		t.Errorf("the map:\n%swant:\n%s", got, mapWanted)
	}

	// With other pages in the top two slots, the labels there move and
	// nothing else does.
	if err := writeMapFile(path, result, [8]uint32{14, 15, 10, 11, 4, 5, 60, 61}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	want := strings.NewReplacer("0000E100 00002100 00 top", "0000E100 0007A100 00 top", "0000C000 00000000 00 upper", "0000C000 00078000 00 upper").Replace(mapWanted)
	if string(got) != want {
		t.Errorf("the map with pages 60 and 61 on top:\n%swant:\n%s", got, want)
	}
}

func TestSymbolKinds(t *testing.T) {
	result := assembleForMap(t, mapSource, func(a *assembler.Assembler) { a.Define("ZENAS_TAG_x", 1) })
	for name, want := range map[string]assembler.SymbolKind{"start": assembler.SymbolLabel, "table": assembler.SymbolLabel, "BORDER": assembler.SymbolConstant, "COUNT": assembler.SymbolConstant, "ZENAS_TAG_x": assembler.SymbolConstant} {
		got, ok := result.SymbolKinds[name]
		if !ok || got != want {
			t.Errorf("%s: kind %v (there: %v), want %v", name, got, ok, want)
		}
	}
	if len(result.SymbolKinds) != len(result.Symbols) {
		t.Errorf("%d kinds for %d symbols", len(result.SymbolKinds), len(result.Symbols))
	}
}

func TestImportedSymbols(t *testing.T) {
	dir := t.TempDir()
	rom := filepath.Join(dir, "rom.map")
	os.WriteFile(rom, []byte("[SYMBOLS]\n0000229B 0001E29B 00 BORDER\n00000010 0001C010 00 PRINT@A\n00000005 00000005 01 COUNT\n00012345 00012345 00 FAR\n[FILES]\nc:\\x.asm\n"), 0644)
	sym := filepath.Join(dir, "more.sym")
	os.WriteFile(sym, []byte("CLS\t\tEQU 00DAFH\nBORDER\t\tEQU 0229BH\n"), 0644)

	opts := newSymbolOptions()
	for _, arg := range []string{"--symbols=" + rom, "--symbols=" + sym} {
		if handled, err := opts.parse(arg); !handled || err != nil {
			t.Fatalf("%s: handled %v, %v", arg, handled, err)
		}
	}
	const source = "        ORG 0x8000\nmain:   ld b,COUNT\n        call BORDER\n        call PRINT_A\n        call CLS\n        ret\n"
	var n int
	result := assembleForMap(t, source, func(a *assembler.Assembler) {
		var err error
		if n, err = opts.importSymbols(a); err != nil {
			t.Fatal(err)
		}
	})
	if n != 4 {
		t.Errorf("%d names imported, want 4: FAR does not fit in 16 bits and BORDER is in both files", n)
	}
	want := []byte{0x06, 0x05, 0xCD, 0x9B, 0x22, 0xCD, 0x10, 0x00, 0xCD, 0xAF, 0x0D, 0xC9}
	if string(result.MachineCode) != string(want) {
		t.Errorf("code % X, want % X", result.MachineCode, want)
	}
	// What was imported is not the program's: only main is.
	if len(result.Symbols) != 1 || result.Symbols["main"] != 0x8000 {
		t.Errorf("the program's symbols: %v", result.Symbols)
	}

	// A name the source defines itself is the program's again.
	result = assembleForMap(t, "COUNT EQU 9\n        ORG 0x8000\nmain:   ld b,COUNT\n", func(a *assembler.Assembler) { opts.importSymbols(a) })
	if result.Symbols["COUNT"] != 9 || result.SymbolKinds["COUNT"] != assembler.SymbolConstant || result.MachineCode[1] != 9 {
		t.Errorf("COUNT defined over an imported one: %v, code % X", result.Symbols, result.MachineCode)
	}

	// Two files that disagree are an error.
	os.WriteFile(sym, []byte("BORDER\t\tEQU 01234H\n"), 0644)
	if _, err := opts.importSymbols(assembler.New()); err == nil || !strings.Contains(err.Error(), "BORDER") {
		t.Errorf("two values for BORDER: %v", err)
	}
}

func TestSymbolOptionParsing(t *testing.T) {
	o := newSymbolOptions()
	if handled, _ := o.parse("--sym"); handled {
		t.Error("--sym is not one of these options")
	}
	if handled, err := o.parse("--map"); !handled || err != nil || o.mapPath("out/game") != "out/game.map" {
		t.Errorf("--map: %v %v %q", handled, err, o.mapPath("out/game"))
	}
	if _, err := o.parse("--map=x.map"); err != nil || o.mapPath("game") != "x.map" {
		t.Errorf("--map=x.map: %v %q", err, o.mapPath("game"))
	}
	if _, err := o.parse("--map-paging=14,15,10,11,4,5,0x3C,61"); err != nil || o.paging[6] != 60 || o.paging[7] != 61 {
		t.Errorf("--map-paging: %v %v", err, o.paging)
	}
	for _, bad := range []string{"--map-paging=1,2,3", "--map-paging=a,b,c,d,e,f,g,h", "--symbols-format=vice"} {
		if handled, err := o.parse(bad); !handled || err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
	if newSymbolOptions().mapPath("game") != "" {
		t.Error("a map without --map")
	}
}
