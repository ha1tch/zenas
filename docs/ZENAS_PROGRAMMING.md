# Zenas programming

This guide is for someone who already knows Z80 assembly and another assembler —
pasmo, sjasmplus, z80asm — and wants to understand how zenas fits their hands.
It is in three parts: where zenas is just like home, where it differs, and what
it lets you do that other assemblers cannot.

For the exhaustive command and directive reference, see the
[manual](MANUAL.md). This guide is the orientation; the manual is the dictionary.
For a hands-on, build-and-test walkthrough, see the
[packaged-program tutorial](PACKAGED_PROGRAM_TUTORIAL.md).

## Contents

- [Where zenas is just like home](#where-zenas-is-just-like-home)
- [Where zenas differs](#where-zenas-differs)
- [What zenas lets you do that others cannot](#what-zenas-lets-you-do-that-others-cannot)
  - [Run the code you just assembled](#run-the-code-you-just-assembled)
  - [Assert on machine state](#assert-on-machine-state)
  - [Go-style test files](#go-style-test-files)
  - [C-style macros](#c-style-macros)
  - [Packages and disambiguation](#packages-and-disambiguation)
  - [Build tags as a bitmask](#build-tags-as-a-bitmask)
  - [Scoped dialect modes](#scoped-dialect-modes)
- [A worked example](#a-worked-example)

## Where zenas is just like home

If you have written Z80 in any mainstream assembler, the core of zenas needs no
relearning. It is a two-pass assembler with forward-reference resolution, and the
following all behave the way you expect:

- **Mnemonics and operands.** The full documented Z80 instruction set, written
  the usual way: `LD A, (HL)`, `JP NZ, loop`, `BIT 7, (IX+4)`.
- **Labels.** `name:` defines a label; a label on its own line attaches to the
  following instruction. Symbols are case-sensitive.
- **Directives.** `ORG`, `EQU`, `DB`/`DW` (with strings and multiple values),
  `DS`/`DEFS`, `INCLUDE`, `INCBIN` (with optional skip and length).
- **Numbers.** `0x1F`, `$1F`, `%0001`, `31`, and character literals `'A'`.
- **Expressions.** Symbol arithmetic in operands and displacements:
  `LD HL, table + 2*index`.
- **Conditional assembly.** `IF`/`IFDEF`/`IFNDEF`/`ELSE`/`ENDIF`, with symbols
  pre-defined from the command line via `--define`.
- **A pasmo-format symbol file.** `--sym` writes the same `NAME EQU 0XXXXH` file
  pasmo does, so existing tooling that parses it keeps working.

zenas assembles real-world operating-system source byte-for-byte identically to
pasmo, so for ordinary code the output is not merely equivalent — it is the same
bytes, with the same symbol addresses.

## Where zenas differs

The differences are deliberate and small in number. None of them change how a
plain instruction assembles; they are about the layers around it.

- **Two output modes, not one.** `zenas assemble` emits a raw binary, the way any
  assembler does. `zenas build` goes a step further and packages the result into
  a loadable artifact — a tape (`.tap`/`.tzx`) or a runnable snapshot
  (`.sna`/`.z80`) — without a separate tool. See [the manual](MANUAL.md) for the
  full `build` flag set.
- **Macros come in two flavours.** Alongside a traditional
  `MACRO NAME(params) ... ENDMACRO` system, zenas has a C-style function syntax
  that transpiles to macros. You choose per file with `.MACRO_STYLE`. Most
  assemblers offer only the traditional kind.
- **Names can be packaged.** A `.PACKAGE` affiliation lets two libraries each
  define a `rotate` or an `add` without colliding, with qualified calls
  (`math.add`) to disambiguate. This is closer to a module system than the flat
  global namespace most assemblers give you.
- **Build tags, not just defines.** Beyond `--define`, zenas has `--tag`, which
  defines a presence flag, a numbered bit, and contributes to a composite
  bitmask — a structured variant-selection mechanism rather than a bag of loose
  symbols.
- **Dialect modes are scoped.** Rather than a global "pasmo-compatible" switch,
  `.pasmo`/`.zenas` flip the lexer mid-stream, so you can ingest pasmo source in
  one included file without changing how the rest of your program is read.
- **A JSON report.** `--json=LEVEL` emits machine code, symbols, and (at higher
  levels) an instruction breakdown and timing, instead of a binary — useful when
  another program is consuming the assembler's output.

If you do none of these things, zenas is a drop-in for your existing flow.

## What zenas lets you do that others cannot

The headline capability is that **zenas can execute the code it just assembled,
and test it, without leaving the assembler.** A conventional assembler's job ends
when it has written bytes; verifying those bytes means loading them into a
separate emulator, by hand or through a script. zenas folds the emulator in.

### Run the code you just assembled

`zenas run <file>` assembles the source and executes it in-process on the bundled
zen80 Z80/Z80N core, then reports the final CPU and memory state. There is no
file round-trip and no second program.

```
zenas run game.asm
```

Execution starts at the first `ORG` (or at a named routine with `--call`),
registers start zeroed for reproducibility, and it stops at `HALT`, at a
returning `--call` routine, or when the instruction cap is hit — the cap
(`--max-steps`, default one million) is the infinite-loop guard.

`--trace` prints each instruction as it runs; `--dump=START:LEN` (or `--hex`)
dumps a memory region afterwards; `--preload=ADDR,FILE` loads a binary into
memory before the run, so a routine can operate on real input data.

```
zenas run sort.asm --call=sort --preload=0xC000,unsorted.bin --dump=0xC000:16 --trace
```

### Assert on machine state

`zenas assert` runs the code and then checks the final state against
expectations, printing `PASS`/`FAIL` per check and exiting non-zero if any fail —
so it drops straight into CI. The expectation vocabulary covers:

- **registers** — `A`, `B`...`L`, the pairs `BC`/`DE`/`HL`/`AF`, `IX`/`IY`,
  `SP`, `PC`, and the shadow pairs `AF_`/`BC_`/`DE_`/`HL_`;
- **flags** — `CF`, `ZF`, `SF`, `HF`, `PF`, `NF`, each `0` or `1`;
- **memory bytes** — `(0xC000)=0x42`.

Checks are comma-separated, or given as repeated `--expect` flags:

```
zenas assert math.asm --call=multiply --expect="A=0x0C,CF=0"
zenas assert math.asm --call=multiply --expect="A=12" --expect="(0x9000)=12"
```

This turns "did my routine compute the right answer" into a one-line, scriptable
check, with no emulator wiring of your own.

### Go-style test files

For a suite rather than a single check, a file named `*_test.asm` can hold the
program under test plus a set of `test_*` routines, each followed by an `.EXPECT`
directive (and optionally `.MATCH` for memory spans). `zenas assert file_test.asm`
then discovers and runs every `test_*` routine, go-test style.

```asm
; math_test.asm
    INCLUDE "math.asm"

test_add_basic:
    LD A, 5
    LD B, 10
    CALL add
    RET
    .EXPECT A=15, CF=0

test_add_carry:
    LD A, 0xFF
    LD B, 1
    CALL add
    RET
    .EXPECT A=0, CF=1
```

```
zenas assert math_test.asm
```

`.EXPECT` and `.MATCH` are legal **only** in `*_test.asm` files — using one
elsewhere is an assembly error. Test metadata therefore cannot leak into a
production build: the same source that defines your tests cannot accidentally
carry them into the shipped binary.

### C-style macros

zenas accepts a C-like function syntax that transpiles to traditional macros,
selected with `.MACRO_STYLE C`. Instead of `MACRO`/`ENDMACRO`, you write
function-shaped blocks with typed signatures and an `asm { ... }` body:

```asm
.MACRO_STYLE C
void main() {
    asm {
        HALT;
    }
}
```

Functions can be typed — `uint8_t`, `uint16_t`, or `void` — and zenas checks the
return *width*: a `uint8_t` function must return an 8-bit value, a `void` function
must not return one, and a non-void function that never returns is an error. This
is a portable half of a calling convention (the width is checked; where the value
lives is the primitive library's concern), and it is checking no plain macro
assembler does. It is still sugar over the traditional macro system — the
transpiler lowers it before assembly — so the two coexist. See the C-style
examples bundled in [`examples/`](../examples/) (the `example1*-cstyle-*.asm`
files) and the Macros section of the [manual](MANUAL.md).

By default a macro's body is expanded at every call site. `.MACRO_MODE SINGLETON`
instead emits the body once as a routine and turns each call into a `CALL` (with
arguments passed through fixed memory slots) — smaller when a sizeable body is
reused. See `examples/example15-macro-mode.asm` and the manual for the details
and its constraints.

### Packages and disambiguation

A `.PACKAGE` directive affiliates the macros that follow with a named package.
The point is collision-free libraries: two packages can each define `add`, and a
qualified call `math.add(5, 2)` selects one unambiguously. A qualified name is
also never confused with the `ADD` mnemonic, so a primitive library can use short
names like `rotate` or `add` without shadowing instructions for its callers.
Unqualified calls still work and resolve bare-unless-ambiguous. This is closer to
a namespacing system than the single flat symbol table most assemblers expose.

### Build tags as a bitmask

`--tag NAME` is more structured than a plain define. Each tag:

- defines `ZENAS_TAG_NAME` — a presence flag you can test in `IF`;
- defines `ZENAS_TAGBIT_NAME` — its bit, assigned in command-line order;
- contributes to `ZENAS_TAGS` — a composite bitmask, always defined (0 when no
  tags are set).

Because the tags compose, `IF` conditions can use boolean logic over them:

```asm
    IF ZENAS_TAG_debug AND ZENAS_TAG_plus3
        ; debug build for the +3
    ENDIF
```

```
zenas assemble game.asm --tag debug --tag plus3
```

This gives you a single, ordered, testable notion of "which variant is this"
rather than a handful of unrelated `--define`d symbols.

### Scoped dialect modes

`.pasmo` switches the lexer into a pasmo-compatible dialect mid-stream, and
`.zenas` switches back. The switch is scoped, not global: you can `.pasmo` then
`INCLUDE` a file of pasmo source and have it read correctly, while the rest of
your program continues to be read as native zenas. The mechanism is the same one
`.MACRO_STYLE C` uses — a directive flips the lexer until it is turned off again.
It is intentionally not an attempt to make the native dialect a superset of
pasmo, which would create ambiguities (notably around `$`).

## A worked example

Write a routine, then prove it works without ever opening an emulator:

```asm
; add.asm
    ORG 0x8000
add:                 ; A = A + B, sets CF on overflow
    ADD A, B
    RET
```

```
# run it once, watching every instruction and the result
zenas run add.asm --call=add --trace

# assert the contract in CI
zenas assert add.asm --call=add --expect="A=0"      # with A=0,B=0 to start

# build a runnable snapshot to try the larger program by hand
zenas build add.asm --z80 --start add --model 48k
```

The same source assembles to the same bytes pasmo would produce; what zenas adds
is everything *after* the bytes — running them, asserting on them, testing them,
and packaging them — in one tool.
