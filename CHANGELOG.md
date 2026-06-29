# Changelog

All notable changes to zenas are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.7.0] - 2026-06-29

### Added

- **`build` command**: assembles the source and packages the result into
  loadable artifacts, complementing `assemble` (which emits a raw binary).
  Outputs are selected with `--tap`, `--tzx`, `--sna`, and `--z80`:
  - Snapshots (`.sna`, `.z80` v3) carry full machine state and run immediately
    at the entry point set by `--start`. The snapshot is overlaid on the booted
    state of the model chosen with `--model` (48k, 128k, plus2, plus2a, plus3).
  - Tape images (`.tap`, `.tzx`) encode the code as a CODE block. `--loader`
    prepends a BASIC auto-run loader so `LOAD ""` runs the code; the loader is
    generated as ASCII BASIC and tokenised via the zentools tokeniser.
  - The recommended workflow is to use `.z80`/`.sna` snapshots for development
    testing and tapes for wider distribution.
- `--start` accepts an address (`0x8000`, `$8000`, `32768`) or a source label;
  `--sp` overrides the stack pointer (default `0xFF00`) with a collision warning
  when it would sit inside or just above the code; `-o` sets the output basename.
- Spectrum tape names, +3DOS 8.3 names, and host filenames are handled as three
  separate namespaces rather than being conflated.

### Changed

- Adds a direct dependency on `github.com/ha1tch/zentools` for the tape,
  snapshot, and BASIC tokenisation formats.

## [0.6.3] - 2026-06-28

### Changed

- Cleaned up the examples directory. Removed six developer scratch files
  (`simple_test*.asm`, `debug_test.asm`, `debug_main_only.asm`) whose `_test.asm`
  names collided with the test-file convention and which no longer assembled.
  Rewrote stale and developer-facing comments across the remaining examples to be
  accurate and user-facing (e.g. C-style examples no longer carry "may not work
  yet" notes for features that now work, and instruction examples no longer use
  internal "Test Pass N" headers).

## [0.6.2] - 2026-06-28

### Added

- Radix-input data forms for `.DB`/`.DW`/`.DM`. A `0x` (hex) or `0d` (decimal)
  marker introduces values the directive emits by its own width and order:
  `.DB 0x600DF00D` and `.DB 0x 60 0D F0 0D` give the bytes `60 0D F0 0D`;
  `.DW 0x 600D F00D` gives little-endian words; `.DB 0d 96 13 240 13` is the
  decimal equivalent. A group that does not fit the directive's unit is an error
  rather than being silently re-split. Useful for entering binary blobs.
- `.MATCH <location>, <data>` test directive: asserts that a span of memory at an
  address or symbol matches the bytes produced by an ordinary `.DB`/`.DW`/`.DM`
  data operand (which is assembled through the real data path, so all data forms
  work). Reports the first differing offset on failure. Test-only.

### Changed

- Test discovery now keys on the `test_` label prefix. Every `test_*` routine is
  a test and must carry at least one `.EXPECT`/`.MATCH`; one with none fails. An
  `.EXPECT`/`.MATCH` on a non-`test_` label is an assembly error. A `*_test.asm`
  file with no `test_` routines emits a warning (and passes).
- A test routine may carry several `.EXPECT`/`.MATCH` directives, all checked as
  one test.

## [0.6.1] - 2026-06-28

### Added

- Go-style test files. A file named `*_test.asm` may include the program under
  test, define `test_*`-style routines, and annotate each with an `.EXPECT`
  directive stating expected post-conditions. `zenas assert file_test.asm` (with
  no `--expect`) discovers every annotated routine, runs each via its label,
  checks its expectations, and reports go-test-style PASS/FAIL with a summary and
  a non-zero exit on failure.
- `.EXPECT` directive: binds to the nearest preceding label, emits no bytes, and
  uses the same assertion vocabulary as `--expect` (registers, flags, memory). It
  is only permitted in a `*_test.asm` file - an assembly error elsewhere - so
  test expectations can never appear in a production build.
