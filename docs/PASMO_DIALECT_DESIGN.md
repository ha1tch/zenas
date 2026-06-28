# The `.pasmo` dialect mode - design

> Status: partially implemented. The dialect-mode machinery (a general `Dialect`
> lexer state, switched by `.pasmo` / `.zenas` as the stream is lexed and reset
> per tokenise), `$`-as-location-counter, and the `DEFM` alias are done and
> tested, and the `.pasmo` + `.include` idiom works end-to-end. Steps still to do:
> no-colon labels, pasmo `name MACRO args` syntax, and `PROC`/`LOCAL` scoping.
> The `Dialect` abstraction was built general (an enum, not a one-off boolean) so
> a future dialect slots in beside `DialectPasmo` rather than requiring a refactor.


## Purpose

A scoped compatibility mode that lets zenas ingest pasmo source. It follows the
exact mechanism already proven by `.MACRO_STYLE C`: a directive flips the lexer
(and a little of the parser) into a different dialect, streaming, until it is
turned off again. It is **not** an attempt to make zenas's native dialect a
superset of pasmo - that would create ambiguities (notably `$`). Instead, within
`.pasmo` scope the pasmo conventions hold unambiguously, and outside it zenas's
native conventions hold.

## The idiom this enables

Because includes are spliced as raw text *before* lexing (see "Why this composes"
below), the natural and powerful pattern is to wrap only the legacy region:

```
        ; ... native zenas source ...

.pasmo
        INCLUDE "pasmo_code.inc"     ; assembled under pasmo rules
.zenas

        ; ... native zenas source continues ...
```

The mode is **scoped**, not whole-file: `.pasmo` turns pasmo rules on, `.zenas`
(or end of source) turns them off. A file may switch back and forth. This means a
legacy pasmo include can be dropped into an otherwise-native project without
converting it, and without forcing the whole project into pasmo mode.

## Why this composes (the load-bearing fact)

Include expansion happens **once, before lexing**: an `INCLUDE "file"` line is
replaced by the raw text of the file, producing a single combined source string
that is then lexed. So by the time dialect matters (at the lexer), the included
file is already inline text. A `.pasmo` directive before the include therefore
governs the included text automatically - the include mechanism needs no
dialect awareness at all. This is the same structural reason the C-style lexer
mode works: dialect is a streaming lexer property, includes are pre-lexing text
splicing, and the two layer for free.

## Scope of the differences to absorb

From testing real pasmo constructs against zenas, the pasmo dialect must change
the following. All are syntactic/lexical - none require new semantic subsystems,
which is what makes `.pasmo` bounded (unlike a hypothetical `.sjasm`, which would
need `STRUCT`/`DEVICE`/`DUP` semantic machinery and is deliberately not pursued).

| pasmo construct | native zenas | `.pasmo` mode behaviour |
|-----------------|--------------|-------------------------|
| `$` as location counter | `$` is the hex prefix | `$` means the current address; pasmo hex is `0xNN`/`NNh`, so `$` is free to be the location counter |
| no-colon labels (`loop NOP`) | labels need `:` | a leading-column identifier followed by an instruction is a label |
| `DEFM "str"` | `DB "str"` | `DEFM` accepted as a string/byte directive alias |
| `name MACRO args` ... `ENDM` | `MACRO name(args)` ... `ENDMACRO` | pasmo macro definition/call syntax accepted |
| `PROC` / `LOCAL` / `ENDP` | (no scoping construct) | local-label scoping per pasmo |

`ORG`, `EQU`, `DEFB`/`DEFW`/`DEFS`, `IF`/`ENDIF`, colon labels, and the entire
instruction set already match pasmo and need no mode - they work identically in
both dialects. The instruction body of any pasmo program already assembles; the
mode only bridges the directive/meta-syntax gap.

## Mechanism, mirroring `.MACRO_STYLE C`

The C-style mode added a `cStyleMode` flag to the lexer, gated some token
decisions on it (semicolon-as-terminator inside braces), and detected the style
directive before tokenising. `.pasmo` is the same pattern:

1. **Lexer flag `pasmoMode`** (sibling of `cStyleMode`), set/cleared by `.pasmo`
   / `.zenas` directives encountered in the token stream. Like `braceDepth`, it
   is streaming state, reset at the start of tokenising.
2. **`$` handling**: when `pasmoMode` is set, a `$` not followed by a hex digit
   (or always, since pasmo hex is `0xNN`/`NNh`) lexes as a location-counter
   token rather than a hex prefix. This is the one change that *must* be a mode,
   because `$` cannot mean two things in one dialect.
3. **Label handling**: in `pasmoMode`, an identifier in label position without a
   trailing `:` is treated as a label. This is a parser-level relaxation gated on
   the mode.
4. **`DEFM` / pasmo macro syntax / `PROC`**: directive and macro-form aliases
   recognised only in `pasmoMode`, so they do not pollute the native grammar.

The `.pasmo` and `.zenas` directives themselves are no-ops for emitted code; they
only set the dialect flag. `.zenas` is the explicit "return to native" switch;
end of source implicitly returns to native for the next assembly.

## What stays native even inside `.pasmo`

The location counter, labels, `DEFM`, macros, and `PROC` are bridged. Everything
that already matches (instructions, `ORG`, `EQU`, byte/word directives,
conditionals) is *unchanged* - `.pasmo` does not re-implement them, it inherits
them. This keeps the mode small: it is a set of *deltas* from native, not a
second assembler.

## Deliberately out of scope

- **No `.sjasm` mode.** sjasmplus's distinctive constructs (`STRUCT`, `MODULE`,
  `DUP/EDUP`, `DEVICE`/memory paging) are semantic subsystems, not syntax skins.
  A `.sjasm` directive would be a thin label on a multi-year reimplementation and
  would promise a completeness it could not deliver. Where a specific sjasmplus
  construct proves genuinely useful, it should be added to zenas's *native*
  dialect on its own merits (e.g. a `DUP`-style repetition), not as
  sjasmplus-compatibility. Constructs that overlap zenas's own vocabulary
  (`MODULE` ~ packages) are skipped entirely - zenas already has the better-shaped
  version.
- **No attempt to be a pasmo superset in native mode.** The ambiguities (`$`)
  are precisely why this is a scoped mode and not a base-grammar change.

## Order of work

1. `pasmoMode` lexer flag + `.pasmo`/`.zenas` directive detection and handling,
   reset per tokenise (mirror `cStyleMode`).
2. `$`-as-location-counter under `pasmoMode` (the one mandatory mode change).
3. `DEFM` alias under `pasmoMode`.
4. No-colon label acceptance under `pasmoMode`.
5. pasmo `name MACRO args` / `ENDM` macro syntax under `pasmoMode`.
6. `PROC`/`LOCAL`/`ENDP` scoping under `pasmoMode`.
7. Tests: a `.pasmo`-wrapped block using `$`, no-colon labels, `DEFM`, and a
   pasmo macro; the `.pasmo` + `.include` idiom with a real pasmo include;
   switching back to `.zenas` and confirming native rules resume (e.g. `$1234`
   hex works again).
8. Docs: a dialect-mode section in the manual; note in MACRO_STATUS / readiness.

Each step is independently testable and the mode starts useful after step 2
(the `$` bridge alone unblocks a large fraction of pasmo source).

## Measured compatibility

For the measured state of pasmo and sjasmplus source compatibility - which
constructs assemble, which fail, why, and what each non-zenas construct does -
see `docs/DIALECT_COMPATIBILITY.md`. That document records the corpus diagnostic
so it need not be repeated.
