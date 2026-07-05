# zenas

A Z80 and Z80N macro assembler written in Go that can also **execute and test
the code it assembles**. It assembles Z80 source to raw machine code, packages
it directly into runnable ZX Spectrum tapes and snapshots, or runs it in a
built-in Z80 emulator and asserts on the result. No third-party assembler
dependencies; one static binary.

```sh
# the whole loop, no other tools:
zenas assemble game.asm game.bin          # bytes
zenas build game.asm --z80 --start main   # runnable snapshot
zenas assert math_test.asm                # run the test suite, exit non-zero on failure
```

Beyond the conventional core, zenas carries a small set of deliberate ideas —
a typed macro linkage layer, package namespacing, Go-style build tags and test
files — that together form a coherent design. New users should start with the
**[programming guide](docs/ZENAS_PROGRAMMING.md)**; the reasoning behind the
design is in **[the design rationale](docs/ZENAS_DESIGN.md)**.

## Commands

| Command          | Does                                                        |
| ---------------- | ----------------------------------------------------------- |
| `zenas assemble` | assemble source to a raw binary (or a JSON report)          |
| `zenas build`    | assemble and package into a tape or snapshot                |
| `zenas run`      | assemble and execute in the built-in Z80 emulator           |
| `zenas assert`   | execute and check the final machine state (CI-friendly)     |
| `zenas version`  | print the version                                           |
| `zenas help`     | list options; `help --all` for the full reference           |

### Usage: assemble

```
zenas assemble <input.asm> [output.bin] [--hex] [--json=LEVEL] [--charset=NAME]
              [--sym[=path]] [--define=NAME[=VAL]] [--tag NAME]
              [--next | --cpu=Z80|Z80N]
```

```
# assemble to a raw binary
zenas assemble game.asm game.bin

# enable the ZX Spectrum Next instruction set
zenas assemble game.asm --next

# conditional build with a defined symbol and two build tags
zenas assemble game.asm out.bin --define DEBUG --tag debug --tag plus3

# write a pasmo-format symbol file alongside the output
zenas assemble game.asm game.bin --sym

# emit a JSON report instead of a binary
zenas assemble game.asm --json=detailed
```

`--next` (or `--cpu=Z80N`) turns on the Z80N extensions; the default is `Z80`.
`--define` pre-sets a symbol for `IF`/`IFDEF`. `--tag` selects a build tag, the
way Go build tags select variants: each tag defines `ZENAS_TAG_NAME`,
`ZENAS_TAGBIT_NAME`, and contributes to the composite `ZENAS_TAGS` bitmask, so
tags compose in `IF` conditions with `AND`/`OR`/`NOT`. `--charset` picks the
string encoding (Spectrum, MSX, CPC and regional variants; see `help --all`).

### Usage: build

```
zenas build <input.asm> [--tap] [--tzx] [--sna] [--z80] [--loader]
            [--start <addr|symbol>] [--sp <addr>]
            [--model 48k|128k|plus2|plus2a|plus3] [-o <basename>]
```

```
# snapshot for development testing — loads and runs in one step
zenas build game.asm --z80 --start main --model 128k

# tapes for distribution, with a BASIC auto-run loader
zenas build game.asm --tap --tzx --loader --start main
```

`build` assembles the source and packages it into loadable artifacts. Snapshots
(`--sna`, `--z80`) carry a full machine state, so the code runs immediately at
`--start`; tapes (`--tap`, `--tzx`) encode the code as a CODE block, and
`--loader` prepends a BASIC loader so `LOAD ""` runs it. `--start` (an address
or a source label) is required for snapshots and for `--loader`; `--sp` overrides
the stack pointer; `--model` picks the machine a snapshot is overlaid on; `-o`
sets the output basename. Recommended workflow: snapshots while iterating, tapes
to ship. Examples are in [`examples/`](examples/).

### Usage: run and test

