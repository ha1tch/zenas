# ZENAS_READINESS.md - zenas readiness for assembling ZX Opal

> **ACCEPTANCE MILESTONE REACHED (zenas 0.1.0).** zenas assembles the full ZX
> Opal kernel (`kernel.asm` + all 10 subsystem includes, ORG 0x5CAD) to a
> **byte-identical** 6530-byte binary, with all 400 symbol addresses matching
> pasmo exactly. zenas is a verified drop-in for pasmo on ZX Opal's real source.

A tracking checklist of everything ZX Opal's source actually requires from an
assembler, measured against zenas's current state. The goal: zenas can assemble
ZX Opal **byte-identically to pasmo** with **zero source changes**. When every row
is met, zenas becomes a clean drop-in replacement and the switch is a build-script
edit, not a rewrite.

Status legend: ☐ not yet · ◧ partial / unreliable · ☑ working

The "ZX Opal use" column is measured from the current source (not hypothetical):
how many call sites or why it is load-bearing. The "zenas state" column was
**tested empirically against the v3 build** (the first version that compiles and
runs here - it needed only a `go.mod` with module path `github.com/ha1tch/zen80`).
Results below are observed, not inferred from notes.

## 1. Directives (load-bearing - the build cannot proceed without these)

| Directive | ZX Opal use | Notes | zenas state |
|-----------|-------------|-------|-------------|
| `INCLUDE` | 16 sites; kernel.asm pulls in all 11 subsystem files, and the multi-target plan needs `INCLUDE "layout.asm"` | **Done.** Pasmo-style textual inclusion: paths resolve relative to the including file, single flat namespace, forward references across the include boundary resolve (expansion runs before both passes). Nested includes, cycle detection, and trailing comments all tested. Bare and dotted `.INCLUDE` both accepted. | ☑ |
| `ORG` | 5 sites (kernel, detecttest, reloc_stub) | **Tested: bare `ORG` works.** | ☑ |
| `EQU` | 35 sites; all layout constants and config | **Tested: bare `NAME EQU value` works and binds correctly** (incl. 16-bit address constants like `KERNEL EQU $5CAD`). | ☑ |
| `DB` | 1003 sites; font tables, strings, data | **Tested: bare `DB` works**, incl. multi-value lists and strings. | ☑ |
| `DW` | 110 sites; vtable, pointers, sysvars | **Tested: bare `DW` works.** | ☑ |
| `END` | optional; detecttest uses `.END` | ZX Opal kernel does not require it; low priority. | ◧ |

## 2. Conditional assembly (required for the multi-target build)

| Feature | ZX Opal use | Notes | zenas state |
|---------|-------------|-------|-------------|
| `IF` / `ELSE` / `ENDIF` | the multi-target build selects the per-machine layout via `IF TARGET_48K ... ELSE ... ENDIF` over an included config | **Done.** `IF`/`IFDEF`/`IFNDEF`/`ELSE`/`ENDIF` via a per-pass condition stack (nesting supported). Conditions evaluate against symbols known at that point; an undefined symbol is 0 (false) consistently in both passes, so branch decisions never diverge. `--define=NAME[=VAL]` pre-seeds a symbol from the command line (build-tag style): select layout with `--define=TARGET_48K`. | ☑ |
| Symbol-defined test (e.g. `IF SYMBOL`) | the config include sets one target symbol to 1 | Equality test `IF TARGET = 1` or truthiness `IF TARGET_48K` both acceptable. | ☐ |

## 3. Instructions (every mnemonic ZX Opal actually emits)

ZX Opal uses a conventional Z80 subset. Most are ordinary and presumed working;
the rows below single out the ones the zenas notes flag as **missing**, which are
load-bearing in ZX Opal.

| Instruction | ZX Opal use | Where | zenas state |
|-------------|-------------|-------|-------------|
| `LDIR` | 9 sites | the relocator stub copies the kernel to its run address; block moves | ☑ (ED B0) |
| `RST` | 5 sites | vector table / ROM-space variant entry points | ☑ (C7|n) |
| `IN A,(C)` | keyboard + detect | `keyboard.asm` reads the matrix; `detect.asm` reads port 0xFE | ☑ (ED 40|r<<3); parser now recognises (C) as a port operand |
| `OUT (C),A` | paging | `paging.asm` writes ports 0x7FFD / 0x1FFD with the 16-bit address in BC | ☑ (ED 41|r<<3) |
| `EX` / `EXX` | a few sites | register exchange | verify |
| `DJNZ` | loops | counted loops | verify |
| `BIT` / `SET` / `RES` | bit ops | flags, masks | verify |
| Rotates: `RLC`/`RR`/`RLA`/`RRA`/`RLCA`/`RRCA` | several | bit manipulation | verify |
| `ADC`/`SBC`/`SCF`/`CPL`/`DAA` | arithmetic | hex conversion, math | verify |

