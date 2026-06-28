# Running and testing code with zenas

zenas can execute the code it assembles, in-process, through a bundled Z80/Z80N
emulator core. This is a capability most assemblers do not have: there is no
separate emulator to launch and no binary to write out and load elsewhere. You
assemble, run, and check results in a single command.

Two subcommands provide this:

- `zenas run` - assemble and execute, then report final state.
- `zenas assert` - assemble, execute, and check the result against expectations,
  exiting non-zero on failure (so it fits in a CI pipeline or a Makefile).

This guide covers both, the execution model they share, and the assertion
vocabulary.

## The execution model

Both subcommands work the same way up to the point of reporting:

1. The source is assembled in-process.
2. A fresh 64 KB RAM is created. Any `--preload` files are loaded first, then the
   assembled bytes are loaded at the program's first `ORG`.
3. The CPU registers are set to a known, zeroed state, so a given program always
   produces the same result. (This matters: real Z80 power-on register contents
   are undefined, which would make assertions non-reproducible.)
4. Execution begins at the first `ORG`, or at a named routine if `--call` is
   used.
5. Execution stops at the first of: a `HALT` instruction, a `--call` routine
   returning, or the instruction cap (`--max-steps`).

### Stopping, and the infinite-loop guard

A Z80 program has no inherent end, so the stop condition is explicit:

- **`HALT`** ends a whole-program run. Most test programs finish with `HALT`.
- **A returning `--call` routine** ends a subroutine run (see below).
- **`--max-steps=N`** caps the number of instructions (default 1,000,000). This
  is the infinite-loop guard. If it trips, `run` reports it and exits with code
  2; `assert` treats it as a test failure. Without this, a buggy program would
  hang the command instead of reporting.

## `zenas run`

```
zenas run <file> [options]
```

Assembles and runs the program, then prints the final register and flag state
(and optionally a memory dump). Example:

```
zenas run game.asm
```

```
Halted after 5 instructions (29 cycles).
  A=00  F=00 [--------]   AF=0000
  BC=0000  DE=0000  HL=0000
  IX=0000  IY=0000  SP=FFFF  PC=8008
  AF_=1100  BC_=2233  DE_=0000  HL_=0000
```

The flag field shows the F register as `SZ5H3PNC`, with a dash where a flag is
clear. `AF_` `BC_` `DE_` `HL_` are the alternate (shadow) register pairs.

### Watching execution: `--trace`

`--trace` prints each instruction as it runs - program counter, opcode, and the
accumulator and HL after the step. It is the quickest way to see where a program
spins if it never reaches `HALT`:

```
zenas run loop.asm --trace --max-steps=20
```

```
Trace:
      0  PC=8000  op=3E  A=00  HL=0000
      1  PC=8002  op=06  A=00  HL=0000
      2  PC=8004  op=80  A=05  HL=0000
      ...
```

### Loading data: `--preload`

`--preload=ADDR,FILE` loads a binary file into memory before execution. It is
repeatable, so a program can be run with the screen, sprite data, or other
assets it expects already in place:

```
zenas run game.asm --preload=0xC000,sprites.bin --preload=16384,title.scr
```

Addresses may be decimal or hexadecimal (`0x...`, `$...`, or a trailing `h`).
Preloads are loaded before the assembled program, so if regions overlap the
program wins.

### Dumping memory: `--hex` and `--dump`

`--hex` dumps the assembled region after the run. `--dump=START:LEN` dumps an
arbitrary region (and implies `--hex`):

```
zenas run sort.asm --dump=0xC000:16
```

### Structured output: `--json`

`--json[=level]` emits the result as JSON instead of the human-readable block,
with the same level names used elsewhere in zenas:

| Level | Contents |
|-------|----------|
| `basic` | Outcome, step count, core registers |
| `standard` | Adds flags and cycle count (the default for `--json`) |
| `detailed` | Adds the memory window (if `--hex`/`--dump` used) |
| `full` | Adds the full per-instruction trace (if `--trace` used) |

## `zenas assert`

```
zenas assert <file> --expect "<checks>" [options]
```

Runs the program exactly as `zenas run` does, then checks the final state against
a comma-separated list of expectations. Each check prints `PASS` or `FAIL`; the
command exits 0 only if every check passes, and 1 otherwise. A run that hits the
instruction cap fails as well, with a clear message - an infinite loop is a
failed test, not a hang.

```
zenas assert math.asm --call=multiply --expect="A=0x0C,CF=0"
```

```
PASS  A=0x0C
PASS  CF=0
```

### Calling a routine: `--call`

`--call=<label>` runs a single subroutine rather than the whole program. zenas
pushes a return address, sets the program counter to the label, and stops when
the routine returns. This is the natural unit to test: set up inputs, call the
routine, assert its outputs.

```
zenas assert routines.asm --call=to_uppercase --expect="A=0x41"
```

