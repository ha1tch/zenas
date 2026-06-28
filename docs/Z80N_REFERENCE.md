# Z80N (ZX Spectrum Next) instruction reference

> **Status: implemented and validated.** Enable with `--next` or `--cpu=Z80N`
> (default is `--cpu=Z80`, Z80N off). Every Z80N form is verified byte-for-byte
> against **sjasmplus** (the de facto ZX Spectrum Next assembler, and the
> reference implementation of Z80N encoding) by `tools/check_z80n_coverage.sh`.
> The base Z80 set additionally matches sjasmplus across all 516 checked forms.

Implementation reference for adding Z80N support to zenas. Every opcode below is
taken from the official SpecNext wiki (Extended Z80 instruction set) and
cross-checked against the implementers' list in z88dk issue #312. zenas does not
currently support Z80N; this document is the ground truth for that work.

All Z80N instructions are `ED`-prefixed. Gating them behind a `--next` flag (or a
`Z80N`/`CPU Z80N` directive) is recommended so a stray `MUL` in plain Z80 source
is still an error.

## Verified opcode table

| Mnemonic | Encoding (hex) | Bytes | Operand notes |
|----------|----------------|-------|---------------|
| `SWAPNIB` | `ED 23` | 2 | none |
| `MIRROR A` | `ED 24` | 2 | operand is always `A`; accept `MIRROR` and `MIRROR A` |
| `TEST n` | `ED 27 n` | 3 | 8-bit immediate |
| `BSLA DE,B` | `ED 28` | 2 | operands fixed (`DE,B`) |
| `BSRA DE,B` | `ED 29` | 2 | operands fixed |
| `BSRL DE,B` | `ED 2A` | 2 | operands fixed |
| `BSRF DE,B` | `ED 2B` | 2 | operands fixed |
| `BRLC DE,B` | `ED 2C` | 2 | operands fixed |
| `MUL D,E` | `ED 30` | 2 | operands fixed (`D,E`); accept `MUL` and `MUL D,E` |
| `ADD HL,A` | `ED 31` | 2 | second operand is `A` |
| `ADD DE,A` | `ED 32` | 2 | second operand is `A` |
| `ADD BC,A` | `ED 33` | 2 | second operand is `A` |
| `ADD HL,nn` | `ED 34 lo hi` | 4 | 16-bit immediate, little-endian |
| `ADD DE,nn` | `ED 35 lo hi` | 4 | 16-bit immediate, little-endian |
| `ADD BC,nn` | `ED 36 lo hi` | 4 | 16-bit immediate, little-endian |
| `PUSH nn` | `ED 8A hi lo` | 4 | **16-bit immediate, BIG-endian** (unique - see note) |
| `OUTINB` | `ED 90` | 2 | none |
| `NEXTREG n,n` | `ED 91 reg val` | 4 | two 8-bit immediates: register number, then value |
| `NEXTREG n,A` | `ED 92 reg` | 3 | 8-bit immediate register number; value comes from `A` |
| `PIXELDN` | `ED 93` | 2 | none |
| `PIXELAD` | `ED 94` | 2 | none |
| `SETAE` | `ED 95` | 2 | none |
| `JP (C)` | `ED 98` | 2 | none (fixed operand `(C)`) |
| `LDIX` | `ED A4` | 2 | none |
| `LDWS` | `ED A5` | 2 | none |
| `LDDX` | `ED AC` | 2 | none |
| `LDIRX` | `ED B4` | 2 | none |
| `LDPIRX` | `ED B7` | 2 | none |
| `LDDRX` | `ED BC` | 2 | none |

## Critical encoding notes

1. **`PUSH nn` is big-endian.** It is the only Z80 operand encoded high-byte
   first. `PUSH $1234` assembles to `ED 8A 12 34`, not `ED 8A 34 12`. Every other
   16-bit immediate on the Z80 (including `ADD HL/DE/BC,nn` above) is little-endian.

2. **`ADD rr,A` vs `ADD rr,nn` share a mnemonic but differ by operand type.**
   `ADD HL,A` is `ED 31` (2 bytes); `ADD HL,$im16` is `ED 34 lo hi` (4 bytes).
   The encoder must dispatch on whether the second operand is the register `A` or
   an immediate. Note these collide in mnemonic with the standard `ADD HL,rr`
   (`09/19/29/39`) and `ADD A,r` - the operand types disambiguate.

3. **Fixed-operand instructions.** `MUL D,E`, `MIRROR A`, the `BSxx/BRLC DE,B`
   group, and `JP (C)` take only their hard-wired operands. Accept the canonical
   form; optionally accept the bare mnemonic (`MUL`, `MIRROR`) as the Next
   community commonly writes them, but reject wrong operands (`MUL B,C`).

4. **`NEXTREG` has two forms** distinguished by the second operand: an immediate
   (`ED 91 reg val`) or the register `A` (`ED 92 reg`). The register number is
   always an 8-bit immediate, never a CPU register.

## Provenance / rejected data

- Primary source: <https://wiki.specnext.dev/Extended_Z80_instruction_set>
  (opcode table), cross-checked against
  <https://github.com/z88dk/z88dk/issues/312> (implementers' early list).
- The deprecated `zxa` project's Z80N table was found to be **wrong** for the
  register-pair adds (it had `ADD HL,A = ED 7C`, `ADD DE,A = ED 77`,
  `ADD BC,A = ED 76`; the correct values are `ED 31/32/33`). It also omitted
  `ADD rr,nn` (`ED 34/35/36`), `JP (C)` (`ED 98`), and `LDWS` (`ED A5`), and
  listed a `CUP = ED B5` ("Copper") instruction that does **not** appear in the
  official table. Do not copy any opcode from zxa; this document supersedes it.

## Suggested implementation approach

1. Add a `--next` CLI flag (and/or a source directive) that enables a Z80N
   instruction table on top of the base set. Default off.
2. Register the fixed-operand and no-operand forms as simple `ED`-prefixed
   emits (mirrors the existing block-I/O registrations).
3. Add dedicated encoders for the three operand-bearing shapes:
   `TEST n` / `NEXTREG n,n` / `NEXTREG n,A` (8-bit immediates),
   `ADD rr,nn` (little-endian 16-bit), and `PUSH nn` (big-endian 16-bit).
4. Extend the coverage checker (`tools/`) with the Z80N forms, guarded by the
   `--next` flag, and compare against a Next-aware reference assembler
   (sjasmplus with `--zxnext`, or CSpect's assembler) rather than pasmo, which
   does not know Z80N.