Full mnemonic inventory actually used by ZX Opal (for a complete pass): ADC ADD
AND BIT CALL CP CPL DAA DEC DI DJNZ EI EX EXX IN INC JP JR LD LDIR OR OUT POP PUSH
RET RLA RLC RLCA RR RRA RRCA SBC SCF SRL SUB XOR. No exotic / undocumented opcodes
are used, so the instruction surface is small and bounded.

## 4. Number formats and literals

| Format | ZX Opal use | Notes | zenas state |
|--------|-------------|-------|-------------|
| `0x` hex (`0x5CAD`) | 140+ sites; the dominant hex form | **Tested v3: works** (`LD HL,0x5CAD` -> `21 AD 5C`). | ☑ |
| `%` binary (`%01111000`) | 20 sites; wallpaper texture, bit masks | **Tested v3: works** (`LD A,%10101010` -> `3E AA`). The v2 notes calling it unreliable are out of date. | ☑ |
| Decimal (`23725`) | throughout | plain integers | verify |
| `&` hex (Spectrum style) | ~0 sites; effectively unused | zenas notes flag `&8000` as unrecognised, but ZX Opal does **not** rely on it - low priority. | ☐ (not needed) |
| `H` suffix (`5CADh`) | 0 sites | not used by ZX Opal. | n/a |
| Char literals (`'A'`) | a few | within DB/comparisons | verify |
| String literals (`DB "ZX Opal"`) | 30 `DB`-string sites; banners, version | **Tested v3: works** (`.DB "ZX Opal",0` and multi-value `.DB 1,2,3` both encode correctly). | ☑ |

## 5. Syntax compatibility (the zero-source-change goal)

| Concern | ZX Opal form | Notes | zenas state |
|---------|--------------|-------|-------------|
| Bare vs dot-prefixed directives | ZX Opal uses **bare** `ORG`/`DB`/`DW`/`EQU`/`INCLUDE`; zenas examples use **`.ORG`/`.DB`/...** | **Done.** zenas now accepts both bare and dotted `ORG`/`DB`/`DW`/`EQU`, including the `NAME EQU value` label form (two fixes: label detection in the parser, and EQU-value binding in the assembler matching both `.EQU` and `EQU`). `INCLUDE` still pending (own row, §1). | ☑ |
| Label syntax | plain `name:` labels; no local/relative label sigils | ZX Opal uses only named labels, so no special-label support is required. | verify |
| Label + expression operands | `CALL vtable+N*3`, `LD HL,kernel_end+255`, `LD (rowcount+1),A` | **Done.** Symbol arithmetic (`+ - * /`, correct precedence) now parses in instruction immediate operands, indirect operands, and IX/IY displacements. Verified against the real ZX Opal forms: `LD HL,base+255`, `LD BC,CELLMAP_SIZE-1`, `ADD A,ATOM_HDR+7` (8-bit, value-checked), `LD HL,base+N*3`, `LD (rowcount+1),A`. Note: `&` is a hex prefix in this lexer (ZX-BASIC style), not bitwise-AND; ZX Opal does not use bitwise operators in operands. | ☑ |
| `EQU` to expression | `GUI_CEILING EQU 0xA000`, derived constants | constant expressions evaluated at assembly time. | verify |
| Comments | `;` to end of line | standard. | verify |
| Case | mixed-case labels (`sv_host_machine`), uppercase mnemonics | **Done.** zenas is now case-sensitive for symbols and case-insensitive for mnemonics/registers/conditions/directives, matching pasmo. Verified the ZX Opal `line_h`/`LINE_H` pair (a runtime byte vs a compile-time constant in print.asm) assembles as two distinct symbols. Note: the symbol file therefore now preserves original case, so `gen_memmap.py` lookups match without modification. | ☑ |

## 6. Output / toolchain contract (what the build pipeline consumes)

| Requirement | Current (pasmo) | Notes | zenas state |
|-------------|-----------------|-------|-------------|
| Raw binary output | `pasmo --bin in.asm out.bin` | zenas has `assemble in.asm out.bin`; the invocation differs but is workable. | ◧ |
| **Symbol table file** | `pasmo --bin in.asm out.bin out.sym` emits an `EQU`-style sym file | **Done.** `--sym` (default path = output basename + `.sym`) or `--sym=path` emits pasmo-format lines `NAME\t\tEQU 0XXXXH`, sorted, parseable by `gen_memmap.py` (verified). Symbol names now preserve original case (zenas is case-sensitive for symbols), so `gen_memmap.py`'s case-sensitive lookups match directly - no tooling change needed. | ☑ |
| Exit status on error | non-zero on failure | `build.sh` uses `set -e`; zenas must fail loudly. | verify |
| Deterministic output | byte-stable | required for byte-identical comparison against pasmo. | verify |

## 7. The acceptance test

zenas is ready to replace pasmo for ZX Opal when:

1. `zenas` assembles `src/kernel.asm` (with all its `INCLUDE`s) to a binary that is
   **byte-identical** to the pasmo `zxok.bin`.
2. It assembles `tools/reloc_stub.asm` byte-identically to the pasmo `stub.bin`.
3. It emits a symbol file `gen_memmap.py` can parse unchanged (or a documented
   adapter is written).