- `AssemblyResult.Tests` exposes the collected test specifications.

### Fixed

- `run` and `assert` now resolve `INCLUDE` paths relative to the input file
  (previously only relative to the working directory).
- `run` and `assert` no longer panic when assembly fails during include
  expansion; the error is reported cleanly.

## [0.6.0] - 2026-06-28

### Added

- In-process execution of assembled code via a bundled Z80/Z80N emulator core
  (github.com/ha1tch/zen80), with no separate emulator or file round-trip.
- `zenas run <file>`: assemble and execute, reporting final CPU and memory state.
  Options: `--max-steps=N` (instruction cap / infinite-loop guard, default
  1,000,000), `--call=<label>` (run a subroutine and stop on return),
  `--preload=ADDR,FILE` (load a binary into memory before running; repeatable),
  `--trace` (print each instruction as it runs), `--hex` and `--dump=START:LEN`
  (memory dump), `--json[=basic|standard|detailed|full]`, and `--next` (Z80N).
- `zenas assert <file> --expect "..."`: run, then check final state and exit
  non-zero on failure. Checks cover registers (`A`, `BC`, `HL`, `AF`, the shadow
  pairs `AF_`/`BC_`/`DE_`/`HL_`, `IX`, `IY`, `SP`, `PC`, and the 8-bit
  registers), flags (`CF` `ZF` `SF` `HF` `PF` `NF`), and memory bytes
  (`(0xC000)=0x42`). A run that hits the instruction cap fails as a test.
- Registers start zeroed for reproducible runs.
- `AssemblyResult.Origin` exposes the first ORG (the load address).

### Documentation

- New `docs/RUNTIME.md` guide for the run/assert features, and a manual section.

### Notes

- This adds zenas's first external dependency (github.com/ha1tch/zen80 v0.1.0).

## [0.5.2] - 2026-06-28

### Added

- Pasmo dialect: `#` hexadecimal prefix (`#80` == `0x80`), alongside the existing
  `&`, `0x`, and `h`-suffix forms.
- Bare unquoted `INCLUDE` filenames (`INCLUDE if.asm`), the pasmo style, in
  addition to the quoted form. Works in any dialect.
- Pasmo dialect: `@` is accepted within identifiers.

### Changed

- Pasmo no-colon labels are now recognised at any indentation, not only column 1,
  matching pasmo (the first identifier on a line is a label unless it is a
  mnemonic, directive, or known macro name). This also makes indented pasmo
  source (e.g. tab-indented labels) assemble.

### Notes

- Measured pasmo sample-suite coverage rose from 6/34 to 7/34. The remaining
  failures are dominated by the `PROC`/`LOCAL`/`PUBLIC` scoping subsystem and the
  `?:` ternary operator, which are larger features deliberately not included here.
  See docs/DIALECT_COMPATIBILITY.md.

## [0.5.1] - 2026-06-28

### Added

- Character literals usable in expressions. A single-character literal such as
  `'A'` may now appear as a term in an arithmetic expression (`DEFB 'A'+1`,
  `'Z'-'A'`), evaluating to the character code. This works in both native and
  pasmo dialects and applies to instruction operands and data directives alike.
  Multi-character strings remain string literals.
- Pasmo dialect: column-1 labels without colons. In `.pasmo` mode an identifier
  starting in column 1 is treated as a label even without a trailing colon (e.g.
  `loop    NOP`); a column-1 instruction mnemonic or directive is unaffected,
  matching pasmo. Native mode is unchanged.
- Source dialect modes. A `.pasmo` directive switches the lexer into the pasmo
  dialect for the source that follows, and `.zenas` switches back; the switch is
  scoped and may be repeated. In the pasmo dialect `$` is the location counter
  (current address) - while `$8000` is still a hex literal - and `DEFM` is
  accepted as a string/byte directive. Because includes are spliced as text
  before lexing, wrapping an `INCLUDE` in `.pasmo`/`.zenas` assembles a legacy
  pasmo include in place without converting it. The dialect mechanism is built
  general (a `Dialect` lexer state), so further dialects can be added beside it.
