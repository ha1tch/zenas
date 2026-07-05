# Reproducing the seam on other assemblers

The [design rationale](ZENAS_DESIGN.md) describes the seam: a typed
linkage layer — width-checked signatures, calls, assignment, namespaced
packages — built natively into the assembler. A fair question is how hard it
would be to obtain the same thing elsewhere. This table records an assessment.

The purpose is calibration, not competition. Each assembler below is a
well-regarded tool with its own design goals, and none of them set out to
provide a typed linkage layer — the scores measure distance from a goal that
is zenas's, not theirs. They serve here as parameters of reference: they
locate the seam in the design space and show which of its parts existing
facilities can approximate and which parts genuinely require assembler
support.

## What is being scored

"The same" decomposes into four components of unequal difficulty:

1. **Seam semantics** — typed signatures with width checking, calls,
   assignment.
2. **Namespaced packages** with qualified calls and bare-unless-ambiguous
   resolution.
3. **C-flavoured surface syntax.**
4. **Verification apparatus** — integrated run/assert, with test constructs
   legal only in test files.

Components 3 and 4 are roughly equally out of reach everywhere: the syntax
requires an external transpiler on any conventional assembler, and no
mainstream assembler executes its own output, so assertions mean external
emulator wiring. The scores below are therefore dominated by components 1
and 2: how far each assembler's native facilities go towards a checked,
namespaced linkage layer.

**Scale:** 1 = the capability is native; 10 = nearly everything must be
built outside the tool. Scores assume a competent user exploiting the assembler's full macro
and scripting facilities, without modifying the assembler itself.

## The table

| Assembler | Difficulty | What helps | What blocks |
|---|---:|---|---|
| **zenas** | **1.0** | The seam is native: `.MACRO_STYLE C` width-checked signatures, `.PACKAGE` namespacing, `zenas assert` with `.EXPECT`/`.MATCH` confined to `*_test.asm` | Nothing structural; the remaining cost is adopting the layering discipline |
| sjasmplus | 4.0 | Embedded Lua scripting (arbitrary compile-time checks, metadata tables), `MODULE` namespacing, strong macros | Width/return checking must be hand-built in Lua; calls stay macro-shaped; no test/production file separation |
| ca65 (cc65) — *6502, comparator only* | 4.5 | Real scopes, `.assert`, rich macro language, linker segments | No scripting layer, so per-function type metadata is laborious; wrong CPU for a Z80 project |
| z88dk z80asm | 5.0 | A genuine linker: modules and libraries make package-swapping ordinary relinking | Weak macro system; no mechanism for width checking — type information has nowhere to live |
| Macroassembler AS (asl) | 5.5 | Very capable macro language, `SECTION` scoping, string/symbol manipulation, `ERROR` directive | Tracking per-function signatures across macros is possible but tortured; no namespaced call resolution |
| RASM | 5.5 | Powerful expression and loop directives, module-prefixed labels, assertion support | No scripting; type metadata exists only by convention |
| Glass | 6.0 | Proper lexical scoping (`PROC`, sections), clean design | Small macro system; assertion vocabulary too thin for width contracts |
| WLA-DX | 6.5 | Sections, `.ENUM`/`.STRUCT`, multi-platform linker | Macro language too weak for signature checking; namespacing is prefix discipline |
| vasm | 6.5 | Solid conventional macros, multiple syntax front-ends | Nothing resembling scoped packages or compile-time metadata |
| zmac / tniASM | 8.0 | Competent classical assemblers | Classical macros only; every seam component would be an external preprocessor |
| pasmo | 8.5 | Clean, predictable, good `EQU`/`IF` | Minimal macros, flat namespace, no assertions — the seam would be almost entirely external tooling |

## Reading the table

Three observations the scores compress:

- **sjasmplus at 4.0 is the nearest reference point**, and the reason is
  instructive: Lua is a general-purpose escape hatch. Reaching the seam there
  means embedding a scripting language — which is precisely the class of
  dependency the seam exists to avoid. The score is low because sjasmplus is
  powerful, and the gap remains because that power is of a different kind.
- **z88dk's score hides an asymmetry.** The package-swap half of the
  architecture is cheap there (it is ordinary linking), while the typed
  interface half is nearly impossible. The seam's distinguishing element is
  the *checked contract*, not the modularity.
