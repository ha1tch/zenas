# Code architecture

This describes how zenas's own source is organised -- packages, files,
and how they fit together. For *why* the language looks the way it
does (the C-style seam, packages, portability), see
[the design rationale](ZENAS_DESIGN.md) instead; this document is
about the code, not the language it implements.

## Top level: the CLI

- **`main.go`** -- the `zenas` command itself: flag parsing and the
  `assemble`/`version`/`help` subcommands.
- **`build_cmd.go`** -- the `zenas build` subcommand. Where `assemble`
  produces a raw binary (or JSON), `build` wraps that output as a
  loadable ZX Spectrum artefact (tape, snapshot) via `pkg/build`.
- **`run.go`** -- the `zenas run`/`zenas assert` subcommands: executes
  assembled code against zen80 (via a Go module dependency, not
  vendored) and checks `.EXPECT`/`.MATCH` assertions from `*_test.asm`
  files.

## `assembler`: lexer, parser, macros, encoder

The core pipeline, in the order source actually flows through it:

1. **`lexer.go`** -- tokenises source text. Handles zenas's own
   number-literal notation (decimal, `$`/`&`/`0x`-hex, `%`/`B`-binary,
   character literals) and the traditional-assembler `H`-suffix hex
   form (`0BCH`) that needs to be disambiguated from a `0b`-prefixed
   binary literal or a `0d` radix marker -- see this file's own
   `looksLikeHexHSuffix` for the disambiguation and
   `CHANGELOG.md`'s `[0.9.0]` entry for the bug this fixed.
2. **`parser.go`** -- tokens into an AST: instructions, directives,
   labels, expressions.
3. **Macros** -- a family of files, not one: `macro_ast.go` (the
   shared AST node types), `macro_parser.go` (the notation-agnostic
   interface), `macro_traditional.go` (`MACRO name(...)` style),
   `macro_c_style.go` (the C-flavoured `.MACRO_STYLE C` syntax the
   design rationale calls "the seam"), `c_style_converter.go`
   (converts C-style macro tokens down to traditional-macro tokens, so
   expansion itself has one code path regardless of source notation),
   `macro_table.go` (definitions and signature validation),
   `macro_expander.go` (turns a macro call in the AST into the
   assembly it expands to).
4. **`include.go`** -- file inclusion, expanded once before lexing and
   before either assembly pass.
5. **`symbols.go`** -- the label/constant table used across both
   assembly passes.
6. **`encoder.go`** -- instruction templates
   (`mnemonic, operandTypes, baseOpcode, prefix, cycles, encodeFunc`)
   and the `encode*` functions that turn a resolved instruction into
   bytes. As of `[0.10.0]`, every one of these functions (57 total,
   plus the half-index-register post-processing step
   `applyHalfIndexPrefix`) is a thin adapter over `pkg/z80enc` --
   see that package's own section below. As of a full clock-count
   review against reference Z80 timing data, a template's `Cycles`
   field alone is wrong for any instruction whose real T-state count
   depends on which specific register the resolved operands use --
   `(HL)` vs `(IX+d)`/`(IY+d)` is the largest such family, but it also
   covers plain vs half-index 8-bit registers, `BC`/`DE`/`HL`/`SP` vs
   `IX`/`IY`, and `JP (HL)` vs `JP (IX)`/`(IY)`. `CyclesFunc` and its
   accessor `CyclesFor` exist for exactly these ~34 cases (the other
   ~120-odd templates never needed it and still don't); the encoder's
   own `EncodeWithCycles` is the only way to observe the real value
   from outside this package, since `Encode` itself only ever
   returned bytes.
7. **`assembler.go`** -- ties the above together: the two-pass
   assembly driver, directive handling (`.PACKAGE`, `.EXPECT`,
   conditional assembly, build tags), and the top-level `Assembler`
   type main.go and run.go both call into.

## `pkg/z80enc`: pure opcode-encoding knowledge

Added in `[0.9.0]`-`[0.10.0]`. Byte-level Z80 opcode arithmetic, taking
plain Go primitives (register names as strings, displacements as
`int8`, bit numbers as `uint8`) with zero dependency on `assembler`'s
own `ResolvedOperand` type or any other parser-specific representation.
`encoder.go`'s `encode*` functions are thin adapters over this package,
not duplicate logic -- the actual arithmetic lives here exactly once.

This split exists specifically so a fork with its own front end (a
different lexer/parser, e.g. one serving multiple target architectures
uniformly) can import the real opcode arithmetic directly instead of
re-deriving it from zenas's own release notes each time a fix lands --
see this package's own doc comment and `CHANGELOG.md`'s `[0.9.0]`
entry for the motivating case.

One file per instruction-shape family, each independently tested:

| File | Covers |
|------|--------|
| `registers.go` | Shared register/condition-code mappings (`Reg8`, `Reg16`, `StackReg16`, `Condition`, `JRCondition`, `IndexPrefix`) everything else builds on |
| `reg8.go` | 8-bit register operations: `INC`/`DEC r`, `LD r,n`, `LD r,r'`, `ALU r`, `CB r`, `BIT`/`RES`/`SET b,r` |
| `reg16.go` | 16-bit register-pair operations: `LD rp,nn`, `ADD HL,rp`, `INC`/`DEC rp`, `PUSH`/`POP rp`, `LD rp,(nn)`, `LD (nn),rp`, `ADC`/`SBC HL,rp` |
| `indirect.go` | `(HL)`/`(IX+d)`/`(IY+d)` addressing: `LD r,(...)`/`LD (...),r`, `LD (...),n`, ALU-on-indirect, `INC`/`DEC` on indirect |
| `branch.go` | Jump/call/branch: `JP`/`CALL` (with and without condition), `JR`/`DJNZ`, `JP` indirect, `RET cc`, `RST` |
| `ddfdcb.go` | The DDCB/FDCB group: rotate/shift and `BIT`/`RES`/`SET` on `(IX+d)`/`(IY+d)`, including the undocumented copy-to-register forms (`RLC (IX+d),B`) |
| `misc.go` | `EX`, `EX (SP),HL`, `IM`, `IN`/`OUT` (register- and immediate-port forms, including the undocumented bare `IN (C)`/`OUT (C),0`), half-index-register prefix handling, and the trivial fixed-opcode/`LD SP,HL` shapes |
| `z80n.go` | The whole Z80N (ZX Spectrum Next) extension set |

## `pkg/build`: ZX Spectrum output formats

Turns assembled machine code into loadable Spectrum artefacts --
`build.go` (tape/snapshot orchestration), `embed.go` (one real
booted-machine snapshot per model, embedded and decoded on demand as
the base a program's own bytes get written into), `loader.go`
(generates a BASIC loader for a CODE-only tape, since a real Spectrum
needs a BASIC program to `LOAD` and jump to raw code), `names.go`
(the three separate filename namespaces a build touches -- host
filesystem, tape block name, BASIC program name -- each with its own
normalisation rules).

## `pkg/version`

Trivial: the version string `syncver.sh` keeps in sync with `VERSION`.

## What's deliberately not here

The lexer/parser/macro layer is specific to zenas's own front end and
is not factored out the way `pkg/z80enc` is -- see that package's own
doc comment for why this is a boundary, not a gap: a fork with a
genuinely different front end (serving multiple target architectures
through one lexer, for instance) needs its own independent
verification of that layer regardless of how zenas structures its own.