- Tests for the location counter, DEFM, and dialect return-to-native.

### Notes

- The pasmo dialect currently covers the location counter and DEFM; no-colon
  labels, pasmo macro syntax, and PROC/LOCAL scoping are planned (see
  docs/PASMO_DIALECT_DESIGN.md).

## [0.5.0] - 2026-06-28

### Added

- Package concept for macros. A `.PACKAGE name` directive groups subsequently
  defined macros, so two libraries may each define a macro of the same name.
  Calls may be qualified (`math.add`); an unqualified call resolves only when one
  package defines the name and is an error (with a "qualify it" message) when
  more than one does. A real instruction is never shadowed by a macro of the same
  name - `ADD` is always the instruction, and a macro `add` is reached as
  `package.add`. This resolves the macro-versus-instruction name collision.
  Internally the macro is keyed by a structured `(package, name)` pair, so the
  package can later become the unit a per-architecture tier is selected in.
- Tests for qualified disambiguation, macro/instruction coexistence, ambiguous
  bare call (error), and unambiguous bare call.

## [0.4.2] - 2026-06-28

### Fixed

- C-style functions can now be written on a single physical line (e.g.
  `void f() { asm { LD A,1; } }`). Previously the C statement terminator `;` was
  lexed as a Z80 comment, swallowing the closing braces and the rest of the line.
  In C-style mode `;` is now a statement terminator inside a brace-delimited block
  and remains a comment at file level, so `;`-style header comments still work.

### Added

- C-style return-width contract. A function's return type fixes its return width:
  `return <expr>;` checks that the value's width matches the declared return
  width and emits `RET`. Returning a value from a `void` function, a bare
  `return;` in a typed function, and falling off the end of a typed function with
  no `return` are all errors - the declared width must be delivered. Value
  placement remains the primitive tier's concern; zenas emits the `RET` and
  enforces the width only.
- Built-in tests for the return contract (width match, width mismatch, value from
  void, missing return, void bare return).

### Added

- A built-in test for a single-line C-style function and call.

## [0.4.1] - 2026-06-28

### Added

- Parameter width markers on macros, shared by traditional and C-style. A
  parameter may carry `uint8_t`/`uint16_t` (aliases `uint8`/`byte`,
  `uint16`/`word`): `MACRO STORE(uint16_t addr, uint8_t value)`. The marker is a
  size-compatibility contract on the signature, not a typed variable.
- Width-signature checking. When a parameter declares a width, a literal
  argument of a different width is an error in either direction - a wider
  argument would be truncated, a narrower one would not fill the promised width.
  Arguments whose width is unknowable (symbols, expressions) are trusted; untyped
  parameters are not checked. This makes the C-style `uint8_t`-style markers
  load-bearing rather than cosmetic.
- Macro tests for width-marker match, width mismatch (error), and untyped pass-through.

## [0.4.0] - 2026-06-28

### Fixed

- Macro bodies can now contain nested macro calls with multiple (or zero)
  arguments. Previously a body line like `INNER(a, b)` failed to parse because
  the body parser treated `(a, b)` as an instruction operand. The body parser is
  now macro-aware: a call to a known macro is parsed as an argument list. This
  also makes C-style functions with more than one parameter work (they transpile
  to nested traditional macro calls), including the previously-failing
  `examples/example14-cstyle-involved.asm`.

### Added

- Macro tests for nested multi-argument and nested zero-argument calls.
- A "Macros" section in `docs/MANUAL.md` covering definitions, calls, nesting,
  local-label uniqueness, the parameter-name/register collision caveat, and the
  scope of the C-style mode.

### Notes

