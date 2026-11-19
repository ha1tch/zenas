// Package artifact turns an assembled program into the files a Spectrum,
// a Spectrum Next or an emulator loads: what `zenas build` writes, for a
// program that assembles through the assembler package and wants the
// result as a tape, a snapshot or a NEX file in memory.
//
//	result, err := asm.AssembleString(source)
//	img, err := artifact.Encode(result, artifact.NEX, artifact.Options{Name: "game", Start: "main"})
//
// The formats themselves are written by zentools' pkg/build; this
// package is the step between an AssemblyResult and a build.Request,
// and the rules `zenas build` applies (which formats need an entry
// point, what a loader does to a tape, which format is the Next's).
package artifact

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zentools/pkg/build"
)

// Format is one of the files Encode writes. Its value is the file's
// extension, without the dot.
type Format string

const (
	TAP Format = "tap" // tape: the code as one CODE block
	TZX Format = "tzx" // the same tape as TZX
	PZX Format = "pzx" // the same tape as PZX, with the ROM's standard timings
	SNA Format = "sna" // snapshot, 48K or 128K by Options.Model
	Z80 Format = "z80" // snapshot, version 3
	SZX Format = "szx" // snapshot in the ZX-State format
	NEX Format = "nex" // Spectrum Next executable
)

// Formats returns every format, in the order `zenas build` writes them.
func Formats() []Format { return []Format{TAP, TZX, SNA, Z80, SZX, PZX, NEX} }

// ParseFormat reads a format's name: its extension, with or without the
// dot, in either case.
func ParseFormat(s string) (Format, error) {
	name := Format(strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), ".")))
	for _, f := range Formats() {
		if f == name {
			return f, nil
		}
	}
	return "", fmt.Errorf("unknown format %q (want tap, tzx, pzx, sna, z80, szx or nex)", s)
}

// Ext is the file extension, with its dot.
func (f Format) Ext() string { return "." + string(f) }

// Tape reports a tape format: TAP, TZX or PZX.
func (f Format) Tape() bool { return f == TAP || f == TZX || f == PZX }

// Snapshot reports a snapshot format: SNA, Z80 or SZX.
func (f Format) Snapshot() bool { return f == SNA || f == Z80 || f == SZX }

// Next reports the format that runs on a Spectrum Next only, NEX. Its
// source is assembled with the Z80N instructions on
// (assembler.EnableZ80N) unless it uses none: `zenas build --nex` turns
// them on by itself.
func (f Format) Next() bool { return f == NEX }

// NeedsStart reports whether the format needs an entry point: a
// snapshot and a NEX file always, a tape when it has a loader.
func (f Format) NeedsStart(loader bool) bool {
	return f.Snapshot() || f == NEX || (f.Tape() && loader)
}

// Options is what a file needs that the assembly does not say.
type Options struct {
	// Name is the program's name on a tape and in a PZX title, 10
	// characters at most (longer is cut).
	Name string
	// Start is the entry point: an address (decimal, 0x or $ hex) or a
	// label of the source. Needed where Format.NeedsStart says so.
	Start string
	// SP is the stack pointer; nil is build.DefaultSP.
	SP *uint16
	// Model is the machine a snapshot is made for; empty is the 48K.
	Model build.Model
	// Loader puts a BASIC program ahead of the code on a tape, so that
	// LOAD "" loads and runs it. It has no meaning for the other formats.
	Loader bool
	// NEX is a NEX file's border colour and the core version it asks for.
	NEX build.NEXOptions
}

// Request makes zentools' build.Request of an assembly, for a caller
// that calls pkg/build itself. needStart says whether an empty
// Options.Start is an error.
func Request(result *assembler.AssemblyResult, o Options, needStart bool) (build.Request, error) {
	if result == nil || len(result.MachineCode) == 0 {
		return build.Request{}, fmt.Errorf("artifact: the assembly has no code")
	}
	req := build.Request{
		Name:   o.Name,
		Code:   result.MachineCode,
		Origin: result.Origin,
		SP:     build.DefaultSP,
		Model:  o.Model,
	}
	if req.Model == "" {
		req.Model = build.Model48K
	}
	if len(req.Name) > 10 {
		req.Name = req.Name[:10]
	}
	if o.SP != nil {
		req.SP = *o.SP
	}
	switch {
	case o.Start != "":
		start, err := ResolveStart(o.Start, result)
		if err != nil {
			return build.Request{}, err
		}
		req.Start = start
	case needStart:
		return build.Request{}, fmt.Errorf("artifact: an entry point (Options.Start) is needed")
	}
	return req, nil
}

// Encode writes an assembled program in one format.
func Encode(result *assembler.AssemblyResult, format Format, o Options) ([]byte, error) {
	if _, err := ParseFormat(string(format)); err != nil {
		return nil, err
	}
	req, err := Request(result, o, format.NeedsStart(o.Loader))
	if err != nil {
		return nil, err
	}
	if format.Tape() {
		tape := build.EncodeTAP(req)
		if o.Loader {
			if tape, err = build.EncodeTAPWithLoader(req); err != nil {
				return nil, err
			}
		}
		switch format {
		case TZX:
			return build.EncodeTZXFromTAP(tape)
		case PZX:
			return build.EncodePZXFromTAP(tape, req.Name)
		}
		return tape, nil
	}
	switch format {
	case SNA:
		return build.EncodeSNA(req)
	case Z80:
		return build.EncodeZ80(req)
	case SZX:
		return build.EncodeSZX(req)
	}
	return build.EncodeNEX(req, o.NEX)
}

// ResolveStart turns an entry point given as text into an address: a
// number (decimal, 0x or $ hex), or a symbol of the assembly, looked up
// exactly and then without regard to case.
func ResolveStart(spec string, result *assembler.AssemblyResult) (uint16, error) {
	if addr, err := ParseAddress(spec); err == nil {
		return addr, nil
	}
	if result != nil && result.Symbols != nil {
		if addr, ok := result.Symbols[spec]; ok {
			return addr, nil
		}
		for name, addr := range result.Symbols {
			if strings.EqualFold(name, spec) {
				return addr, nil
			}
		}
	}
	return 0, fmt.Errorf("start %q is neither a valid address nor a defined symbol", spec)
}

// ParseAddress reads a 16-bit address: decimal, 0x hex or $ hex.
func ParseAddress(s string) (uint16, error) {
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

// ParseNEXOptions reads a NEX file's border colour ("0" to "7") and the
// core version it asks for ("a.b.c"), as text. An empty border is 0 and
// an empty core is build.DefaultNEXCore.
func ParseNEXOptions(border, core string) (build.NEXOptions, error) {
	o := build.NEXOptions{Core: build.DefaultNEXCore}
	if border != "" {
		n, err := strconv.ParseUint(strings.TrimSpace(border), 10, 8)
		if err != nil || n > 7 {
			return o, fmt.Errorf("border %q is not a colour from 0 to 7", border)
		}
		o.Border = uint8(n)
	}
	if core != "" {
		parts := strings.Split(strings.TrimSpace(core), ".")
		if len(parts) != 3 {
			return o, fmt.Errorf("core %q is not a version of the form a.b.c", core)
		}
		for i, p := range parts {
			n, err := strconv.ParseUint(p, 10, 8)
			if err != nil {
				return o, fmt.Errorf("core %q is not a version of the form a.b.c", core)
			}
			o.Core[i] = uint8(n)
		}
	}
	return o, nil
}
