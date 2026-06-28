# Macro support: current state and what is needed

This is an assessment of zenas's macro subsystem as it stands, and what would be
required to make traditional macros and C-style macros genuinely usable. It is
based on testing the current build, not on the code's intent.

## Summary

zenas ships a sizeable macro subsystem (~2,900 lines across `macro_*.go` and
`c_style_converter.go`) supporting two styles, selected by a `.MACRO_STYLE`
directive:

- **Traditional** - `MACRO NAME(params) ... ENDMACRO`, called as `NAME(args)`.
- **C-style** - C-like `void f() { asm { ... } }` function syntax, transpiled to
  traditional macros before assembly.

Both are **partially working and not production-ready**. The core building
blocks exist, but several essentials are broken or missing, and the C-style path
emits debug output. The subsystem should be treated as a prototype: the
architecture is reasonable, but it needs focused repair before it can be relied
on.

## What works today (traditional macros)

Phase 1 of the macro work, plus the nested-call fix, made the traditional macro
system usable for the common cases:

| Capability | Status |
|------------|--------|
| Single, multiple, and zero arguments (`ONE(5)`, `TWO(5,6)`, `Z()`) | works |
| Local labels unique per expansion (a loop macro called twice) | works |
| Nested calls, including nested multi-argument and zero-argument calls | works |
| `.MACRO_STYLE` selection between TRADITIONAL and C | works |
| C-style functions, including with parameters, transpiled to traditional macros | works |
| Width markers on parameters (`uint8_t`/`uint16_t`), shared by both styles | works |
| Width-signature checking of literal arguments (mismatch is an error) | works |
| Return-width contract: typed function must return its declared width, emits RET | works |
| Packages: `.PACKAGE` affiliation, qualified calls, ambiguity errors | works |
| C-style debug output | removed |

All of these are covered by tests in `zenas test`.

## Known limitation: parameter names share the register/condition namespace

Macro expansion is textual: a parameter is substituted wherever its name appears
in the body. If a parameter is named the same as a register or condition code,
the body will use the register/condition, not the argument. For example:

```
MACRO BAD(a)        ; 'a' also names register A
    LD A, a         ; expands to LD A, A - NOT LD A, <argument>
ENDMACRO
```

Avoid single-letter parameter names that collide with `A B C D E H L` (registers)
or `Z C NC NZ P M PE PO` (condition codes). Use descriptive names (`val`, `count`,
`xx`) instead. This is inherent to substitution-style macros and matches how
comparable assemblers behave; it is a usage caveat, not a defect.

## Still unimplemented: C-style calling conventions

C-style function *parameters* now work by passing them through to the underlying
traditional macro (textual substitution). What is **not** implemented is a real
calling convention - passing arguments in registers or on the stack per a defined
ABI, with return values. The C-style mode is structured-asm sugar over traditional
macros, not a C compiler. For C on Z80, z88dk is the mature option (see
`docs/Z88DK_REUSE.md`).


## Status of the planned work

**Phase 1 (traditional macros) - done.** The debug output was removed,
multi-argument and zero-argument call parsing was fixed, local-label uniqueness
was fixed, nested calls (including nested multi-argument calls) were fixed via a
macro-aware body parser, and tests were added. Traditional macros now cover the
common need: parameterised code templates with safe local labels.

**Phase 2 (C-style calling conventions) - deliberately not pursued.** C-style
parameters work by textual substitution through the underlying traditional
macro, which is enough for structured-asm use. A real calling convention
(register/stack argument passing, return values) was assessed and declined: it
reimplements the core of a C compiler, and z88dk already does this well on the
same targets. C-style mode is positioned as sugar over traditional macros, not a
C compiler.

## A note on syntax

zenas's traditional macro syntax is `MACRO NAME(params)` with call `NAME(args)`,
which differs from the pasmo / sjasmplus convention `NAME MACRO params` with call
`NAME args` (no parentheses). If compatibility with existing ZX Spectrum macro
source is a goal, supporting the pasmo-style form is a separate item worth
considering - but it interacts with the same call-site parsing work in gap 1, so
the two are best designed together.