- C-style macros remain structured-assembly sugar over traditional macros, with
  parameters passed by textual substitution. A real calling convention was
  assessed and declined (see `docs/MACRO_STATUS.md`); z88dk is the mature option
  for compiling C to Z80 (see `docs/Z88DK_REUSE.md`).

## [0.3.0] - 2026-06-28

### Fixed

- Traditional macros now accept multi-argument and zero-argument calls. Calls
  like `TWO(5, 6)` and `Z()` previously failed because the call site was parsed
  as an instruction operand, where `(a, b)` and `()` are not valid; single-
  argument calls also produced wrong bytes. Traditional macro calls are now
  intercepted at the token level and parsed as proper argument lists.
- Local labels inside a macro body are now unique per expansion. A macro
  containing a loop label, called more than once, previously produced a corrupt
  relative-jump displacement; each expansion's labels now resolve to their own
  addresses. (Two causes: a label pre-definition to 0 that broke jump
  arithmetic, and a unique-label counter that diverged between assembly passes.)
- The C-style macro converter no longer prints debug output. It previously
  emitted hundreds of `[DEBUG]` lines on every C-style assembly.

### Added

- Built-in macro tests (`zenas test`): single-argument, multi-argument,
  zero-argument, and local-label-uniqueness cases.

### Notes

- These fixes cover Phase 1 of the macro work (traditional macros). C-style
  function *parameters* remain unimplemented; C-style support is limited to
  parameterless functions. See `docs/MACRO_STATUS.md`.

## [0.2.2] - 2026-06-28

### Added

- `INCBIN "file"[,skip[,length]]` directive: inserts the raw bytes of a binary
  file at the current address, with optional skip (offset) and length (slice),
  resolved relative to the including file like INCLUDE. Output matches sjasmplus
  byte-for-byte. Out-of-range skip/length and missing files are clear errors.
- `--tag NAME` flag: selects a build tag, the way Go build tags select variants.
  Each tag defines `ZENAS_TAG_NAME` (presence) and `ZENAS_TAGBIT_NAME` (its bit,
  assigned in command-line order), and contributes to the composite bitmask
  `ZENAS_TAGS`. `ZENAS_TAGS` is always defined (0 with no tags), so source
  referencing it compiles in every configuration. Up to 16 distinct tags occupy
  bits. Accepts `--tag NAME` and `--tag=NAME`; tags compose.
- `IF` conditions now support the boolean operators `AND`, `OR`, `NOT` and
  parentheses (with short-circuit evaluation), scoped to `IF` so they do not
  affect operand or `DB` expressions. Existing `IF`/`IFDEF`/`IFNDEF` forms are
  unchanged.
- `examples/example-incbin-tags.asm` demonstrating INCBIN and composable tags.

### Changed

- `zenas help` is now concise (core usage and common options); the full option,
  charset and output reference moved to `zenas help --all`. Removed dead help
  code and corrected stale usage strings.
- Added [`docs/MANUAL.md`](docs/MANUAL.md), a reference for the command line,
  source language, directives, conditionals, build tags, INCLUDE/INCBIN, Z80N,
  and character sets.

## [0.2.1] - 2026-06-28

### Changed

- Added `tools/sjasmplus_compare.sh` and a `make test-sjasmplus` target: an
  optional cross-assembler check that builds sjasmplus from source and verifies
  every documented, half-register, and Z80N form byte-for-byte against it.
- Documentation refreshed to the current coverage: 468 documented Z80 + 48
  undocumented IX/IY half-register + 29 Z80N instructions (`docs/INSTRUCTION_SET.md`,
  README). The stale "core subset / in progress" status was corrected.
- The Z80N coverage checker (`tools/check_z80n_coverage.sh`) now uses **sjasmplus**
  as a live oracle when it is installed, assembling each instruction under
  `DEVICE ZXSPECTRUMNEXT` and comparing byte-for-byte against `zenas --next`;
  it falls back to the baked-in reference bytes otherwise. All 31 Z80N forms match
  sjasmplus, as do all 516 base-Z80 forms - so zenas now agrees with two
  independent reference assemblers (pasmo and sjasmplus).

