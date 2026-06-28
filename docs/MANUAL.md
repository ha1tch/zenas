# zenas manual

zenas is a two-pass Z80 / Z80N assembler. This manual is the reference for the
command line, the source language, the directives, and the extensions. For the
instruction-set coverage table see [INSTRUCTION_SET.md](INSTRUCTION_SET.md); for
the Z80N opcode details see [Z80N_REFERENCE.md](Z80N_REFERENCE.md).

## Contents

- [Running zenas](#running-zenas)
- [Source format](#source-format)
- [Numbers and character literals](#numbers-and-character-literals)
- [Symbols and expressions](#symbols-and-expressions)
- [Directives](#directives)
- [Conditional assembly](#conditional-assembly)
- [Build tags](#build-tags)
- [Including files](#including-files)
- [Macros](#macros)
- [Packages](#packages)
- [Dialect modes](#dialect-modes)
- [Z80N (ZX Spectrum Next)](#z80n-zx-spectrum-next)
- [Character sets](#character-sets)
- [Output and reports](#output-and-reports)
- [Running and testing assembled code](#running-and-testing-assembled-code)

## Running zenas

```
zenas assemble <input.asm> [output.bin] [options]
zenas version
zenas help [--all]
```

If no output file is given, the input name is used with a `.bin` extension.
`zenas help` shows the common options; `zenas help --all` shows the full
reference.

Options:

| Option | Effect |
|--------|--------|
| `--next` | Enable the Z80N (ZX Spectrum Next) instruction set |
| `--cpu=Z80\|Z80N` | Select the CPU target (default `Z80`); `--cpu=Z80N` is the same as `--next` |
| `--define=NAME[=VAL]` | Pre-define a symbol (default value 1); drives `IF`/`IFDEF` |
| `--tag NAME` | Select a build tag (see [Build tags](#build-tags)) |
| `--sym[=path]` | Write a pasmo-format symbol file (default `<output>.sym`) |
| `--hex` / `--no-hex` | Force or suppress the hex dump of the output |
| `--charset=NAME` | Character encoding for strings (see [Character sets](#character-sets)) |
| `--no-warnings` | Suppress character-replacement warnings |
| `--json=LEVEL` | Emit a JSON report instead of a binary (see [Output and reports](#output-and-reports)) |

## Source format

One statement per line. A line may have an optional label, an instruction or
directive, and an optional comment:

```
label:          LD      A,5         ; a comment
```

- **Labels** end with a colon, or begin a line in the first column. They are
  **case-sensitive** (`Loop` and `loop` are different).
- **Mnemonics, register names and directive names** are case-insensitive
  (`ld a,b` and `LD A,B` are the same).
- **Comments** start with `;` and run to the end of the line.

## Numbers and character literals

| Form | Example | Notes |
|------|---------|-------|
| Decimal | `42`, `-1` | |
| Hex, `$` prefix | `$2A` | |
| Hex, `&` prefix | `&FF` | ZX-BASIC style; `&` is **not** a bitwise operator here |
| Hex, `H` suffix | `2AH` | Must start with a digit (`0FFH`, not `FFH`) |
| Binary, `%` prefix | `%101010` | |
| Binary, `B` suffix | `101010B` | |
| Character | `'A'` | Evaluates to the character code in the active charset |

### Radix-input data forms

In `.DB`, `.DW`, and `.DM`, a `0x` (hex) or `0d` (decimal) marker introduces a
run of values that the directive emits according to its own width and order. The
marker may be glued to a run of digits, or stand alone before space-separated
groups:

```
        DB 0x600DF00D            ; 60 0D F0 0D  (bytes, written order)
        DB 0x 60 0D F0 0D        ; 60 0D F0 0D  (spaced hex)
        DW 0x 600D F00D          ; 0D 60 0D F0  (words, little-endian)
        DB 0d 96 13 240 13       ; 60 0D F0 0D  (decimal)
```

`.DB`/`.DM` take byte-sized groups; `.DW` takes word-sized groups (little-endian).
A group whose size or value does not fit the directive's unit is an error rather
than being silently re-split. These forms are convenient for entering binary
blobs (sprite data, tables, captured dumps).

## Symbols and expressions

Define a constant with `EQU`:

```
WIDTH           EQU     32
HEIGHT          EQU     24
SIZE            EQU     WIDTH*HEIGHT
```

Operands and `EQU`/`DB`/`DW` values accept arithmetic expressions with `+`, `-`,
`*`, `/` and parentheses, and may reference symbols (including forward
references, resolved on the second pass):

```
                LD      HL,buffer+WIDTH
                LD      A,(IX+OFFSET)
                DW      table, table+2, table+4
```

Index displacements (`(IX+d)` / `(IY+d)`) accept the same expressions and are
range-checked to a signed byte at assembly time.

## Directives

| Directive | Purpose |
|-----------|---------|
| `ORG addr` | Set the assembly origin. `addr` may be a symbol or expression. |
| `EQU value` | Define a constant (`NAME EQU value`). |
| `DB` / `DEFB` | Define bytes. Accepts numbers, expressions, and quoted strings. |
| `DW` / `DEFW` | Define 16-bit words, little-endian. |
| `DS` / `DEFS count[,fill]` | Reserve `count` bytes, filled with `fill` (default 0). |
| `INCLUDE "file"` | Textually include another source file. |
| `INCBIN "file"[,skip[,length]]` | Embed the raw bytes of a binary file. |
| `IF` / `IFDEF` / `IFNDEF` / `ELSE` / `ENDIF` | Conditional assembly. |
| `END` | Marks the end of the source (optional). |

`DB` mixes strings and values freely:

```
message:        DB      "SCORE: ", 0
row:            DB      WIDTH, HEIGHT, $FF
```

## Conditional assembly

`IFDEF`/`IFNDEF` test whether a symbol is defined; `IF` tests whether an
expression is non-zero:

```
                IFDEF   DEBUG
                CALL    dump_state
                ENDIF

                IF      FEATURE_SOUND
                CALL    init_sound
                ENDIF
```

`IF` evaluates an arithmetic expression (`+`, `-`, `*`, `/`, parentheses, and
symbols) and treats any non-zero result as true; an undefined symbol counts as
0. On top of that, `IF` conditions support the boolean operators `AND`, `OR`,
`NOT` with short-circuit evaluation:

```
                IF      DEBUG AND NOT RELEASE
                IF      TARGET_128 OR TARGET_PLUS3
                IF      A AND (B OR C)
```

These boolean operators are recognised only inside `IF`; they are not part of
ordinary operand or `DB` expressions. Symbols are pre-defined from the command
line with `--define` or `--tag`.

## Build tags

`--tag NAME` selects a build tag, the way Go build tags select variants. Each
tag defines three things:

| Symbol | Value |
|--------|-------|
| `ZENAS_TAG_NAME` | `1` — a presence flag, for `IFDEF` / `IF` / `AND` / `OR` / `NOT` |
| `ZENAS_TAGBIT_NAME` | `1 << bit` — this tag's bit, assigned in command-line order |
| `ZENAS_TAGS` | the OR of every set tag's bit |

`ZENAS_TAGS` is **always defined** (0 when no tags are given), so source that
references it compiles in every configuration. Up to 16 distinct tags occupy
bits. Tag names must be valid identifiers.

```
zenas assemble game.asm out.bin --tag debug --tag plus3
```

```
                IFDEF   ZENAS_TAG_debug
                DB      $DE, $AD            ; debug-only
                ENDIF

                IF      ZENAS_TAG_debug AND ZENAS_TAG_plus3
                DB      $D3
                ENDIF

build_stamp:    DB      ZENAS_TAGS         ; embed the combined tag bitmask
```

`--tag NAME` is equivalent to `--define ZENAS_TAG_NAME=1` plus the bit and mask
symbols. Both `--tag NAME` and `--tag=NAME` are accepted, and multiple tags
compose.

## Including files

`INCLUDE "file"` performs textual inclusion: the file's source replaces the
directive in place, so labels defined in an included file are visible to the
including file and vice versa, including forward references across the boundary.
Paths are resolved relative to the file that contains the `INCLUDE`.

`INCBIN "file"[,skip[,length]]` embeds the raw bytes of a binary file at the
current address:

```
sprite:         INCBIN  "sprite.bin"          ; the whole file
tail:           INCBIN  "sprite.bin",16        ; skip the first 16 bytes
head:           INCBIN  "sprite.bin",0,8       ; the first 8 bytes
```

`skip` is an offset into the file; `length` limits how many bytes are inserted.
Both are optional and may be expressions. An out-of-range `skip` or `length`, or
a missing file, is an error. Paths resolve relative to the including file.

## Macros

Macros are enabled by selecting a style with `.MACRO_STYLE TRADITIONAL` (or
`.MACRO_STYLE C`). The traditional style is the recommended one.

A traditional macro is defined with `MACRO name(params)` ... `ENDMACRO` and
called as `name(args)`:

```
.MACRO_STYLE TRADITIONAL

MACRO LOAD16(reg, value)
    LD reg, value
ENDMACRO

MACRO DELAY(count)
    LD B, count
loop:
    DJNZ loop
ENDMACRO

        LOAD16(HL, $4000)
        DELAY(20)
        DELAY(50)
```

Each argument is substituted textually wherever its parameter name appears in
the body. Macros may take zero, one, or several arguments, and a macro body may
call other macros (including with multiple arguments), to any depth.

**Local labels** defined inside a macro body (like `loop:` above) are made
unique on every expansion, so a macro containing a loop can be called more than
once without the labels colliding.

**Width markers.** A parameter may carry an optional width marker - `uint8_t` (or
`uint8`, `byte`) for an 8-bit parameter, `uint16_t` (or `uint16`, `word`) for a
16-bit one:

```
MACRO STORE(uint16_t addr, uint8_t value)
    LD HL, addr
    LD (HL), value
ENDMACRO
```

The marker is a size-compatibility contract on the signature, not a typed
variable: it records how wide the parameter is, not where it lives. When a
parameter declares a width, the argument's width must match it exactly. Passing
an argument of the wrong width is an error, in either direction - a wider
argument would be silently truncated, a narrower one would not fill the width the
signature promises:

```
        STORE($8000, 5)     ; ok: 16-bit and 8-bit arguments
        STORE(5, 5)         ; error: 8-bit argument for a 16-bit parameter
```

Only arguments whose width is knowable are checked - literal numbers (a value up
to 255 is 8-bit, larger is 16-bit). Symbols and expressions, whose width cannot
be determined at assembly time, are trusted and not checked. A parameter with no
marker is untyped and never width-checked. Width markers work the same way in
both traditional and C-style macros.

**Naming caveat.** Because substitution is textual, a parameter named the same as
a register or condition code is shadowed by it. `MACRO M(a)` with a body line
`LD A, a` assembles as `LD A, A`, not `LD A, <argument>`. Avoid single-letter
parameter names that clash with the registers `A B C D E H L` or the condition
codes `Z C NC NZ P M PE PO`; use descriptive names such as `value`, `count`, or
`reg`.

A C-like style is also available via `.MACRO_STYLE C`, which transpiles C-shaped
function definitions to traditional macros. It is structured-assembly sugar - a
way to group `asm { ... }` blocks into named, callable units with simple
parameter substitution - not a C compiler. It does not implement a calling
convention (register or stack argument passing, return values). For compiling C
to Z80, use z88dk.

In C-style source, `;` is a statement terminator inside a `{ ... }` block (so a
whole function may be written on one line), and a comment elsewhere - the file
may still use `;` for header comments. The `//` form is also a comment. Width
markers work the same as in traditional macros.

A C-style function's return type is a width contract. A `return <expr>;` must
return a value of the function's declared width, and it emits a `RET` (where the
value lives is the primitive tier's concern, not zenas's):

- a `uint8_t` function must `return` an 8-bit value; a `uint16_t` function a
  16-bit value; a width mismatch is an error;
- a `void` function must not return a value (`return <expr>;` is an error), and
  may use a bare `return;`;
- a non-void function must return: a bare `return;`, or falling off the end with
  no `return` at all, is an error, because the declared width would go
  undelivered.

## Packages

Macros may be grouped into packages, so that two libraries can each define a
macro of the same name without clashing. A `.PACKAGE name` directive sets the
package for the macros defined after it, until the next `.PACKAGE` or the end of
the file:

```
.PACKAGE math
MACRO add(uint8_t aa, uint8_t bb)
    LD A, aa
    ADD A, bb
ENDMACRO

.PACKAGE counter
MACRO add(uint8_t nn)
    LD A, nn
    INC A
ENDMACRO

.PACKAGE main
        math.add(10, 5)     ; the math package's add
        counter.add(7)      ; the counter package's add
```

A call may be **qualified** with the package name (`math.add`) to select one
exactly. An **unqualified** call (`add`) resolves only when a single package
defines that name; if more than one does, it is an error and the call must be
qualified:

```
        add(5)              ; error: ambiguous, defined in math and counter
        math.add(5, 3)      ; fine
```

A real instruction is never shadowed by a macro of the same name: a bare `ADD`
is always the `ADD` instruction, and a macro named `add` is reachable only as
`package.add`. This lets a library use short, natural names without colliding
with the instruction set.

Macros defined with no `.PACKAGE` in effect belong to the default (unnamed)
package and are called unqualified, as before. Affiliation is currently per file;
the directive does not emit any code.

## Dialect modes

zenas can read a region of source in a different assembler's dialect, so legacy
code - typically a pasmo include - can be assembled without converting it. A
`.pasmo` directive switches into the pasmo dialect; `.zenas` switches back to
zenas's native syntax. The switch is scoped and may be used repeatedly; the
directives emit no code.

```
.pasmo
        INCLUDE "legacy.inc"     ; assembled under pasmo rules
.zenas
        ; native zenas syntax resumes here
```

Because an include is spliced in as text before lexing, wrapping the include in
`.pasmo` / `.zenas` is enough to assemble the included file under pasmo rules.

In the pasmo dialect:

- `$` is the location counter (the current address). A `$` followed by a hex
  digit, e.g. `$8000`, is still a hex literal, so `ORG $8000` works as in pasmo.
  In native zenas, `$` is always the hex prefix.
- `DEFM "..."` is accepted as a string/byte directive (also accepted in native
  mode as a convenience; it is equivalent to `DEFB` with a string).
- A label may be written without a colon, as in `loop    NOP`, at any
  indentation. An identifier that is an instruction mnemonic, a directive, or a
  known macro name is treated as such rather than as a label, matching pasmo.
- `#` is a hexadecimal prefix (`#80` is `0x80`), in addition to `&`, `0x`, and
  the `h` suffix which work in both dialects.
- `INCLUDE` accepts a bare unquoted filename (`INCLUDE if.asm`) as well as the
  quoted form.

Further pasmo conveniences (`name MACRO args` macro syntax, `PROC`/`LOCAL`
scoping) are planned.

For the full measured compatibility picture (which pasmo and sjasmplus
constructs assemble, which do not, and what each one does), see
`docs/DIALECT_COMPATIBILITY.md`. The instruction set, `ORG`, `EQU`,
`DEFB`/`DEFW`, and conditionals are identical in both dialects and need no mode.

## Z80N (ZX Spectrum Next)

The Z80N extended instructions are off by default. Enable them with `--next` (or
`--cpu=Z80N`):

```
zenas assemble next-game.asm out.bin --next
```

The full set is supported - `SWAPNIB`, `MIRROR`, `TEST n`, the barrel-shift
group (`BSLA`/`BSRA`/`BSRL`/`BSRF`/`BRLC DE,B`), `MUL`, `ADD rr,A`, `ADD rr,nn`,
`PUSH nn`, `OUTINB`, `NEXTREG`, `PIXELDN`/`PIXELAD`/`SETAE`, `JP (C)`, and the
DMA block-copy group (`LDIX`/`LDWS`/`LDDX`/`LDIRX`/`LDPIRX`/`LDDRX`). Output
matches sjasmplus byte-for-byte. See [Z80N_REFERENCE.md](Z80N_REFERENCE.md) for
the encodings.

Without `--next`, a Z80N mnemonic is an error, so a stray `MUL` in plain Z80
source is caught.

## Character sets

`--charset=NAME` chooses how the bytes of string literals (in `DB` and character
literals) are encoded. The default is `ascii`. The available sets are:

`ascii`, `spectrum-uk`, `spectrum-tk90x`, `spectrum-inves`, `spectrum-czech`,
`spectrum-polish`, `msx-jp`, `msx-eu`, `cpc-uk`, `cpc-fr`.

When a character is not present in the target set, zenas substitutes the closest
available one (for example an accented letter to its base letter) and issues a
warning. `--no-warnings` suppresses those warnings.

## Output and reports

By default `zenas assemble` writes a raw binary. A hex dump is shown for small
outputs; `--hex` forces it and `--no-hex` suppresses it.

`--sym[=path]` writes a pasmo-format symbol file (`NAME EQU 0XXXXH`, one per
line, sorted), suitable for memory-map tooling.

`--json=LEVEL` emits a JSON report instead of a binary:

| Level | Contents |
|-------|----------|
| `basic` | Machine code and symbols only |
| `standard` | Adds errors and assembly info |
| `detailed` | Adds an instruction breakdown |
| `full` | Complete metadata with timing |

## Running and testing assembled code

Unlike a plain cross-assembler, zenas can execute the code it just assembled,
in-process, using the bundled Z80/Z80N emulator core - no separate emulator and
no file round-trip. Two subcommands do this:

- `zenas run <file>` assembles and executes, then reports the final CPU and
  memory state.
- `zenas assert <file> --expect ...` does the same, then checks the final state
  against expectations and exits non-zero if any fail (suitable for CI).

The program is loaded at its first `ORG` and execution starts there (or at a
named routine with `--call`). Registers start zeroed for reproducibility.
Execution stops at `HALT`, at a returning `--call` routine, or when the
instruction cap (`--max-steps`, default 1,000,000) is reached - the cap is the
infinite-loop guard.

Common options (shared by both subcommands): `--max-steps=N`,
`--call=<label>` (run a subroutine and stop when it returns),
`--preload=ADDR,FILE` (load a binary into memory before running; repeatable),
`--trace` (print each instruction as it executes), `--hex` / `--dump=START:LEN`
(dump a memory region afterwards), `--json[=basic|standard|detailed|full]`, and
`--next` (Z80N).

`assert` adds `--expect="..."`, a comma-separated list of checks over registers
(`A`, `BC`, `HL`, `AF`, the shadow pairs `AF_`/`BC_`/`DE_`/`HL_`, `SP`, `PC`,
...), flags (`CF` `ZF` `SF` `HF` `PF` `NF`, value 0 or 1), and memory bytes
(`(0xC000)=0x42`). It prints `PASS`/`FAIL` per check.

```
zenas run game.asm --preload=0xC000,sprites.bin --trace
zenas assert math.asm --call=multiply --expect="A=0x0C,CF=0"
```

Test files named `*_test.asm` can include the program under test and define
`test_*` routines each followed by an `.EXPECT` directive; `zenas assert
file_test.asm` then discovers and runs them go-test style. `.EXPECT` is only
allowed in `*_test.asm` files (an assembly error elsewhere), so test metadata
cannot reach a production build.

For a fuller walkthrough see `docs/RUNTIME.md`.
