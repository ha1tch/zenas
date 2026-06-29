# Dialect compatibility reference

This document records the measured state of zenas's compatibility with pasmo and
sjasmplus source, and explains what each non-zenas construct does. It exists so
the corpus diagnostic does not have to be repeated: the findings below were
produced by assembling pasmo's own sample suite (34 files) under `.pasmo` mode
and a sjasmplus/SpecNext corpus (70 files) plain, then tracing every failure to
its cause and verifying each construct's behaviour against the real pasmo and
sjasmplus binaries.

The corpora themselves are third-party (GPL/BSD) and are not vendored; they were
used only as a diagnostic oracle. This document is the durable artefact.

## Headline results (as measured)

| Corpus | Mode | Pass | Fail | Of |
|--------|------|------|------|-----|
| pasmo own sample suite | `.pasmo` | 7 | 27 | 34 |
| sjasmplus / SpecNext | plain + `--next` | 7 | 63 | 70 |

The low pasmo number is the important correction. The reference kernel (real
pasmo source that assembles byte-identical) passes because it uses only the
common core. The
pasmo sample suite deliberately exercises every pasmo feature, so it exposes the
true gap. The 7 passing sjasmplus files are the trivial tutorial files with no
SpecNext-specific constructs; every realistic SpecNext program fails immediately
on its meta-language (`DEVICE`, `{ }`, `DISP`), not on its instructions.

Two layers, opposite verdicts: the **instruction body** of any pasmo or sjasmplus
program already assembles (shared mnemonics, verified on 516 base + 31 Z80N
instructions). The **directive / meta-syntax layer** is where the gaps are - a
small, closeable gap for pasmo and a large, semantic-subsystem gap for sjasmplus.

## What already works (do not re-investigate)

Verified present in zenas; these are NOT gaps despite first impressions:

| Construct | Note |
|-----------|------|
| `$` location counter | pasmo mode; `$8000` still hex, bare `$` is current address |
| `$` arithmetic (`$+4`, `DJNZ $`) | works |
| `DEFB`/`DB`, `DEFW`/`DW`, `DEFM`, `DEFS`/`DS` | full pasmo data-directive set; `DM`/`DC`/`DZ` are NOT pasmo |
| `&FF` hex | works in both dialects |
| `1010b` binary suffix | works |
| `%1010` binary prefix | works |
| `0FFh` hex suffix | works |
| `0x40` hex prefix | works |
| character literals `'A'` | works as byte and (since 0.5.1) as expression term `'A'+1` |
| no-colon labels at any indentation | pasmo mode; matches pasmo (first non-mnemonic identifier is a label) |
| `#80` hex prefix | pasmo mode; `#80` == `0x80` |
| bare unquoted `INCLUDE if.asm` | works in any mode; native source uses quoted form |
| `IF`/`ELSE`/`ENDIF`, nested | works |
| `EQU`, forward references | works |

## pasmo gaps (measured, with cause and cost)

Ordered by value-per-effort. "Cost" is implementation effort; "files" is how many
of the 34 sample files are blocked by it.

### Closed (as of 0.5.2)

`#` hex prefix, bare unquoted `INCLUDE`, and no-colon labels at any indentation
(not just column 1) are now supported - see the "what already works" table above.
The remaining gaps below are the ones not yet closed.

### Cheap, isolated lexer additions (remaining)

**`?:` ternary operator** - pasmo supports a C-style conditional expression,
`condition ? a : b`, evaluated with short-circuit semantics (the untaken branch
may contain undefined symbols). This is the real meaning of `?` in pasmo source
(it is NOT an identifier character, contrary to a first reading of the corpus).
It appears in `black.asm`, `macro.asm`, `rept.asm`. This is an expression-grammar
feature (it also pulls in `:` as an operator), not a cheap lexer alias, so it is
grouped with the larger gaps rather than the cheap ones. Cost: moderate
(expression parser). Files: ~3.

### Expensive scoping subsystem (the largest blocker by file count)

**`PROC` / `ENDP`** - opens and closes a local scope. Labels defined inside a
`PROC ... ENDP` block are visible only within it, so the same label name can be
reused in different procedures without collision. This is pasmo's structured-label
mechanism.

**`LOCAL name[, name...]`** - declares the named labels as local to the current
`PROC` (or to the autolocal scope). A `LOCAL n` line means `n` is private to this
block; an outer `n` is a different symbol.

**`PUBLIC name`** - the inverse: marks a label as exported from its scope, visible
to the symbol table / other modules.