4. The conditional-assembly mechanism (§2) selects a layout include correctly, so
   the multi-target build produces `zxok_48`/`zxok_128`/`zxok_p3` byte-correctly.
5. The full `make` + `release.sh` pipeline runs green with zenas swapped in.

When 1-5 hold, switching is a one-line change in `build.sh` / `Makefile`
(`pasmo --bin` -> `zenas assemble`), with no edits to any `.asm` source.

## 8. v3 build notes and newly-discovered gaps (tested)

The v3 source compiles and runs here with only a `go.mod` added (module path
`github.com/ha1tch/zen80`, `main.go` at root importing the `assembler/`
subpackage). It assembles real machine code and prints a symbol table. Testing it
directly surfaced facts the v2 WORKINPROGRESS notes did not capture:

**Better than the notes claimed:**
- `%` binary, `0x` hex, string `DB`, and multi-value `DB` all work correctly.

**Encoder gaps beyond the four headline instructions** - NOW CLOSED (each tested
byte-correct):
- `EX DE,HL` (EB), `EX AF,AF'` (08), `EX (SP),HL` (E3) - done. `AF'` required a
  lexer fix (trailing apostrophe on prime registers) and a parser register-set
  entry.
- `EXX` (D9) - done.
- 16-bit `ADC HL,rr` (ED 4A|p<<4) / `SBC HL,rr` (ED 42|p<<4) - done.
- `BIT`/`RES`/`SET n,(HL)` and rotate/shift `(HL)` forms (CB base|...|6) - done.
- `IN r,(C)` / `OUT (C),r` (ED 40|r<<3 / ED 41|r<<3) - done; the parser now
  recognises `(C)` as a port operand.
- Also added: `LDI`/`LDD`/`CPI`/`CPD`/`CPIR`/`CPDR`, `NEG`, `RETI`/`RETN`,
  `RRD`/`RLD`, `IM 0/1/2`, `RST n`.

**Parser gaps:**
- **Bare directives fail**: `ORG`/`DB`/`DW`/`EQU` without the dot prefix produce a
  parse error. Only `.ORG`/`.DB`/`.DW`/`.EQU` are accepted. This is the
  single highest-leverage fix - accepting bare directives lets ZX Opal's existing
  source assemble unmodified (see §5).
- **Label+expression operands**: DONE - `CALL base+3`, `LD HL,base+255`,
  `ADD A,ATOM_HDR+7`, `LD HL,base+N*3`, `LD (rowcount+1),A` all assemble
  byte-correctly. (Historical note: these previously gave "expected newline ...
  got PLUS".) ZX Opal relies on these (`CALL vtable+N*3`,
  `DW CELLMAP_ADDR`, aligned-address math).

**Symbol output:**
- The symbol table is printed to stdout / available via `--json`, but there is no
  `.sym` *file* output matching pasmo's third-argument sym file. `gen_memmap.py`
  and `release.sh` parse that file, so either zenas grows a sym-file writer or a
  small adapter converts its JSON symbol map to the expected format.

**Net assessment:** v3 is a real, working assembler with a clean Go codebase and a
correct core encoder for common register/immediate instructions - a solid
foundation. But the gap to assembling ZX Opal unmodified is wider than the v2
notes implied: it spans the parser (bare directives, operand expressions, INCLUDE,
conditionals) and the encoder (EX/EXX, 16-bit ADC/SBC, `(HL)` bit ops, `(C)`
ports). None is architecturally hard; all are bounded and enumerated above. The
priority order below still holds, with these additions folded into items 2-3.

## Priority order (suggested)

1. **`INCLUDE`** - nothing assembles without it.
2. **Bare-directive acceptance** (§5) - unlocks ingesting the existing source unmodified.
3. **The four missing instructions** - `LDIR`, `RST`, `IN A,(C)`, `OUT (C),A`.
4. **Symbol-file output** (§6) - unblocks the memmap/release tooling.
5. **`IF/ELSE/ENDIF`** - unblocks the multi-target build.
6. **`%` binary reliability** - correctness across the source.
7. **IX/IY indexed addressing** - DONE. Implemented all forms ZX Opal uses,
   verified byte-identical to pasmo: `LD r,(IX+d)`, `LD (IX+d),r`, `LD IX,nn`,
   `ADD IX,rr`, `PUSH/POP IX`, `LD SP,IX`, plus the ALU/INC/DEC indirect forms
   (`CP (IX+d)`, and the previously-missing `(HL)` ALU forms `ADD A,(HL)`,
   `AND (HL)`, `CP (HL)`, `INC/DEC (HL)`). Fixed a latent bug where `LD IX,nn`
   silently encoded as `LD HL,nn` (no DD prefix). IY variants use FD throughout.

Items 1-4 get a single-target ZX Opal assembling byte-identically; 5 adds the
multi-target build; 6 is correctness hardening. The `&`-hex and `H`-suffix formats
are **not** required by ZX Opal and can be deprioritised for this purpose.
