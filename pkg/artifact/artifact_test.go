package artifact

import (
	"bytes"
	"testing"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zentools/pkg/build"
	"github.com/ha1tch/zentools/pkg/nex"
	"github.com/ha1tch/zentools/pkg/tap"
)

func assemble(t *testing.T, next bool) *assembler.AssemblyResult {
	t.Helper()
	asm := assembler.New()
	source := "        ORG 0x8000\nmain:   ld a, 2\n        out (0xFE), a\nstop:   jr stop\n"
	if next {
		asm.EnableZ80N()
		source = "        ORG 0x8000\nmain:   nextreg 0x07, 3\nstop:   jr stop\n"
	}
	r, err := asm.AssembleString(source)
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("%v %v", err, r.Errors)
	}
	return r
}

// TestEncodeEveryFormat: each format is written, and is what zentools'
// pkg/build writes for the same request.
func TestEncodeEveryFormat(t *testing.T) {
	r := assemble(t, false)
	o := Options{Name: "prog", Start: "main"}
	req, err := Request(r, o, true)
	if err != nil {
		t.Fatal(err)
	}
	if req.Start != 0x8000 || req.Origin != 0x8000 || req.SP != build.DefaultSP || req.Name != "prog" {
		t.Fatalf("%+v", req)
	}
	for _, f := range Formats() {
		got, err := Encode(r, f, o)
		if err != nil || len(got) == 0 {
			t.Errorf("%s: %d bytes, %v", f, len(got), err)
		}
	}
	sna, _ := Encode(r, SNA, o)
	want, _ := build.EncodeSNA(req)
	if !bytes.Equal(sna, want) {
		t.Error("the .sna is not build.EncodeSNA's")
	}
}

func TestEncodeNEX(t *testing.T) {
	r := assemble(t, true)
	opts, err := ParseNEXOptions("4", "3.1.10")
	if err != nil {
		t.Fatal(err)
	}
	img, err := Encode(r, NEX, Options{Start: "MAIN", NEX: opts}) // a label, whatever its case
	if err != nil {
		t.Fatal(err)
	}
	f, err := nex.Decode(img)
	if err != nil {
		t.Fatal(err)
	}
	if f.PC != 0x8000 || f.Border != 4 || f.CoreVersion != [3]uint8{3, 1, 10} || !bytes.HasPrefix(f.Banks[2], []byte{0xED, 0x91, 0x07, 0x03}) {
		t.Errorf("PC %04X border %d core %v bank 2 % X", f.PC, f.Border, f.CoreVersion, f.Banks[2][:4])
	}
}

// TestLoaderAndStart: a tape needs an entry point only with a loader,
// which puts two blocks ahead of the code's two; the others always.
func TestLoaderAndStart(t *testing.T) {
	r := assemble(t, false)
	bare, err := Encode(r, TAP, Options{Name: "prog"})
	if err != nil {
		t.Fatal(err)
	}
	if blocks, _ := tap.Decode(bare); len(blocks) != 2 {
		t.Errorf("%d blocks on a bare tape", len(blocks))
	}
	loaded, err := Encode(r, TAP, Options{Name: "prog", Start: "0x8000", Loader: true})
	if err != nil {
		t.Fatal(err)
	}
	if blocks, _ := tap.Decode(loaded); len(blocks) != 4 {
		t.Errorf("%d blocks on a loader tape", len(blocks))
	}
	for _, c := range []struct {
		f Format
		o Options
	}{{TAP, Options{Loader: true}}, {SNA, Options{}}, {NEX, Options{}}, {Z80, Options{Start: "nowhere"}}, {Format("dsk"), Options{Start: "main"}}} {
		if _, err := Encode(r, c.f, c.o); err == nil {
			t.Errorf("%s %+v: no error", c.f, c.o)
		}
	}
}

func TestFormatsAndParsing(t *testing.T) {
	if f, err := ParseFormat(".NEX"); err != nil || f != NEX || f.Ext() != ".nex" || !f.Next() || !f.NeedsStart(false) {
		t.Errorf("%v %v", f, err)
	}
	if TAP.NeedsStart(false) || !TAP.NeedsStart(true) || !SZX.Snapshot() || !PZX.Tape() || SNA.Next() {
		t.Error("the formats' kinds are wrong")
	}
	if _, err := ParseFormat("dsk"); err == nil {
		t.Error("dsk is not an error")
	}
	if o, err := ParseNEXOptions("", ""); err != nil || o.Border != 0 || o.Core != build.DefaultNEXCore {
		t.Errorf("defaults: %+v %v", o, err)
	}
	for _, bad := range [][2]string{{"8", ""}, {"red", ""}, {"", "3.1"}, {"", "3.1.x"}, {"", "3.1.300"}} {
		if _, err := ParseNEXOptions(bad[0], bad[1]); err == nil {
			t.Errorf("border %q core %q: no error", bad[0], bad[1])
		}
	}
	if a, err := ParseAddress("$C000"); err != nil || a != 0xC000 {
		t.Errorf("%04X %v", a, err)
	}
}