zenas can execute the code it assembles, in a built-in Z80 emulator, and assert
on the result — no separate emulator, no file round-trip.

```
# run a routine and watch it execute
zenas run game.asm --call=main --trace

# assert a routine's contract (exits non-zero on failure, so it fits CI)
zenas assert math.asm --call=multiply --expect="A=0x0C,CF=0"

# run a whole test suite: a *_test.asm file with test_* routines and .EXPECT
zenas assert math_test.asm
```

`run` executes the assembled code and reports the final CPU and memory state;
`--trace` shows each instruction, `--dump=START:LEN` dumps memory, and
`--preload=ADDR,FILE` loads input data first. `assert` adds `--expect` checks
over registers, flags, and memory bytes. A `*_test.asm` file with `test_*`
routines each followed by an `.EXPECT` directive runs go-test style — and
`.EXPECT`/`.MATCH` are legal *only* in `*_test.asm` files, so test metadata can
never leak into a shipped binary. See [`docs/RUNTIME.md`](docs/RUNTIME.md) for
the complete run/assert reference.

## Install

```sh
go install github.com/ha1tch/zenas@latest
```

Or build from a checkout:

```sh
make build         # -> build/zenas
make test          # run the Go test suite
make smoke         # assemble the bundled examples as a sanity check
```

### Requirements

- Go 1.25 or later (build from source only; the released binaries have no
  dependencies)

## What it assembles

zenas assembles complete Z80 programs and matches pasmo byte-for-byte on real
operating-system source — a multi-kilobyte kernel with many included subsystems
assembles to an identical binary, with every symbol address matching. The
instruction set covers three classes, each cross-checked against an established
reference assembler:

- the **468** documented Z80 instructions (match pasmo and sjasmplus);
- the **48** undocumented IX/IY half-register forms (match pasmo and sjasmplus),
  with the illegal combinations rejected;
- the **29** Z80N (ZX Spectrum Next) extensions (match sjasmplus), off by
  default, enabled with `--next`.

See [`docs/INSTRUCTION_SET.md`](docs/INSTRUCTION_SET.md) for the coverage
detail, including the scripts that reproduce the verification.

A note on scope: the byte-for-byte match covers the shared instruction and
data-directive core. Source that leans on pasmo's or sjasmplus's own
directive layers (macro systems, `DEVICE`, structs) is a different matter —
the measured state of that compatibility, gap by gap, is documented in
[`docs/DIALECT_COMPATIBILITY.md`](docs/DIALECT_COMPATIBILITY.md).

## Documentation

- **[Programming guide](docs/ZENAS_PROGRAMMING.md)** — coming from another
  assembler: what's familiar, what differs, and what zenas uniquely enables.
- **[Design rationale](docs/ZENAS_DESIGN.md)** — the single idea behind
  the unusual features, and what the C-style layer is (and is not) for.
- **[Seam comparison](docs/SEAM_COMPARISON.md)** — how far other assemblers'
  facilities go towards the same design, as a set of reference points.
- **[Packaged-program tutorial](docs/PACKAGED_PROGRAM_TUTORIAL.md)** — a worked
  walkthrough building and testing a program with packages, `.EXPECT`, `.MATCH`.
- **[Manual](docs/MANUAL.md)** — full command line, language, and directives.
- **[Runtime reference](docs/RUNTIME.md)** — running and testing code with
  `run` and `assert`.
- **[Instruction set](docs/INSTRUCTION_SET.md)** — coverage by family, with
  reproducible verification.
- **[Z80N reference](docs/Z80N_REFERENCE.md)** — Z80N opcode encodings.
- **[Dialect compatibility](docs/DIALECT_COMPATIBILITY.md)** — the measured
  state of pasmo and sjasmplus source compatibility.

`zenas help` lists the common options; `zenas help --all` is the full reference.

## Licence

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).

Copyright (c) 2026 haitch <h@ual.li>