## [0.2.0] - 2026-06-28

### Added

- Z80N (ZX Spectrum Next) instruction set, off by default and enabled with the
  `--next` flag or its alias `--cpu=Z80N` (the default is `--cpu=Z80`). Covers all
  31 Next-specific forms - SWAPNIB, MIRROR, TEST, the barrel-shift group
  (BSLA/BSRA/BSRL/BSRF/BRLC), MUL, ADD rr,A / ADD rr,nn, PUSH nn (big-endian
  operand), OUTINB, NEXTREG (both forms), PIXELDN/PIXELAD/SETAE, JP (C), and the
  DMA block-copy group (LDIX/LDWS/LDDX/LDIRX/LDPIRX/LDDRX). Opcodes are taken from
  the official SpecNext reference; see docs/Z80N_REFERENCE.md. A dedicated
  coverage checker (tools/check_z80n_coverage.sh) verifies all encodings.

### Changed

- When several instruction encodings match the same operand types (e.g. JP (HL)
  and the Z80N JP (C)), the encoder now tries each in turn and only reports an
  error if none apply, rather than failing on the first.

## [0.1.2] - 2026-06-28

### Fixed

- Illegal undocumented half-register combinations are now rejected instead of
  silently emitting a different instruction: mixing an IX half with an IY half
  (`LD IXH,IYL`) and combining a half register with an indexed memory operand
  (`LD (IX+d),IXH`). All legal `IXH`/`IXL`/`IYH`/`IYL` forms continue to encode
  correctly. The instruction-coverage check now also exercises the half-register
  set (516 forms, all matching the reference assembler).

## [0.1.1] - 2026-06-28

### Added

- Conditional `CALL cc,nn` (all eight conditions), `JP (HL)` / `JP (IX)` /
  `JP (IY)`, `LD (IX+d),n` / `LD (IY+d),n`, the interrupt/refresh register loads
  `LD A,I` / `LD A,R` / `LD I,A` / `LD R,A`, and the block-I/O group
  `INI/IND/INIR/INDR/OUTI/OUTD/OTIR/OTDR`.
- IX/IY index displacements now accept symbol expressions (`LD A,(IX+OFFSET)`,
  `(IX+N*2)`), evaluated at assembly time, and are range-checked to a signed byte.
- `docs/INSTRUCTION_SET.md` coverage table and a reproducible checker
  (`tools/check_instr_coverage.sh` + `tools/gen_instr_coverage.py`) that assembles
  a representative of every Z80 instruction family and diffs the bytes against a
  reference assembler. The full set (468 forms) matches byte-for-byte.

### Fixed

- `INC IX` / `DEC IX` / `INC IY` / `DEC IY` were emitted without the DD/FD prefix
  (encoding as `INC HL` / `DEC HL`), producing wrong machine code. Now prefixed.
- An out-of-range IX/IY displacement is now a clear error instead of silently
  wrapping to a signed byte.
- `LD A,I` / `LD A,R` / `LD I,A` / `LD R,A` previously mis-encoded as ordinary
  register transfers; they now emit the correct ED-prefixed opcodes.

## [0.1.0] - 2026-06-28

### Added

- Undocumented half-index registers IXH/IXL/IYH/IYL (e.g. `LD IXH,A`, `DEC IXH`),
  encoded as the H/L-form instruction with a DD/FD prefix.
- Accumulator rotates `RLCA`, `RRCA`, `RLA`, `RRA`.
- `DS` / `DEFS` (define-space) recognised as directives.
- Character literals in value contexts (`FONT_FIRST EQU '!'`) evaluate to the
  charset-mapped character code.
- Unary `+`/`-` in operand and directive expressions.

### Fixed

