# Packages for macros - design

> Status: implemented. `.PACKAGE` affiliation, structured `(package, name)`
> keying, qualified calls, bare-unless-ambiguous resolution, and
> instruction-precedence are all in place and tested. The `--tag`/tier-selection
> step (using the package as the unit a tier is selected in) remains future work.


## Purpose

Two purposes, in order of when they matter:

1. **Now: disambiguation.** Two packages may each define a macro called `add`;
   qualifying the call as `math.add(5, 2)` selects one unambiguously. This also
   removes the macro-versus-instruction collision (a qualified `math.add` is
   never confused with the `ADD` mnemonic), and lets a primitive library define
   short names like `rotate` or `add` without shadowing instructions for callers.

2. **Later: the organizing unit for primitive tiers.** The portability model has
   per-architecture primitive libraries selected by `--tag`. A package is the
   natural unit for such a library: `math` with a Z80 implementation and a 6502
   implementation, the same package name, the tier chosen by tag. The design must
   therefore carry the package as real structure now, so this can grow without a
   rewrite.

Because of (2), the package is **not** implemented as a string prefix folded into
the macro name. It is a distinct identity component: a macro is keyed by
`(package, name)`, not by `"package.name"`. The qualified textual form
`math.add` is only surface syntax over that structured identity.

## Affiliation

Modelled on Go: a file declares the package it belongs to. Zenas has no
directory-aware build unit yet (it assembles a single source, with `INCLUDE`
pulling files in textually), so the Go convention "one package per directory"
cannot mean a compiled directory today. It is approximated at file scope:

- A `.PACKAGE name` directive declares the package for the macros defined after
  it, until end of file or the next `.PACKAGE`.
- Macros defined with no `.PACKAGE` in effect belong to the unnamed default
  package (current behaviour - existing code keeps working unchanged).
- When zenas later gains a project/directory model, "one package per directory"
  becomes a real rule layered on top of this file-level affiliation, not a
  contradiction of it.

`.PACKAGE` is a no-op for emitted code; it only sets the affiliation used when
registering subsequent macro definitions.

## Resolution

A call resolves as follows:

- **Qualified call** `math.add(...)`: look up `add` in package `math` exactly. If
  it does not exist, error (`add not found in package math`).
- **Bare call** `add(...)`: collect every package that defines `add`.
  - Exactly one: resolve to it.
  - More than one: error, listing the packages, requiring qualification
    (`add is ambiguous: defined in math and string; qualify the call`).
  - None: not a macro; fall through to normal instruction handling (so bare
    `ADD` remains the instruction).

This "bare works unless ambiguous" rule is more permissive than Go (which always
qualifies imported names) and fits an assembler's terser style. Its one
non-obvious property: ambiguity is decided against the set of packages visible at
the call, so pulling in a second `INCLUDE` that defines a clashing name can turn
a previously-valid bare call ambiguous. The error message must name the
conflicting packages so this is self-explanatory rather than mysterious.

The macro-versus-instruction collision is resolved by the same rule: a primitive
library defines `add` in package `prim`; callers either write `prim.add` (never
confused with the mnemonic) or bare `add` only where it is unambiguous and
intended. Bare `add` with no matching macro stays the `ADD` instruction.

## Internal representation

- `MacroDefinition` gains a `Package string` field (empty = default package).
- The macro table keys on the pair. The minimal change that preserves the
  structured identity is a map keyed by package then name, or a composite key
  type `{Package, Name}`; either keeps package as a first-class component rather
  than a substring. (Concatenating into `"math.add"` is rejected: it would have
  to be unpicked when `--tag` later selects between same-named packages across
  tiers.)
- `Define` records the current package; `Lookup` gains a qualified form
  (exact `(package, name)`) and a bare form (search by name across packages,
  returning the match or signalling ambiguity).

## Syntax is already free

The lexer already accepts `.` inside an identifier, so `math.add` tokenises as a
single identifier and round-trips through symbol resolution today (verified). No
tokeniser change is needed; a leading `.` still starts a directive, an embedded
`.` is part of the name. The work is entirely in the table and resolution layer,
which is where it belongs.

## Deliberately out of scope for the first version

- No `IMPORT`/`USE` bringing a package's names into unqualified scope - the
  bare-unless-ambiguous rule covers the immediate need without it.
- No directory/project build model - file-level affiliation only, designed to
  accept the directory convention later.
- No visibility/export rules (Go's capitalisation) - every macro in a package is
  reachable as `package.name`.
- No `--tag`/tier selection wiring yet - the representation is built to support
  it, but selecting an implementation by tag is the later step (item 2 of the
  deferred roadmap), not part of introducing the package concept.

## Order of work

1. `Package` field on `MacroDefinition`; table keyed on `(package, name)`.
2. `.PACKAGE` directive sets the current affiliation for subsequent definitions.
3. Qualified lookup (`math.add` exact) and bare lookup (search-with-ambiguity).
4. Wire call resolution to use qualified vs bare paths; good ambiguity errors.
5. Tests: two packages same macro name, qualified call, bare unambiguous, bare
   ambiguous (error), bare-falls-through-to-instruction, default-package
   backward-compatibility.
6. Docs: manual section, MACRO_STATUS, and a note in the deferred roadmap that
   the package is the intended tier unit.
