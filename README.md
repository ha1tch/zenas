# zenas

A Z80 macro assembler written in Go.

zenas assembles Z80 source to raw machine code. It has a two-pass design with
forward-reference resolution, a traditional macro system, and an instruction
encoder whose patterns derive from the zen80 emulator's decoder.

## Status

zenas assembles complete Z80 programs. It builds a real-world Z80 operating
system kernel (ZX Opal, ~6.5 KB with ten included subsystems at `ORG $5CAD`)
byte-for-byte identically to pasmo, with every symbol address matching.

The instruction set covers three classes, verified against established reference
assemblers (see [`docs/INSTRUCTION_SET.md`](docs/INSTRUCTION_SET.md)):

- the **468** documented Z80 instructions;
- the **48** undocumented IX/IY half-register forms (IXH/IXL/IYH/IYL), with the
  illegal combinations rejected;
- the **29** Z80N (ZX Spectrum Next) extensions, off by default and enabled with
  `--next` (alias `--cpu=Z80N`).

The documented and half-register forms match both pasmo and sjasmplus; the Z80N
forms match sjasmplus, the de facto ZX Spectrum Next assembler.

Working today: two-pass assembly with forward references; `INCLUDE` with
cross-boundary forward references; conditional assembly (`IF`/`IFDEF`/`IFNDEF`/
`ELSE`/`ENDIF`) with `--define` build flags; operand and displacement expressions
with symbol arithmetic; case-sensitive symbols; a pasmo-compatible symbol file
(`--sym`); the traditional macro system; number formats `0x`/`$`/`%`/decimal and
character literals; string and multi-value `DB`/`DW`; `DS`/`DEFS`; and `INCBIN` (with optional
skip and length).

## Build

Requires Go 1.25 or later.

```
make build         # -> build/zenas
make test          # run the test suite
make smoke         # assemble the bundled examples as a sanity check
```

Or directly:

```
go build -o build/zenas .
```

## Usage

```
zenas assemble <input.asm> [output.bin] [--hex] [--json=level] [--charset=name]
                [--sym[=path]] [--define=NAME[=VAL]] [--tag NAME]
                [--next | --cpu=Z80|Z80N]
zenas build <input.asm> [--tap] [--tzx] [--sna] [--z80] [--loader]
                [--start <addr|symbol>] [--sp <addr>]
                [--model 48k|128k|plus2|plus2a|plus3] [-o <basename>]
zenas version
zenas help
```

`--next` (or `--cpu=Z80N`) enables the ZX Spectrum Next Z80N instruction set; the
default is `--cpu=Z80`. `--tag NAME` selects a build tag, the way Go build tags
select variants. Each tag defines `ZENAS_TAG_NAME` (a presence flag),
`ZENAS_TAGBIT_NAME` (its bit, assigned in command-line order), and contributes to
the composite bitmask `ZENAS_TAGS` (always defined; 0 when no tags are set). Tags
compose in `IF` conditions with `AND`, `OR`, `NOT` and parentheses - e.g.
`IF ZENAS_TAG_debug AND ZENAS_TAG_plus3`. Examples are in [`examples/`](examples/).

### build

`zenas build` assembles the source and packages the result into loadable
artifacts. Where `assemble` produces a raw binary, `build` emits one or more of:

- `--sna`, `--z80` - snapshots. These carry a full machine state, so the code
  loads and runs immediately at the entry point given by `--start`.
- `--tap`, `--tzx` - tape images. The code is encoded as a CODE block; add
  `--loader` to prepend a BASIC auto-run loader so `LOAD ""` runs it.

**Recommended workflow: use `.z80` (v3) or `.sna` snapshots for development
testing, and tapes (`.tap`/`.tzx`) as the primary format for wider
distribution.** Snapshots get a build running in one step during iteration;
tapes are what you ship.

`--start` sets the entry point (an address such as `0x8000`, `$8000`, `32768`,
or a label from the source) and is required for snapshot output and for
`--loader`. `--sp` overrides the stack pointer (default `0xFF00`). `--model`
selects the target machine for snapshot output (the snapshot is overlaid on that
model's booted state). `-o` sets the output basename; each format appends its own
extension.

```
zenas build game.asm --z80 --start main --model 128k     # snapshot for testing
zenas build game.asm --tap --tzx --loader --start main   # tapes for release
```

## Testing

```
make test           # Go test suite
make smoke          # assemble the bundled examples
make test-sjasmplus # optional: cross-check encodings against sjasmplus
```

`make test-sjasmplus` builds sjasmplus from source and verifies that every
documented, half-register, and Z80N form assembles byte-for-byte identically to
it. It needs git, make and a C++17 compiler, and is not part of `make test`. Set
`SJASMPLUS=/path/to/sjasmplus` to use an existing binary instead of building one.
The base Z80 set is also checked against pasmo via
`tools/check_instr_coverage.sh`.

## Versioning

The project version lives in `VERSION` and is mirrored into
`pkg/version/version.go`. Keep them in sync with:

```
./syncver.sh set 0.2.0
./syncver.sh check
```

## Documentation

- [`docs/MANUAL.md`](docs/MANUAL.md) - the command line, source language, directives, conditionals, build tags, INCLUDE/INCBIN, Z80N, and character sets.
- [`docs/INSTRUCTION_SET.md`](docs/INSTRUCTION_SET.md) - instruction coverage by family.
- [`docs/Z80N_REFERENCE.md`](docs/Z80N_REFERENCE.md) - Z80N opcode encodings.

`zenas help` lists the common options; `zenas help --all` is the full reference.

## Licence

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
