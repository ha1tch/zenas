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
- [Packaging: zenas build](#packaging-zenas-build)
- [Running and testing assembled code](#running-and-testing-assembled-code)

For an orientation aimed at users coming from another assembler — what is
familiar, what differs, and what zenas uniquely enables — see the
[Zenas programming guide](ZENAS_PROGRAMMING.md). For a worked, build-and-test
walkthrough using packages and the test harness, see the
[packaged-program tutorial](PACKAGED_PROGRAM_TUTORIAL.md).

## Running zenas

```
zenas assemble <input.asm> [output.bin] [options]   assemble to a raw binary
zenas build    <input.asm> [options]                assemble and package to tape/snapshot/NEX
zenas run      <input.asm> [options]                assemble and execute in the emulator
zenas assert   <input.asm> --expect ... [options]   execute and check final state
zenas test                                          run zenas's built-in self-tests
zenas version                                        print the version
zenas help [--all]                                   show help; --all for the full reference
```

If no output file is given, the input name is used with a `.bin` extension.
`zenas help` shows the common options; `zenas help --all` shows the full
reference. The `build` command is covered under
[Output and reports](#output-and-reports) (for the artifact formats) and the
`run`/`assert` execution layer under
[Running and testing assembled code](#running-and-testing-assembled-code).

Options:

| Option | Effect |
|--------|--------|
| `--next` | Enable the Z80N (ZX Spectrum Next) instruction set |
| `--cpu=Z80\|Z80N` | Select the CPU target (default `Z80`); `--cpu=Z80N` is the same as `--next` |
| `--define=NAME[=VAL]` | Pre-define a symbol (default value 1); drives `IF`/`IFDEF` |
| `--tag NAME` | Select a build tag (see [Build tags](#build-tags)) |
| `--sym[=path]` | Write a pasmo-format symbol file (default `<output>.sym`) |
| `--map[=path]` | Write a CSpect map file (default `<output>.map`); see [Symbol files](#symbol-files) |
| `--map-paging=P,P,P,P,P,P,P,P` | The 8K page in each slot, for the map's physical addresses |
| `--symbols=FILE` | Define the names in another tool's symbol file before assembling; may be repeated |
| `--symbols-format=NAME` | Format of the `--symbols` files: `auto` (default), `cspect`, `pasmo`, `zenas`, `sjasmplus`, `z88dk` |
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

**Parameter names may not be register or condition names.** Because arguments are
substituted textually, a parameter named `b` in a body line like `LD (HL), b`
would assemble as the register **B**, not the argument — wrong code with no error.
zenas therefore rejects, at definition time, any parameter whose name is a
register (`A`, `B`, ... `L`, `I`, `R`, `IXH`/`IXL`/`IYH`/`IYL`, the pairs `BC`,
`DE`, `HL`, `SP`, `AF`, `IX`, `IY`) or a condition code (`NZ`, `Z`, `NC`, `C`,
`PO`, `PE`, `P`, `M`). Name parameters for what they hold — `val`, `mask`,
`addr`, `count` — and the collision cannot arise.

**A macro may not share a name with an instruction mnemonic unless it is in a
package.** A bare `add` is always the `ADD` instruction, so a macro named `add`
with no package could never be reached. zenas rejects such a definition and
directs you to place it in a package, where it is called qualified (`math.add`)
and is unambiguous. (For C-style functions, the same applies, and a function,
variable, or parameter may not be named after a register or condition either.)

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

### Inline vs singleton expansion

By default every instantiation of a macro emits a full copy of its body (inline
expansion). For a body called many times this trades size for speed. The
`.MACRO_MODE` directive selects the strategy:

```
.MACRO_MODE INLINE      ; the default: every call emits the full body
.MACRO_MODE SINGLETON   ; emit the body once as a routine; every call is a CALL
```

In `SINGLETON` mode the body is emitted once, after the program code, as a
labelled routine ending in `RET`, and each instantiation becomes a `CALL` to it.
This is smaller whenever the body is larger than a `CALL` (3 bytes) and is called
more than once, at the cost of the call/return overhead at run time.

Parameters are passed through fixed memory slots, not registers or the stack.
Each parameter gets one labelled slot (emitted once, beside the routine); a call
writes its argument into the slot and then `CALL`s, and the body reads the
parameter from the slot. So a body that uses `value` reads it as `(value)` — the
parameter name denotes the slot's address:

```
.MACRO_MODE SINGLETON
MACRO emit(uint8_t val)
    LD A, (val)      ; read the argument from its slot
    LD (HL), A
    INC HL
ENDMACRO
```

Because the slots are fixed locations, a parameterised singleton is **not
re-entrant**: it must not call itself, directly or indirectly. zenas detects
recursion among singleton macros and rejects it, pointing you to `INLINE` (or to
an explicit subroutine that saves its context on the stack). Note too that a
singleton reached from an interrupt that could fire mid-call would corrupt the
slots; keep such routines `INLINE`.

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

### Symbol files

`--map[=path]` writes the map file that CSpect's debugger loads with `-map`,
in the form SNASM and sjasmplus (`CSPECTMAP`) write it. One symbol a line,
sorted by name:

```
0000229B 0000029B 01 BORDER
00008000 00008000 00 start
0000C000 00000000 00 upper
```

The columns are the 16-bit value, the physical address, the type (`00` a
label, `01` a constant: an `EQU`, a `--define` or a tag) and the name. A
label's physical address is the 8K page in its slot times 8192, plus its
offset in the page. The pages are the Next's as a program finds them, numbered
as sjasmplus numbers them for its `ZXSPECTRUMNEXT` device: 14 and 15 under the
ROM, then 10, 11 (bank 5), 4, 5 (bank 2) and 0, 1 (bank 0). For the same
source the file is the one sjasmplus writes, except that sjasmplus puts the
names in capitals and zenas leaves them as written.

zenas has no paging directives, so code meant for another page is assembled
at its address and the map is told where it will be:
`--map-paging=14,15,10,11,4,5,60,61` gives the labels from 0xC000 up the
physical addresses of pages 60 and 61.

```
zenas assemble game.asm game.bin --next --map
CSpect.exe -map=game.map game.nex
```

`--symbols=FILE` goes the other way: the names in a symbol file another tool
wrote are defined before the source is assembled, so it can call a ROM's
routines or another module's labels by name without an include file of
`EQU`s. Four formats are read, told apart by the file's first line
(`--symbols-format` names one when that fails):

| Format | Written by | A line |
|--------|-----------|--------|
| `cspect` | SNASM's `-map`, sjasmplus's `CSPECTMAP`, zenas's `--map`, zendis's `--map` | `00008000 00008000 00 START` |
| `pasmo`, `zenas` | pasmo, and zenas's `--sym` | `start		EQU 08000H` |
| `sjasmplus` | sjasmplus's `--sym` | `start: EQU 0x00008000` |
| `z88dk` | z88dk's `.map` | `_main = $8000 ; addr, public, ...` |

```
zenas assemble kernel.asm kernel.bin --sym
zenas assemble app.asm app.bin --symbols=kernel.sym --symbols=rom48.map
```

A character a zenas name cannot have becomes an underscore: CSpect's local
label `START@LOOP` and sjasmplus's `start.loop` are `START_LOOP` and
`start_loop` in the source. A value that does not fit in 16 bits is left out.
Two files that give one name two values are an error. What is imported is not
the program's own: it is left out of `--sym`, `--map` and the symbol table
printed, unless the source defines the name itself, which then replaces the
imported value. `zenas build` takes `--sym`, `--map`, `--map-paging` and
`--symbols` too (see [Packaging](#packaging-zenas-build)).

The formats were taken from files the tools wrote (the maps that come with
CSpect, sjasmplus's output) and CSpect's ReadMe; z88dk's is from its
documented shape. A map zenas wrote has not been loaded into CSpect.

**As a library.** A program that embeds the assembler has the same three
things without files, in the `assembler` package:

```go
asm := assembler.New()
asm.ImportSymbols(romMap, symfile.FormatAuto)        // --symbols: any io.Reader
result, _ := asm.AssembleString(source)

assembler.WriteCSpectMap(w, result, assembler.DefaultNextPaging)   // --map
assembler.WritePasmoSym(w, result)                                 // --sym
symbols := assembler.ExportSymbols(result, paging)                 // the same, as data
```

`result.SymbolKinds` says which of `result.Symbols` are labels and which
constants. `symfile` is zentools' `pkg/symfile`
(`github.com/ha1tch/zentools/pkg/symfile`), which reads and writes the formats
themselves and has `symfile.Table`, the names of several files by address.
zendis uses the same package, so what `ExportSymbols` returns goes straight
into the table a disassembly takes its names from; zendis's `docs/LIBRARY.md` has that
example.

`--json=LEVEL` emits a JSON report instead of a binary:

| Level | Contents |
|-------|----------|
| `basic` | Machine code and symbols only |
| `standard` | Adds errors and assembly info |
| `detailed` | Adds an instruction breakdown |
| `full` | Complete metadata with timing |

## Packaging: zenas build

`zenas build` assembles a source and writes it in the formats a Spectrum, a
Spectrum Next or an emulator loads. Any number of formats can be asked for
in one run; each is written beside the others as `<basename>.<ext>`.

```
zenas build <input.asm> [--tap] [--tzx] [--pzx] [--sna] [--z80] [--szx] [--nex]
            [--loader] [--start <addr|symbol>] [--sp <addr>]
            [--model 48k|128k|plus2|plus2a|plus3] [-o <basename>]
            [--border <0-7>] [--core <a.b.c>]
            [--next | --cpu=Z80|Z80N] [--define=NAME[=VAL]] [--tag NAME]
            [--sym[=path]] [--map[=path]] [--map-paging=...] [--symbols=FILE]
```

| Option | File | What it is | `--start` |
|--------|------|------------|-----------|
| `--tap` | `.tap` | Tape: the code as one CODE block | with `--loader` |
| `--tzx` | `.tzx` | The same tape as TZX | with `--loader` |
| `--pzx` | `.pzx` | The same tape as PZX, with the ROM's standard timings | with `--loader` |
| `--sna` | `.sna` | Snapshot, 48K or 128K by `--model` | required |
| `--z80` | `.z80` | Snapshot, version 3 | required |
| `--szx` | `.szx` | Snapshot in the ZX-State format | required |
| `--nex` | `.nex` | Spectrum Next executable, for `.nexload` and the Browser | required |

`--loader` puts a BASIC program ahead of the code on a tape, so that `LOAD ""`
loads and runs it. `--start` is an address or a label of the source. `--sp`
sets the stack pointer (0xFF00 if it is not given). `--model` picks the machine
a snapshot is made for; it has no meaning for a tape or a `.nex`.

**NEX.** The code goes into the banks that are paged in when a NEX program
starts: bank 5 at 0x4000, bank 2 at 0x8000 and bank 0 at 0xC000. It must
therefore begin at 0x4000 or above. `--border` is the border colour the
loader sets (0 if not given) and `--core` the lowest core version the file
asks for (2.0.28 if not given). The file has no loading screen and uses no
bank beyond those three; `zx next nex` of zentools builds files that do.

```
zenas build game.asm --nex --start main --border 1
```

`--next` (or `--cpu=Z80N`), `--define` and `--tag` are those of `zenas
assemble`: the Z80N instructions, symbols defined on the command line, and
build tags, with `ZENAS_TAGS` defined as there. `--nex` turns the Z80N
instructions on by itself, since a `.nex` runs on a Next only; when other
formats are asked for in the same run they get the same code. `--cpu=Z80`
keeps them off, for a source that uses a Z80N mnemonic as a name of its own.

The formats are written by [zentools](https://github.com/ha1tch/zentools)'
`pkg/build`, which zenas uses as a library; `zx` of zentools reads and
converts every one of them.

**As a library.** `pkg/artifact` is `zenas build` without files: it takes
an assembly's result and returns a format's bytes.

```go
asm := assembler.New()
asm.EnableZ80N()                                   // --next (--nex does it in the command)
asm.DefineTags(tags)                               // --tag, any number; defines ZENAS_TAGS too
name, value, _ := assembler.ParseDefine("LIVES=5") // --define
asm.Define(name, value)
result, _ := asm.AssembleString(source)

nexOpts, _ := artifact.ParseNEXOptions("1", "3.1.10")   // --border, --core
opts := artifact.Options{
    Name:   "game",       // the name on a tape
    Start:  "main",       // --start: an address or a label
    Loader: true,         // --loader, for the tape formats
    NEX:    nexOpts,
}                         // SP and Model as --sp and --model; 0xFF00 and the 48K if not set
for _, format := range artifact.Formats() {
    img, err := artifact.Encode(result, format, opts)
    // write img to "game" + format.Ext()
}
```

`Format.NeedsStart(loader)` says whether a format needs `Start`, and
`Format.Next()` which one is the Next's. `artifact.Request` returns
zentools' `build.Request`, for a caller that goes on with `pkg/build`
itself, and `artifact.ResolveStart` an entry point's address.

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
(`(0xC000)=0x42`, or by symbol, `(total)=0x42`). It prints `PASS`/`FAIL` per check.

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
