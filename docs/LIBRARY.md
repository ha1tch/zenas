# zenas — as a library

What the command does is in packages, for a program that embeds the
assembler (an emulator's debugger, a build tool):

| Package | What it has |
|---------|-------------|
| `assembler` | The assembler: `New`, `EnableZ80N`, `Define`, `SetCharset`, `AssembleString`/`AssembleFile`. Build tags and `--define` (`DefineTags`, `TagDefines`, `ParseDefine`). Symbol files (`ImportSymbols`, `ExportSymbols`, `WriteCSpectMap`, `WritePasmoSym`). |
| `pkg/artifact` | `zenas build`: `Encode(result, format, options)` returns a `.tap`, `.tzx`, `.pzx`, `.sna`, `.z80`, `.szx` or `.nex` as bytes; `Request` gives zentools' `build.Request` for a caller that calls `pkg/build` itself. |
| `pkg/z80enc` | The opcode encoding, as pure functions (below). |

```go
import (
    "github.com/ha1tch/zenas/assembler"
    "github.com/ha1tch/zenas/pkg/artifact"
)

asm := assembler.New()
asm.EnableZ80N()                          // --next
asm.DefineTags([]string{"demo"})          // --tag demo
result, err := asm.AssembleString(source)

nex, err := artifact.Encode(result, artifact.NEX, artifact.Options{Name: "game", Start: "main"})
tap, err := artifact.Encode(result, artifact.TAP, artifact.Options{Name: "game", Start: "main", Loader: true})
```

Not in a package: `zenas run`, `assert` and `test` (running assembled
code on the emulator core and checking `.EXPECT`/`.MATCH`), and the
`--json` reports. They are in the command only.

`pkg/z80enc` is zenas's own opcode-encoding knowledge, exported directly:
pure, dependency-free functions taking plain Go primitives (register
names as strings, displacements as `int8`, bit numbers as `uint8`)
rather than any parser-specific operand type. `encoder.go`'s own
`encode*` functions are thin adapters over this package, not duplicate
logic, so a fork with its own front end can import the real opcode
arithmetic directly instead of re-deriving it from release notes each
time a fix lands:

```go
import "github.com/ha1tch/zenas/pkg/z80enc"

b, err := z80enc.EncodeDDFDCBCopy(0x00, "IX", 1, "B") // RLC (IX+1),B
// b == []byte{0xDD, 0xCB, 0x01, 0x00}
```

Covers every instruction group in this codebase's own encoder: 8-bit
and 16-bit register operations, indirect-memory addressing, jump/call/
branch, the DDCB/FDCB group (including its undocumented copy-to-
register forms), the Z80N extensions, and more. See the package's own
doc comments for the full surface.

The manual has the calls beside the options they stand for: symbol files
under [Symbol files](MANUAL.md#symbol-files) and `pkg/artifact` under
[Packaging](MANUAL.md#packaging-zenas-build). zendis's
[`docs/LIBRARY.md`](https://github.com/ha1tch/zendis/blob/main/docs/LIBRARY.md)
has zenas and zendis in one process.