### The expectation vocabulary

`--expect` takes a comma-separated list. Three kinds of check are available.

**Registers.** 8-bit: `A` `F` `B` `C` `D` `E` `H` `L`. 16-bit pairs: `AF` `BC`
`DE` `HL` `IX` `IY` `SP` `PC`. Shadow pairs: `AF_` `BC_` `DE_` `HL_`. Values may
be decimal or hexadecimal.

```
--expect="A=0x42,HL=0x8000,SP=65000"
```

**Flags.** Written with an `F` suffix to avoid clashing with the register names
`C` and `H`: `CF` (carry), `ZF` (zero), `SF` (sign), `HF` (half-carry), `PF`
(parity/overflow), `NF` (add/subtract). The value is `0` or `1`.

```
--expect="ZF=1,CF=0"
```

**Memory.** `(ADDR)=VALUE` checks a single byte at an address.

```
--expect="(0xC000)=0xAB"
```

A note on `AF`: the low byte of `AF` is the flags register, which includes the
undocumented bits 3 and 5. Asserting an exact `AF` value is therefore more
brittle than asserting `A` together with the specific flags you care about
(`CF`, `ZF`, ...). Prefer the latter where you can.

### Example: testing a routine

A routine that doubles the accumulator, and a check that it sets the carry flag
on overflow:

```
        ORG $8000
        HALT                ; whole-program entry (unused here)

double: ADD A, A
        RET
```

```
zenas assert double.asm --call=double --expect="A=0x00,CF=1"
```

(With the accumulator starting at zero this particular check would not exercise
the overflow; in practice a test harness sets inputs first - see below.)

### Setting inputs

Because registers start zeroed, a routine that operates on inputs needs those
inputs established first. The straightforward pattern is a small wrapper routine
in the source that loads the inputs and then falls through or calls the routine
under test, and to `--call` the wrapper:

```
        ORG $8000
        HALT

test_double_7:
        LD A, 7
        CALL double
        RET

double: ADD A, A
        RET
```

```
zenas assert double.asm --call=test_double_7 --expect="A=0x0E"
```

## Test files (the Go-style layer)

The wrapper-routine convention above is the basis for file-level testing,
analogous to Go's `_test.go` files. A test file is named `*_test.asm`, includes
the program under test, and defines test routines each annotated with an
`.EXPECT` directive stating the expected post-conditions:

```
        ORG $8000
        INCLUDE "game.asm"

test_double_7:
        LD A, 7
        CALL double
        RET
.EXPECT A=0x0E

test_add10:
        LD A, 5
        CALL add10
        RET
.EXPECT A=15
```

Running `zenas assert game_test.asm` (with no `--expect`) discovers every
`.EXPECT`-annotated routine, runs each via its label, checks its expectations,
and reports go-test-style results:

```
PASS  test_double_7
PASS  test_add10

2 passed, 0 failed
```

The command exits non-zero if any test fails, so it drops into a Makefile or CI
step directly.

`.EXPECT` is only permitted in a file whose name ends in `_test.asm`; using it
anywhere else is an assembly error. This guarantees that test expectations can
never appear in a production build - the rule is enforced by the assembler, not
by convention. `.EXPECT` binds to the nearest preceding label and emits no
bytes. The expectation syntax is exactly the `--expect` vocabulary above
(registers, flags, and memory).

### Matching memory spans with `.MATCH`

`.EXPECT` checks one byte at a time. To assert that a whole span of memory holds
expected bytes - the natural test for a routine that writes a buffer, a sprite,
or a table - use `.MATCH`:

```
.MATCH <location>, <data>
```

The location is an address or a symbol; the data is an ordinary `.DB`/`.DW`/`.DM`
directive, assembled through the normal data path and compared against memory
after the test runs:

```
test_fill_marker:
        CALL fill_marker
        RET
.MATCH marker_buffer, .db 0x600DF00D
```

This asserts that the four bytes at `marker_buffer` are `60 0D F0 0D`. Because
the data part is a real data directive, every data form works, including the
radix-input forms:

```
.MATCH 0xC000, .db 0x 60 0D F0 0D      ; spaced hex
.MATCH 0xC000, .dw 0x 600D F00D        ; words, little-endian
.MATCH 0xC000, .db 0d 96 13 240 13     ; decimal
```

On failure, `.MATCH` reports the first differing offset, for example
`.MATCH at marker_buffer+2: expected 0xF0, got 0x00`. Like `.EXPECT`, `.MATCH` is
only allowed in a `*_test.asm` file. A test routine may carry any number of
`.EXPECT` and `.MATCH` directives; they are all checked together as one test.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | `run` completed; or `assert` and all checks passed |
| 1 | Assembly error, bad arguments, or an `assert` check failed |
| 2 | `run` reached the instruction cap (possible infinite loop) |
