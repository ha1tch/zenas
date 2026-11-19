package assembler

import (
	"fmt"
	"io"
	"sort"

	"github.com/ha1tch/zentools/pkg/symfile"
)

// Symbol files, for a program that embeds the assembler: what the
// command line's --sym, --map and --symbols do, without the files.
//
//	asm := assembler.New()
//	asm.ImportSymbols(romMap, symfile.FormatAuto) // names the source may use
//	result, _ := asm.AssembleString(source)
//	assembler.WriteCSpectMap(&buf, result, assembler.DefaultNextPaging)
//
// The symbol types are zentools' pkg/symfile, which zendis uses too: what
// ExportSymbols returns goes straight into a symfile.Table, which is how
// the names of an assembled program reach a disassembler or a debugger
// in the same process.

// DefaultNextPaging is the 8K page in each of the eight slots of a
// Next's 64K as a program finds them, numbered as sjasmplus numbers them
// for its ZXSPECTRUMNEXT device: 14 and 15 under the ROM, then bank 5
// (10, 11), bank 2 (4, 5) and bank 0 (0, 1).
var DefaultNextPaging = [8]uint32{14, 15, 10, 11, 4, 5, 0, 1}

// ExportSymbols returns the symbols a program defines, sorted by name,
// as symbol-file entries: a label with the physical address its 8K
// slot's page gives it (the page times 8192 plus the offset), a
// constant with its offset in an 8K page, which is what sjasmplus puts
// there for an EQU. Imported symbols are not the program's and are not
// among them.
func ExportSymbols(result *AssemblyResult, paging [8]uint32) []symfile.Symbol {
	names := make([]string, 0, len(result.Symbols))
	for name := range result.Symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	symbols := make([]symfile.Symbol, 0, len(names))
	for _, name := range names {
		value := result.Symbols[name]
		s := symfile.Symbol{Name: name, Value: uint32(value), Phys: uint32(value & 0x1FFF), HasPhys: true, Kind: symfile.KindEqu}
		if result.SymbolKinds[name] == SymbolLabel {
			s.Kind = symfile.KindLabel
			s.Phys = paging[value>>13]*0x2000 + uint32(value&0x1FFF)
		}
		symbols = append(symbols, s)
	}
	return symbols
}

// WriteCSpectMap writes a program's symbols as the map file CSpect's
// debugger loads with -map, the file SNASM and sjasmplus (CSPECTMAP)
// write: one line a symbol, "<16-bit value> <physical address> <type>
// <name>", type 00 for a label and 01 for a constant.
func WriteCSpectMap(w io.Writer, result *AssemblyResult, paging [8]uint32) error {
	return symfile.WriteCSpectMap(w, ExportSymbols(result, paging))
}

// WritePasmoSym writes a program's symbols in pasmo's .sym format,
// "NAME<two tabs>EQU 0XXXXH", one a line, sorted by name.
func WritePasmoSym(w io.Writer, result *AssemblyResult) error {
	names := make([]string, 0, len(result.Symbols))
	for name := range result.Symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "%s\t\tEQU 0%04XH\n", name, result.Symbols[name]); err != nil {
			return err
		}
	}
	return nil
}

// ImportSymbols reads another tool's symbol file (a CSpect map, a pasmo
// or zenas .sym, an sjasmplus --sym file, a z88dk .map; symfile.FormatAuto
// tells which from the first line) and defines its names for the source
// to use, as Import does one at a time. It returns how many names the
// file gave.
//
// A character a zenas name cannot have becomes an underscore
// (symfile.Identifier). A value that does not fit in 16 bits is left
// out. A name that an earlier ImportSymbols, or an earlier line of this
// file, gave another value is an error, and nothing after it is
// imported: the source would otherwise mean one or the other by the
// order of the files.
func (a *Assembler) ImportSymbols(r io.Reader, format symfile.Format) (int, error) {
	symbols, err := symfile.Parse(r, format)
	if err != nil {
		return 0, err
	}
	if a.imported == nil {
		a.imported = map[string]uint16{}
	}
	n := 0
	for _, s := range symbols {
		if s.Value > 0xFFFF {
			continue
		}
		name, value := symfile.Identifier(s.Name), uint16(s.Value)
		if name == "" {
			continue
		}
		if old, dup := a.imported[name]; dup {
			if old != value {
				return n, fmt.Errorf("%s is 0x%04X here and 0x%04X in what was imported before", name, value, old)
			}
			continue
		}
		a.imported[name] = value
		a.Import(name, value)
		n++
	}
	return n, nil
}
