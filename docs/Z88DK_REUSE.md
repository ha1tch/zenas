# Reusing the z88dk library by assembling its source with zenas

This is an assessment of what it would take to reuse z88dk's assembly-language
library by assembling its source directly with zenas (rather than linking
z88dk's compiled objects or going through z88dk's own linker). It is based on
inspecting the actual library source, not on assumptions.

## What the z88dk library actually is

The z88dk classic and new libraries together contain about **15,400 `.asm`
source files**. They are not a collection of self-contained routines that can be
assembled to flat binaries and dropped into a program. They are a body of
**separately-compiled, relocatable objects** glued together by a linker.

Measured across the 15,411 library `.asm` files:

| Feature | Files using it | Share |
|---------|---------------:|------:|
| `SECTION` (named linker sections) | 14,979 | 97% |
| `PUBLIC` (export a symbol) | 15,207 | 99% |
| `EXTERN` / `GLOBAL` (import a symbol) | 12,188 | 79% |
| `defc` (z80asm constant/alias) | 4,764 | 31% |
| `INCLUDE` | 2,795 | 18% |
| `MACRO` | 10 | <1% |

Only **180 files (1.2%)** are truly flat - no `SECTION`, no `EXTERN`/`GLOBAL`.
Everything else assumes a linker.

A representative example, `stdlib/c/sccz80/abs.asm`:

```
SECTION code_clib
SECTION code_stdlib
PUBLIC abs
EXTERN asm_abs
abs:
   pop de
   pop hl
   push hl
   push de
   jp asm_abs
IF __CLASSIC
PUBLIC _abs
defc _abs = abs
ENDIF
```

This file cannot be assembled to a usable binary on its own: it exports `abs`,
imports `asm_abs` (defined in another file), places code in named sections, and
defines an alias with `defc`. It is one object among many that the linker
resolves and lays out.

## The key finding

**The blocker is not the instruction set, and not the macro system.** zenas
already assembles the Z80 and Z80N instructions these files use, byte-for-byte
correctly. The blocker is that the library is built on a **linker model that
zenas does not have**: named sections, exported/imported symbols, separate
compilation, and relocation. zenas is currently a flat-binary assembler - one
source (with textual `INCLUDE`s) to one binary, all symbols resolved internally.

This also means the C-style macro work is irrelevant to this goal. Reusing the
z88dk library is a linking-and-symbols problem, not a macro or compiler problem.

## What it would take

To assemble and use z88dk library source, zenas would need three things, in
increasing order of effort:

1. **z80asm directive compatibility (moderate).** Parse `SECTION`, `PUBLIC`,
   `EXTERN`, `GLOBAL`, `MODULE`, `defc`, `defb`/`defw`/`defm`/`defs`, and the
   `IF __CLASSIC`-style conditionals. This is a bounded parser extension and the
   easy part. `defb`/`defw`/`defm` map onto existing `DB`/`DW`/string handling;
   `defc` maps onto `EQU`; the rest are new.

2. **A relocatable object model (substantial).** zenas would have to stop
   emitting only flat binaries and instead emit objects that carry: section
   contents, a symbol table of exported (`PUBLIC`/`GLOBAL`) symbols, a list of
   unresolved external (`EXTERN`) references, and relocation records for
   addresses that depend on final placement. This is a new output format and a
   significant change to how the assembler tracks addresses (symbols are no
   longer all known at assembly time).

3. **A linker (the bulk of the work).** Something must collect multiple objects,
   assign final addresses to each section, resolve every `EXTERN` against the
   matching `PUBLIC`, and apply relocations. This is a real subsystem that zenas
   does not currently have in any form.

In short: reusing z88dk source the "assemble it ourselves" way means **giving
zenas a sections-and-symbols object model and writing a linker.** That is a
larger project than everything done on zenas so far, and it reimplements exactly
what z88dk's own `z80asm` linker already does.

## Options, honestly

- **Build the object model + linker.** The most self-contained and auditable
  end state (zenas controls the whole path from source to bytes, no dependency
  on z88dk's binary toolchain), and it would make zenas a genuinely more capable
  toolchain. But it is a major, multi-stage effort: object format, relocation,
  symbol resolution, section layout, plus the directive compatibility. Worth it
  only if a self-contained linkable toolchain is itself a goal (e.g. for a
  verifiable/auditable build pipeline), not merely as a means to borrow a few
  routines.

- **Port only what you need, by hand.** For a small number of specific routines,
  flatten them: resolve the `EXTERN`s manually, drop the sections, and assemble
  the result as ordinary zenas source. Cheap per routine, but it scales linearly
  and you inherit none of the library's breadth - and many routines pull in
  chains of `EXTERN` dependencies that make "just this one" expand quickly.

- **Reconsider Interpretation 1 (link z88dk's objects) or going through
  z80asm.** If the goal is access to the library's breadth rather than a
  self-contained source path, linking z88dk's existing objects - or letting
  z88dk's `z80asm` do the linking with zenas output as one input - reuses their
  decades of work instead of rebuilding the linker. The trade-off is a
  dependency on z88dk's toolchain/format and less end-to-end auditability.

## Recommendation

If the underlying want is "use z88dk's library in our programs," the
source-assembly route (Interpretation 2) is the most expensive way to get there,
because it forces zenas to grow a linker before a single library routine becomes
usable. It is the right route only if a self-contained, auditable
assembler-plus-linker is a goal in its own right - in which case the object
model and linker should be scoped as a deliberate project with its own
milestones, starting with the relocatable object format and a minimal two-object
link, long before pointing it at 15,000 library files.

If the want is just the routines, linking z88dk's compiled objects is far less
work and reuses the library as designed.