- **Relative jumps (`JR`, `JR cc`, `DJNZ`) computed a wrong displacement** - the
  target was left as an absolute address and truncated to one byte, so e.g.
  `JR NZ,loop` emitted `20 00` instead of the correct signed offset. The
  displacement is now `target - (address + 2)`, range-checked to -128..127.
- **`ORG` with a symbol argument** (`ORG KERNEL_ORG`) placed all code at a
  zero-based origin, because pass 1 returned a dummy 0 for every symbol including
  already-defined ones. Backward references now resolve to their real value in
  pass 1, so `ORG` and similar address math are correct.
- A bare `;` line (and `;` not followed by space/letter) is now a comment.
  Previously it produced a stray semicolon token and a parse error.

### Milestone

- zenas assembles the full **ZX Opal** kernel byte-identically to pasmo (6530
  bytes, all 400 symbols matching). This was the readiness acceptance test.

## [0.0.7] - 2026-06-28

### Added

- IX/IY indexed addressing. All forms used by real Z80 code, verified
  byte-identical to pasmo: `LD r,(IX+d)` / `LD (IX+d),r`, `LD IX,nn`, `ADD IX,rr`,
  `PUSH`/`POP IX`, `LD SP,IX`, and ALU/INC/DEC on indexed operands (`CP (IX+d)`,
  `INC (IX+d)`). IY variants use the FD prefix. Displacements accept constant
  expressions (`(IX+1+3)`).
- ALU and INC/DEC operations on `(HL)` (`ADD A,(HL)`, `AND (HL)`, `CP (HL)`,
  `OR (HL)`, `SUB (HL)`, `INC (HL)`, `DEC (HL)`, etc.), which were previously
  unimplemented and are the base form the indexed variants build on.

### Fixed

- `LD IX,nn` / `LD IY,nn` previously encoded silently as `LD HL,nn` (missing the
  DD/FD prefix), producing wrong machine code. They now emit the correct prefixed
  encoding.

## [0.0.6] - 2026-06-28

### Added

- Operand expressions: instruction operands now accept symbol arithmetic with the
  full `+ - * /` grammar and correct precedence, in three positions: immediate
  operands (`LD HL,kernel_end+255`, `ADD A,ATOM_HDR+7`, `LD HL,base+N*3`), indirect
  operands (`LD (rowcount+1),A`, `LD HL,(base+2)`), and IX/IY displacements
  (`LD D,(IX+1+3)`, folded to a constant at parse time). 8-bit immediate operands
  remain value-checked, so a symbol+offset that exceeds one byte is still rejected
  for an 8-bit slot. Directive arguments (`DW base+10`) already supported this.

## [0.0.5] - 2026-06-28

### Added

- Conditional assembly: `IF`, `IFDEF`, `IFNDEF`, `ELSE`, `ENDIF`, implemented with
  a per-pass condition stack (nesting supported). `IF` takes an expression (true
  when non-zero); `IFDEF`/`IFNDEF` test whether a symbol is defined. A symbol that
  is undefined in an `IF` expression evaluates to 0 (false) consistently in both
  passes, so branch selection never diverges between passes. An unterminated `IF`
  or a stray `ELSE`/`ENDIF` is reported as an error.
- `--define=NAME[=VALUE]` pre-defines a symbol before assembly (default value 1;
  VALUE accepts decimal, `0x` hex, or `$` hex). Combined with `IF`/`IFDEF`, this
  gives Go-build-tag-style selection driven from the command line - e.g. the
  multi-target build can choose a layout with `--define=TARGET_48K`.

## [0.0.4] - 2026-06-28

### Changed

- zenas is now case-sensitive for user symbols (labels and constants) and remains
  case-insensitive for instruction mnemonics, registers, condition codes, and
  directives - matching pasmo and the wider Z80 toolchain. Previously every
  identifier was folded to uppercase, which collapsed distinct mixed-case symbols
  (e.g. a runtime byte `line_h` and a compile-time constant `LINE_H`) into one and
  could silently miscompile. The symbol file now preserves original case.

## [0.0.3] - 2026-06-28

