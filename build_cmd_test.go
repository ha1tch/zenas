package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/nex"
	"github.com/ha1tch/zentools/pkg/pzx"
)

// zenasBinary builds the command once for the tests that run it.
func zenasBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "zenas-under-test")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("building zenas: %v\n%s", err, out)
	}
	return bin
}

const nextSource = `        ORG 0x8000
main:   nextreg 0x07, 3
        ld a, COLOUR
        out (0xFE), a
        IFDEF ZENAS_TAG_extra
        nop
        ENDIF
stop:   jr stop
`

// TestBuildNEX builds a Z80N source to a .nex and reads it back: the code
// is in bank 2 at its origin, and --define, --tag, --border and --core
// reach the file.
func TestBuildNEX(t *testing.T) {
	bin := zenasBinary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.asm")
	if err := os.WriteFile(src, []byte(nextSource), 0o644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "prog")
	out, err := exec.Command(bin, "build", src, "--nex", "--start", "main",
		"--define=COLOUR=4", "--tag", "extra", "--border", "4", "--core=3.1.10", "-o", base).CombinedOutput()
	if err != nil {
		t.Fatalf("zenas build: %v\n%s", err, out)
	}
	data, err := os.ReadFile(base + ".nex")
	if err != nil {
		t.Fatal(err)
	}
	f, err := nex.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.PC != 0x8000 || f.Border != 4 || f.CoreVersion != [3]uint8{3, 1, 10} {
		t.Errorf("PC %04X border %d core %v", f.PC, f.Border, f.CoreVersion)
	}
	bank := f.Banks[2]
	// ED 91 07 03 (NEXTREG 7,3); 3E 04 (LD A,4); D3 FE; 00 (the tag's NOP); 18 FE
	want := []byte{0xED, 0x91, 0x07, 0x03, 0x3E, 0x04, 0xD3, 0xFE, 0x00, 0x18, 0xFE}
	if len(bank) < len(want) || string(bank[:len(want)]) != string(want) {
		t.Errorf("bank 2 starts % X, want % X", bank[:min(len(bank), len(want))], want)
	}
}

// TestBuildZ80NOff: --nex turns the Z80N instructions on (TestBuildNEX
// gives no --next); with --cpu=Z80 the Z80N instruction is an error, and
// so it is for a format that is not the Next's. The error names the line.
func TestBuildZ80NOff(t *testing.T) {
	bin := zenasBinary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.asm")
	if err := os.WriteFile(src, []byte(nextSource), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, format := range [][]string{{"--nex", "--cpu=Z80"}, {"--sna"}} {
		args := append([]string{"build", src, "--start", "main", "--define=COLOUR=4", "-o", filepath.Join(dir, "prog")}, format...)
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err == nil {
			t.Fatalf("%v built with the Z80N instructions off:\n%s", format, out)
		}
		if !strings.Contains(string(out), "Line 2") {
			t.Errorf("%v: the error does not name the line:\n%s", format, out)
		}
	}
}

// TestBuildSZXAndPZX: the other two formats 0.15.0 adds are written and
// are what their names say.
func TestBuildSZXAndPZX(t *testing.T) {
	bin := zenasBinary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.asm")
	if err := os.WriteFile(src, []byte("        ORG 0x8000\nmain:   jr main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "prog")
	out, err := exec.Command(bin, "build", src, "--szx", "--pzx", "--loader", "--start", "main", "-o", base).CombinedOutput()
	if err != nil {
		t.Fatalf("zenas build: %v\n%s", err, out)
	}
	szx, err := os.ReadFile(base + ".szx")
	if err != nil || len(szx) < 4 || string(szx[:4]) != "ZXST" {
		t.Errorf(".szx: %v, starts %q", err, szx[:min(len(szx), 4)])
	}
	data, err := os.ReadFile(base + ".pzx")
	if err != nil {
		t.Fatal(err)
	}
	f, err := pzx.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	blocks := 0
	for _, b := range f.Blocks {
		if _, ok := b.(pzx.DataBlock); ok {
			blocks++
		}
	}
	if blocks != 4 { // loader header and program, code header and code
		t.Errorf("%d data blocks in the .pzx, want 4", blocks)
	}
}
