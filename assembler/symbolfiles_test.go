package assembler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/symfile"
)

const symbolFilesSource = `BORDER  EQU 0x229B
        ORG 0x8000
start:  call BORDER
        jp upper
        ORG 0xC000
upper:  ret
`

func TestSymbolFilesFromAResult(t *testing.T) {
	a := New()
	result, err := a.AssembleString(symbolFilesSource)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assembly failed: %v %v", err, result.Errors)
	}

	var m bytes.Buffer
	if err := WriteCSpectMap(&m, result, DefaultNextPaging); err != nil {
		t.Fatal(err)
	}
	want := "0000229B 0000029B 01 BORDER\n00008000 00008000 00 start\n0000C000 00000000 00 upper\n"
	if m.String() != want {
		t.Errorf("the map:\n%swant:\n%s", m.String(), want)
	}

	var s bytes.Buffer
	if err := WritePasmoSym(&s, result); err != nil {
		t.Fatal(err)
	}
	if want := "BORDER\t\tEQU 0229BH\nstart\t\tEQU 08000H\nupper\t\tEQU 0C000H\n"; s.String() != want {
		t.Errorf("the .sym:\n%swant:\n%s", s.String(), want)
	}

	syms := ExportSymbols(result, [8]uint32{14, 15, 10, 11, 4, 5, 60, 61})
	if len(syms) != 3 || syms[0].Name != "BORDER" || syms[0].Kind != symfile.KindEqu || syms[2].Name != "upper" || syms[2].Phys != 60*0x2000 || syms[2].Kind != symfile.KindLabel {
		t.Errorf("exported: %+v", syms)
	}

	// What one program exports, another imports: the map straight from a
	// buffer, as a program embedding both would pass it.
	b := New()
	n, err := b.ImportSymbols(&m, symfile.FormatAuto)
	if err != nil || n != 3 {
		t.Fatalf("imported %d names, %v", n, err)
	}
	user, err := b.AssembleString("        ORG 0x9000\nmain:   call start\n        jp upper\n")
	if err != nil || len(user.Errors) > 0 {
		t.Fatalf("assembly with imports failed: %v %v", err, user.Errors)
	}
	if got := user.MachineCode; !bytes.Equal(got, []byte{0xCD, 0x00, 0x80, 0xC3, 0x00, 0xC0}) {
		t.Errorf("code % X", got)
	}
	if len(user.Symbols) != 1 {
		t.Errorf("imported names are among the program's own: %v", user.Symbols)
	}
}

func TestImportSymbolsRules(t *testing.T) {
	a := New()
	// A local label of a CSpect map, a value too wide, and a name twice
	// with one value.
	n, err := a.ImportSymbols(strings.NewReader("00000010 0001C010 00 PRINT@A\n00012345 00012345 00 FAR\n00000010 0001C010 00 PRINT@A\n"), symfile.FormatAuto)
	if err != nil || n != 1 {
		t.Fatalf("%d names, %v; want one", n, err)
	}
	res, err := a.AssembleString("        ORG 0x8000\n        call PRINT_A\n")
	if err != nil || len(res.Errors) > 0 || !bytes.Equal(res.MachineCode, []byte{0xCD, 0x10, 0x00}) {
		t.Errorf("PRINT_A: %v %v % X", err, res.Errors, res.MachineCode)
	}
	// The same value again from another file is fine; another value is not.
	if n, err := a.ImportSymbols(strings.NewReader("PRINT_A\t\tEQU 00010H\n"), symfile.FormatAuto); err != nil || n != 0 {
		t.Errorf("the same name and value again: %d, %v", n, err)
	}
	if _, err := a.ImportSymbols(strings.NewReader("PRINT_A\t\tEQU 00020H\n"), symfile.FormatAuto); err == nil || !strings.Contains(err.Error(), "PRINT_A") {
		t.Errorf("another value for PRINT_A: %v", err)
	}
	if _, err := a.ImportSymbols(strings.NewReader("this is not a symbol file\n"), symfile.FormatAuto); err == nil {
		t.Error("a bad file was imported")
	}
}