### Added

- Symbol-file output: `--sym` writes a pasmo-format `.sym` file (lines of the form
  `NAME EQU 0XXXXH`, sorted by name) alongside the binary; `--sym=path`
  sets an explicit path. Without the flag, no symbol file is written. This is the
  format the ZX Opal memory-map and release tooling parses.


### Changed

- Help output now leads with the version (`zenas <version> - Z80 Assembler ...`),
  and lists the `version` command. The `version` command and its `-v` / `--version`
  forms report the version string.

## [0.0.2] - 2026-06-28

### Added

- Instruction encoder coverage extended to close the gaps the ZX Opal source
  exercises. New / fixed forms, all verified byte-correct:
  - Block transfer/search: `LDIR`, `LDDR`, `LDI`, `LDD`, `CPIR`, `CPDR`, `CPI`,
    `CPD`.
  - Port I/O register form: `IN r,(C)`, `OUT (C),r` (the parser now recognises
    `(C)` as a port operand rather than an undefined symbol).
  - Exchange: `EX DE,HL`, `EX AF,AF'`, `EX (SP),HL`, `EXX`.
  - 16-bit arithmetic: `ADC HL,rr`, `SBC HL,rr`.
  - CB-prefixed `(HL)` forms: `BIT/RES/SET n,(HL)` and `RLC/RRC/RL/RR/SLA/SRA/SRL
    (HL)`.
  - Misc: `NEG`, `RETI`, `RETN`, `RRD`, `RLD`, `IM 0/1/2`, `RST n`.

### Fixed

- Lexer now accepts a trailing apostrophe on prime (shadow) register names
  (`AF'`), so `EX AF,AF'` lexes correctly. Ordinary character literals such as
  `'A'` are unaffected (the rule is restricted to register identifiers).

## [0.0.1] - 2026-06-28

First standalone release of zenas as its own repository, extracted from the zen80
project where the assembler was originally developed. The version line starts
fresh at 0.0.1 for the standalone project.

### Added

- Standalone Go module `github.com/ha1tch/zenas` (previously a subpackage of
  `github.com/ha1tch/zen80`).
- Project scaffolding in the house style: `VERSION`, `pkg/version/version.go`,
  `syncver.sh`, `Makefile`, and `release.sh`.
- `zenas version` / `--version` command, wired to the version package.
- `INCLUDE` directive (pasmo-style textual inclusion). Included files are spliced
  at the directive site before lexing, so forward references across the include
  boundary resolve. Paths resolve relative to the file containing the INCLUDE;
  nested includes resolve relative to their own file. Include cycles are detected
  and reported. Both bare `INCLUDE` and dotted `.INCLUDE` are accepted, with
  optional trailing comments. `AssembleFile` is now implemented and sets the
  include base directory automatically.
- Documentation: `docs/ZENAS_READINESS.md` (assembler readiness tracking) and
  `docs/ENCODER_STRATEGY.md` (the zen80 decode-algebra approach to the encoder).

### Fixed

- Bare (non-dotted) `NAME EQU value` now parses and binds correctly. Two fixes:
  the parser now recognises a label followed by a bare directive identifier (not
  only the dotted `.EQU` form), and the assembler binds the EQU value to the label
  for both `EQU` and `.EQU`. Previously `NAME EQU value` either failed to parse or
  bound the label to the location counter instead of the constant.

### Changed

- Module import path updated from `github.com/ha1tch/zen80/assembler` to
  `github.com/ha1tch/zenas/assembler`.
- Historical material (older source snapshots, the legacy macro parser) moved to
  `attic/`.

### Notes

- The assembler is functional for a core Z80 subset: two-pass assembly, forward
  references, a working traditional macro system, multiple number formats
  (`0x`, `%`, decimal), string and multi-value `DB`/`DW`, bare and dotted
  directives, `INCLUDE`, and a symbol table.
- Known gaps are tracked in `docs/ZENAS_READINESS.md`.
