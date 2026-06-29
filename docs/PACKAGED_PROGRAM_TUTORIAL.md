# Building and testing a packaged program

This is a hands-on walkthrough. We build a small but real program — a sprite-row
compositor that writes graphics bytes into a buffer — using zenas **packages** to
organise reusable macros, then prove it works with the built-in test harness
using `.EXPECT` and `.MATCH`. Every listing here assembles and every test passes
as shown.

It assumes you have read the [programming guide](ZENAS_PROGRAMMING.md) for the
lay of the land. For the exhaustive reference, see the [manual](MANUAL.md).

## Contents

- [What we are building](#what-we-are-building)
- [Step 1: a packaged library of helpers](#step-1-a-packaged-library-of-helpers)
- [Step 2: the program](#step-2-the-program)
- [Step 3: run it](#step-3-run-it)
- [Step 4: test it with .EXPECT](#step-4-test-it-with-expect)
- [Step 5: test a whole span with .MATCH](#step-5-test-a-whole-span-with-match)
- [Parameter names and registers](#parameter-names-and-registers)
- [The C-style front-end](#the-c-style-front-end)

## What we are building

A routine, `compose`, that draws a five-byte sprite row into a buffer at `0xC000`
and then writes a colour-attribute byte after it. The sprite is a simple shape:

```
0x3C  0x42  0x00  0x42  0x3C        ; the five pixel bytes
0x47                                ; an attribute byte: ink 7, BRIGHT set
```

We will write the byte-poking helpers once, as a reusable package, and use them
from the program — then test the result without ever opening a separate emulator.

## Step 1: a packaged library of helpers

Put the reusable helpers in their own file, `sprite.asm`. A `.PACKAGE` directive
affiliates the macros that follow it, so a caller can refer to them as
`draw.put`, `util.set_bits`, and so on. This keeps short, natural names like
`put` from colliding with anything else.

```asm
; sprite.asm - packaged helpers for building a row of screen bytes

.PACKAGE draw
; write a byte at the cursor (HL) and advance.
MACRO put(uint8_t val)
    LD (HL), val
    INC HL
ENDMACRO

; write a blank (0x00) and advance
MACRO gap()
    LD (HL), 0
    INC HL
ENDMACRO

.PACKAGE util
; A = A OR mask  (set bits)
MACRO set_bits(uint8_t mask)
    OR mask
ENDMACRO
```

Two packages, `draw` and `util`. Each `MACRO` takes width-typed parameters
(`uint8_t val`); the type is the parameter's width contract. Nothing is emitted
yet — a package definition is just a set of macros waiting to be called.

## Step 2: the program

Now `compose.asm` includes the library and uses it. Calls are **qualified** with
the package name, so `draw.put` is unambiguous even though another package could
define its own `put`.

```asm
; compose.asm - draw a 5-byte sprite row into a buffer
    INCLUDE "sprite.asm"
    ORG 0x8000

ROW: EQU 0xC000

compose:
    LD HL, ROW
    draw.put(0x3C)      ; .XXXX..
    draw.put(0x42)      ; X....X.
    draw.gap()          ; blank middle
    draw.put(0x42)
    draw.put(0x3C)
    ; build an attribute byte in A: ink 7 with BRIGHT (bit 6) set
    LD A, 0x07
    util.set_bits(0x40) ; A = 0x47
    LD (ROW+5), A
    RET
```

Assemble it:

```
zenas assemble compose.asm compose.bin --hex
```

```
Assembled 26 bytes

Hex Dump:
0000: 21 00 C0 36 3C 23 36 42
0008: 23 36 00 23 36 42 23 36
0010: 3C 23 3E 07 F6 40 32 05
0018: C0 C9
```

Each `draw.put(n)` expanded to `LD (HL), n` / `INC HL`; `draw.gap()` to
`LD (HL), 0` / `INC HL`; `util.set_bits(0x40)` to `OR 0x40`. The package
qualification disappeared at assembly time — the output is the same bytes you
would get by writing the expansions out by hand.

## Step 3: run it

Before writing any tests, watch it execute. `zenas run` assembles and runs the
code in the built-in Z80 core; `--call` enters at a label and stops when that
routine returns; `--dump` shows a memory region afterwards.

```
zenas run compose.asm --call=compose --dump=0xC000:6
```

```
Returned after 15 instructions (115 cycles).
...
Memory @ C000:
  C000  3C 42 00 42 3C 47                                <B.B<G
```

The six bytes are exactly what we intended. `--trace` would show each instruction
as it runs, instruction by instruction.

## Step 4: test it with .EXPECT

To turn that into a repeatable check, write a test file. A file whose name ends
`_test.asm` may contain `test_*` routines, each followed by `.EXPECT` directives;
`zenas assert` discovers and runs them go-test style. `.EXPECT` checks one value
at a time — a register, a flag, or a single memory byte.

```asm
; compose_test.asm - tests for the sprite-row compositor
    INCLUDE "compose.asm"

test_attribute_bright:
    CALL compose
    RET
    .EXPECT (0xC005)=0x47
```

```
zenas assert compose_test.asm
```

```
PASS  test_attribute_bright

1 passed, 0 failed
```

The routine runs, and afterwards zenas checks that the byte at `0xC005` is
`0x47`. `.EXPECT` is legal **only** in a `*_test.asm` file, so this test metadata
can never reach a production build — assembling it anywhere else is an error.

## Step 5: test a whole span with .MATCH

Checking one byte at a time is tedious for a buffer. `.MATCH` asserts a whole
span at once. Its second argument is an ordinary data directive (`.db`/`.dw`),
assembled the normal way and compared against memory after the run:

```asm
; (add this routine to compose_test.asm)

test_compose_row:
    CALL compose
    RET
    .EXPECT (0xC000)=0x3C
    .EXPECT (0xC005)=0x47
    .MATCH 0xC000, .db 0x 3C 42 00 42 3C 47
```

```
zenas assert compose_test.asm
```

```
PASS  test_attribute_bright
PASS  test_compose_row

2 passed, 0 failed
```

The `.MATCH 0xC000, .db 0x 3C 42 00 42 3C 47` line asserts that the six bytes at
`0xC000` are exactly that sprite-plus-attribute pattern. The `0x ...` form is the
spaced-hex radix-input notation, which reads naturally for byte tables. A routine
may carry any number of `.EXPECT` and `.MATCH` directives; they are checked
together as one test, and on a mismatch `.MATCH` reports the first differing
offset (for example `.MATCH at 0xC000+2: expected 0xF0, got 0x00`).

This is the whole loop: write a routine, assert its memory effect, and have a
one-command, CI-friendly check that exits non-zero if anything regresses.

## Parameter names and registers

zenas will not let a macro parameter take a name that collides with a register or
condition code (`a`, `b`, `c`, `hl`, `sp`, `nz`, ...). Such a name is a trap: in a
body line like `LD (HL), b`, the `b` would assemble as the **register B**, not the
parameter — wrong code with no error. Rather than leave that to chance, zenas
rejects it at definition time:

```
macro 'put' parameter 'b' is a reserved register or condition name; a parameter
with this name would be silently assembled as the B register/condition inside the
macro body - rename it (for example to 'b_')
```

So name parameters for what they hold — `val`, `mask`, `addr`, `count` — and the
collision cannot happen. There is nothing to remember at the call site; the
assembler enforces it.

## The C-style front-end

zenas also accepts a C-like function syntax, selected with `.MACRO_STYLE C`.
Functions have width-typed signatures (`uint8_t`, `uint16_t`, `void`) and an
`asm { }` body holding ordinary Z80. They are a more structured way to write the
same machine code — useful when a typed, function-shaped surface reads better
than bare macros. The return width is a contract checked at assembly time: a
`uint8_t` function must return an 8-bit value, a `void` function must not return
one, and a non-void function that never returns is an error.

```asm
.MACRO_STYLE C

; add five to the accumulator
uint8_t add_five(uint8_t value) {
    asm {
        ADD A, 5;
    }
    return value;
}

; sum the two arguments
uint8_t sum(uint8_t first, uint8_t second) {
    asm {
        ADD A, B;
    }
    return first;
}

void main() {
    add_five(10);        ; A = 15
    sum(15, 25);         ; A = 40
    asm {
        LD (0xC000), A;
        HALT;
    }
}

.END
```

### How calls work, and why results live in registers

The calling convention is register-based: **the first argument arrives in `A`,
the second in `B`, and a function's return value comes back in `A`.** Functions
are expanded inline at the call site (there is no `CALL`/`RET` overhead), so a
call is just "load the argument registers, then run the body". That is why the
bodies above operate directly on `A` and `B`.

This is the one idea to hold onto: **a result lives in a register — usually `A` —
and the next call overwrites it.** In `main` above, `add_five(10)` leaves 15 in
`A`, but `sum(15, 25)` immediately reloads `A` and `B` and overwrites it with 40.
Registers are working space, not storage. To *keep* a value — across another
call, or past the end of the program so a test can inspect it — you have to move
it out of the register into memory. That is what `LD (0xC000), A` in `main` does:
it copies the final accumulator to address `0xC000` before halting, so the result
survives. Run it:

```
zenas run cstyle.asm --dump=0xC000:1
```

```
Halted after 7 instructions (49 cycles).
...
  C000  28                                               (
```

`0x28` is 40 — the value `sum` left in `A`, now safely in memory at `0xC000`.

### Variables: named memory

The store above used a hard-coded address, `LD (0xC000), A`. You do not have to
hard-code it: a label is a name for a memory location, so `LD (total), A` inside
an `asm { }` block works just as well, given a `total:` defined somewhere (the
[earlier `.DS` storage](#step-1-a-packaged-library-of-helpers) is exactly this).
Numeric address or named label, it is the same instruction either way.

A C-style variable is that pattern made convenient: declare one and assign to it,
and zenas allocates the labelled byte for you (two for a `uint16_t`), placed after
the code so it never sits in the execution path, and turns the assignment into the
store.

```asm
.MACRO_STYLE C
.PACKAGE math
uint8_t add(uint8_t first, uint8_t second) { asm { ADD A, B; } return first; }

void main() {
    uint8_t total;
    total = math.add(20, 22);   ; run the call, store its result (42) in total
    asm {
        LD A, (total);          ; read it back into A when you need it
        LD (0xC000), A;
        HALT;
    }
}
```

`total = math.add(20, 22)` does exactly what the register discussion predicts: it
loads the arguments, runs the function (whose result lands in `A`), and then
stores `A` into `total`'s memory — the move-to-memory step, done for you. `total = 42`
stores a literal the same way. Because `total` is an ordinary labelled byte, an
`asm { }` block reads or writes it by name (`LD A, (total)`, `LD (total), A`),
and a test can assert on it by name: `.EXPECT (total)=42` or `--expect="(total)=42"`
(memory targets accept a symbol or a numeric address).

### The rules, and where packages come in

zenas enforces a few naming rules so an identifier can never be silently misread,
and each error tells you the fix:

- A parameter, variable, or function may not be named after a register or
  condition (`a`, `b`, `hl`, `nz`, ...) — a bare `b` in a body would assemble as
  the register, so name things for what they hold (`value`, `first`, `total`).
- A function may not share a name with an instruction mnemonic (`add`, `ld`,
  `jp`, ...) *unless it is in a package*. A bare `add(...)` would assemble as the
  `ADD` instruction, so a mnemonic-named function with no package is rejected.
  Put it in a package and call it qualified — `math.add(...)` is unambiguous, and
  this is exactly the disambiguation packages exist for. (That is why the variable
  example above could safely call its function `add`: it lives in package `math`.)

One structural limit to know: a file is either C-style or traditional, chosen by
the single `.MACRO_STYLE` at the top, and a packaged *macro* is not called from
inside a C-style `asm { }` block. Reach for traditional macros and packages when
you want a reusable library (as earlier in this tutorial); reach for C-style when
a typed, function-shaped surface suits the program better.