These form one feature (scoped labels) and appear in 11 of the 34 files. They are
the single largest pasmo blocker. Implementing them is a real scoping subsystem
(a symbol-table scope stack), not a syntactic alias - the expensive, deliberate
piece of pasmo compatibility. Note: this corrects an earlier impression (based on
the reference kernel alone, which uses one bare `LOCAL`) that `PROC`/`LOCAL` were
rare; pasmo's own suite uses them heavily.

### Out of scope by nature

**`8086` target files** (`t86`, parts of `hellocpm`, `callvers`, `echovers`) -
pasmo can emit 8086 code as a Z80 translation. These files are not Z80 programs;
zenas is a Z80/Z80N assembler and should not assemble them. Counting them as
"failures" is misleading - they are correctly out of scope.

**`name MACRO args` macro syntax** - pasmo defines a macro as `name MACRO params`
... `ENDM` and calls it bare (`delay 5`). zenas uses `MACRO name(params)` ...
`ENDMACRO` and calls `delay(5)`. The pasmo spelling is a deferred, low-frequency
item (a parser change in the macro subsystem). The no-colon-label rule already
excludes known macro names so they are not mis-parsed as labels once this lands.

## sjasmplus gaps (measured, with cause)

These are why 63 of 70 files fail. Unlike the pasmo gaps, most are semantic
subsystems, not syntax. They are documented for the `.sjasm` design discussion,
NOT as a to-do list - closing them as compatibility would mean reimplementing
sjasmplus.

**`DEVICE <name>`** (33 files) - selects a target machine model (e.g.
`DEVICE ZXSPECTRUMNEXT`, `DEVICE ZXSPECTRUM128`). It establishes a memory model
with banked pages, an output size, and which save directives are valid. It is the
foundation most SpecNext programs are built on, and it is a whole memory-model
subsystem, not a directive.

**`{ ... }` blocks** (103 files across corpus) - sjasmplus uses braces for
structured grouping (conditional bodies, struct bodies, scoped regions). zenas
has no brace-block grammar outside C-style macros.

**`DISP` / `ENT`** (87 files) - `DISP <addr>` / `ENT` assemble code as though it
were located at a different address than where it is stored (displaced assembly),
used for code that is built at one address but copied to and run at another. A
distinct addressing subsystem.

**`=` assignment** - `startline = 0` is a redefinable variable assignment
(unlike `EQU`, which is single-assignment). sjasmplus distinguishes constants
(`EQU`) from reassignable variables (`=`). zenas treats `=` as an unexpected
token in this position.

**`:` code inlining** - sjasmplus allows multiple instructions on one line
separated by colons (`LD A,C : INC A : PUSH AF`). zenas treats `:` only as a
label terminator, so it sees the colon mid-line as unexpected.

**`STRUCT name ... ENDS`** (7 files) - defines a named record layout; field names
become byte offsets (`STRUCT point / x BYTE / y BYTE / ENDS` gives `point.x = 0`,
`point.y = 1`, `point = 2` as the size). A layout/offset system.

**`DUP n ... EDUP`** (7 files) - repeats the enclosed lines `n` times at assembly
time (a repetition engine; pasmo's equivalent is `REPT`). Useful enough that a
native zenas equivalent could be considered on its own merits, independent of
sjasmplus compatibility.

**`SAVENEX` / `SAVEBIN` / `SAVESNA`** (15 files) - write the assembled output to a
file in a specific container format (`.nex` for SpecNext, raw binary, snapshot)
from within the source, often with page/bank arguments. Tied to the `DEVICE`
memory model.

**`MODULE name ... ENDMODULE`** (2 files) - a namespace for labels; names inside
are prefixed by the module name. zenas's **packages** are the equivalent and
better-shaped for the portability goal, so this overlaps existing zenas
functionality rather than being a missing feature - a candidate for
spelling-translation (`MODULE` -> package) rather than reimplementation, should a
`.sjasm` mode ever pursue it.

## Reading of the two gaps

pasmo compatibility is **bounded and mostly cheap**: the remaining gap is `#` hex,
`?` identifiers, bare include, and the one expensive-but-finite scoping subsystem
(`PROC`/`LOCAL`/`PUBLIC`). Reaching high pasmo-corpus coverage is a finishable
task.

sjasmplus compatibility is **open-ended**: the gap is dominated by `DEVICE`,
`{ }`, `DISP`, `STRUCT`, and the save-format subsystems - years of semantic
machinery. A SpecNext program is mostly sjasmplus-specific meta-language wrapping
a shared instruction body. This is the measured evidence that `.sjasm`-as-a-
compatibility-promise would be a large build, and that the productive forms of a
future `.sjasm` are spelling-translation (e.g. `MODULE` -> packages) or
cherry-picking genuinely useful constructs (`DUP`, `ALIGN`) into the native
dialect on their own merits - not wholesale source compatibility.
