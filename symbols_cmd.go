package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ha1tch/zenas/assembler"
	"github.com/ha1tch/zentools/pkg/symfile"
)

// The command line's side of symbol files: --map writes the map CSpect's
// debugger loads, and --symbols reads names from a file another tool
// wrote. What they do is the assembler package's (symbolfiles.go), for a
// program that embeds it; this file is the options and the files.

// symbolOptions is what the command line says about symbol files beyond
// --sym.
type symbolOptions struct {
	mapFile      string // where the CSpect map goes; "" with mapRequested: beside the output
	mapRequested bool
	paging       [8]uint32
	imports      []string // --symbols files, in the order given
	importFormat symfile.Format
}

func newSymbolOptions() *symbolOptions {
	return &symbolOptions{paging: assembler.DefaultNextPaging, importFormat: symfile.FormatAuto}
}

// parse takes one command-line argument. handled is false when the
// argument is not one of these options.
func (o *symbolOptions) parse(arg string) (handled bool, err error) {
	switch {
	case arg == "--map":
		o.mapRequested = true
	case strings.HasPrefix(arg, "--map="):
		o.mapFile, o.mapRequested = strings.TrimPrefix(arg, "--map="), true
	case strings.HasPrefix(arg, "--map-paging="):
		parts := strings.Split(strings.TrimPrefix(arg, "--map-paging="), ",")
		if len(parts) != 8 {
			return true, fmt.Errorf("--map-paging needs eight page numbers, one for each 8K slot from address 0 (the default is 14,15,10,11,4,5,0,1)")
		}
		for i, p := range parts {
			n, perr := strconv.ParseUint(strings.TrimSpace(p), 0, 8)
			if perr != nil {
				return true, fmt.Errorf("--map-paging: %q is not a page number (0 to 255)", p)
			}
			o.paging[i] = uint32(n)
		}
	case strings.HasPrefix(arg, "--symbols="):
		o.imports = append(o.imports, strings.TrimPrefix(arg, "--symbols="))
	case strings.HasPrefix(arg, "--symbols-format="):
		f, ferr := symfile.ParseFormat(strings.TrimPrefix(arg, "--symbols-format="))
		if ferr != nil {
			return true, ferr
		}
		o.importFormat = f
	default:
		return false, nil
	}
	return true, nil
}

// mapPath gives where the map goes: the path asked for, or the output's
// name with .map.
func (o *symbolOptions) mapPath(outputBase string) string {
	if !o.mapRequested {
		return ""
	}
	if o.mapFile != "" {
		return o.mapFile
	}
	return outputBase + ".map"
}

// importSymbols gives the assembler the names of the --symbols files and
// returns how many there were.
func (o *symbolOptions) importSymbols(asm *assembler.Assembler) (int, error) {
	total := 0
	for _, path := range o.imports {
		f, err := os.Open(path)
		if err != nil {
			return total, err
		}
		n, err := asm.ImportSymbols(f, o.importFormat)
		f.Close()
		if err != nil {
			return total, fmt.Errorf("%s: %v", path, err)
		}
		total += n
	}
	return total, nil
}

// writeMapFile writes the program's symbols as a CSpect map.
func writeMapFile(path string, result *assembler.AssemblyResult, paging [8]uint32) error {
	return writeTo(path, func(f *os.File) error { return assembler.WriteCSpectMap(f, result, paging) })
}

// writeTo creates a file, writes it and closes it, with the first error.
func writeTo(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	werr := write(f)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}