- **No score covers the verification apparatus.** Test-only directive
  enforcement and the integrated assemble–run–assert loop require changes to
  the assembler itself; on every tool above, the honest difficulty for the
  full apparatus is "fork it or rewrite it".

## Macro-engine and table-driven assemblers

A separate class deserves its own section, because these tools differ from the
conventional assemblers above in kind, not merely in degree: they treat the
instruction set itself as user-definable. That makes them the most instructive
reference points of all, and the scores need more context than a table row can
carry.

| Assembler | Difficulty | What helps | What blocks |
|---|---:|---|---|
| fasmg | 3.5 | The assembler *is* a macro engine with no built-in instruction set; `namespace` blocks; `calminstruction` compiled macros able to attach metadata to symbols; user-defined `err` diagnostics, so a violated contract can fail assembly with a sensible message. Community Z80 include packs exist (e.g. jacobly0/fasmg-z80) | The seam would be a small compiler written in fasmg's macro language, and the instruction pack is itself a dependency to adopt or maintain before seam work starts; no execution of output |
| Retro Assembler | 6.0 | Modern ergonomics: parameterised macros with default values, `Loop`/`If`/`While` directives, multi-CPU targets, editor integration | Facilities are classical in kind beneath the ergonomics; no signature metadata, no package-style namespacing |
| customasm | 6.0 | Declarative ISA definition (`#ruledef`/`#subruledef`) with **typed, bit-width-checked operand parameters** (`{value: i8}`), `assert()` guards with custom messages, `#fn` expression functions, `#assert`, conditional compilation | The typing lives at the operand level, not the call level: there is no function or call abstraction for code, no namespacing, and no macro system (users resort to an external C preprocessor). Z80's parenthesised indirect operands — `(HL)`, `(IX+d)` — sit close to a known ambiguity in its pattern parser |

The score assigned to fasmg carries an assumption worth stating: it presumes a
usable community Z80 instruction pack. Where one must be written or
substantially maintained, the effective difficulty rises by a point or more.

### What this class teaches

These tools are worth studying rather than merely scoring, for three reasons.

**fasmg shows the seam's dependency question from a third angle.** sjasmplus
reaches seam territory by *embedding* a language (Lua); fasmg reaches it by
*being* one — its macro system is powerful enough to define entire instruction
sets, so it is certainly powerful enough to implement width-checked linkage.
But in both cases the dependency the seam avoids does not disappear; it
changes address. zenas's position is that the linkage layer should be a fixed,
specified part of the assembler, not a program written in it — smaller,
less expressive, and therefore stable.

**customasm shows that typed, checked encoding is natural at the ISA layer.**
Its operand parameters are declared with bit widths and validated with
assertion guards — the same idea as the seam's width checking, applied one
layer down, at the boundary between mnemonic and encoding rather than between
caller and routine. That the idea appears independently at both layers is
mild evidence it is a good one. Its instruction-set-as-data approach is also
the logical extreme of retargetability, and a useful reference for any future
work on additional instruction-set tiers.

**The class shares one absence with everything else in this document.** None
of these tools executes its own output. However far their definitional power
extends, the verification apparatus — run, assert, test-file discipline —
remains outside their design space, which is a useful reminder of which parts
of zenas are reproducible elsewhere with effort and which parts are not.

A brief note on a neighbouring current: modernised classical assemblers such
as Nestor80 (a MACRO-80-compatible assembler and linker with module
namespacing and modern string handling) show that the Z80 tooling world is
actively renovating its traditional lineage too. These belong with the
conventional class above — roughly z88dk territory on this table's scale —
but they indicate that module-style namespacing, at least, is becoming an
expected amenity.

## Confidence

Scores for sjasmplus, pasmo, z88dk, and ca65 rest on detailed familiarity.
Scores for RASM and Glass are assessed from their documented feature sets and
could move by roughly half a point on closer inspection. The macro-engine
section's entries were checked against current project documentation and
repositories at the time of writing: the existence of community fasmg Z80
include files, customasm's typed rule parameters and assertion guards, and
Retro Assembler's directive set are verified; fasmg's score additionally
assumes a maintained instruction pack, as noted above. The table is a
considered estimate, not a measurement; corrections grounded in the
respective manuals are the appropriate way to revise it.
