# Changelog

All notable changes to zenas are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

Entries touching assembler code are tagged by layer -- `[encoder]`,
`[lexer]`, `[parser]`, or `[macro]` -- as a single line-item prefix,
e.g. `[encoder] The DDCB/FDCB instruction group: ...`. This exists
specifically so a downstream fork with its own front end (a different
lexer/parser serving multiple target architectures uniformly, for
example) can tell at a glance whether a fix is portable as a diff
(`[encoder]`, since `pkg/z80enc` and similar pure, primitive-typed
packages hold zenas's actual opcode arithmetic independent of any
particular operand representation) or needs independent verification
against that fork's own front end (`[lexer]`/`[parser]`/`[macro]`,
since those layers are deliberately not shared -- a genuinely
different design concern per front end, not a gap). Entries before
this convention started (anything prior to 0.9.0) are untagged.

## [0.11.2] - 2026-09-05

Found while porting Oakhollow, a real pasmo-sourced ZX Spectrum game, as
a build-tool trial independent of the original pasmo/sjasmplus corpus
diagnostic behind `docs/DIALECT_COMPATIBILITY.md` -- confirmation that
gap is still incomplete even after that pass.

### Added

- `[parser]` pasmo's `LOW`/`HIGH` unary byte-extraction operators
  (`LOW x` = bits 0-7, `HIGH x` = bits 8-15). Previously absent from
  the expression grammar entirely -- not gated behind `.pasmo` mode,
  simply unimplemented, so `CP LOW label` failed with a bare parse
  error ("expected newline at end of line") rather than a dialect
  refusal. Implemented in both operand-parsing entry points
  (`parseIdentifierOperand`, the bare-instruction-operand path that
  `CP LOW label` actually uses, and `parsePrimaryExpression`, reached
  from directive arguments, parens, and expression continuations) so
  the two can't drift, plus the two expression evaluators
  (`evaluateExpression`, `evalConditionExpr`). Reserved as keywords
  wherever an operand is parsed, matching pasmo's own reservation of
  the words. See `assembler/low_high_test.go`.

## [0.11.1] - 2026-09-04

Closes the loop on v0.11.0's clock-count review: the automated
coverage tooling itself had gaps in its own test lists, which is
exactly why the missing-instruction bugs fixed in v0.11.0 survived
undetected for as long as they did. Updated the generators so a
future regression on any of these forms would actually be caught,
then ran both reference-assembler cross-checks live (pasmo, sjasmplus)
to confirm -- not just re-read the existing claims.

### Fixed

- `tools/gen_instr_coverage.py` never included `LD IX,(nn)`,
  `LD (nn),IX`, or `EX (SP),IX`/`IY` in its own generated test list
  -- confirmed the direct cause of those bugs going undetected: the
  tool that's supposed to catch this class of thing never asked the
  question. Added, along with the 5 ALU-on-`(IX+d)`/`(IY+d)`
  mnemonics (`ADC`/`SBC`/`SUB`/`AND`/`XOR`/`OR`) the generator had
  also never covered -- previously only `ADD A,(IX+d)` and
  `CP (IX+d)` were exercised there, leaving the other 5 checked only
  by a one-off manual test during the v0.11.0 review, not by this
  generator's own repeatable cross-check.
- `tools/z80n_coverage.txt` was missing `LDIRSCALE` (`ED B6`) --
  same category of gap, same fix.
- `docs/INSTRUCTION_SET.md`'s coverage table and prose, and the
  matching claims in `docs/Z80N_REFERENCE.md` and
  `docs/DIALECT_COMPATIBILITY.md`, all cited stale counts (516/29/545)
  predating this update. Now 534/30/546, each re-verified live rather
  than incremented by arithmetic: `PASS=534 MISSING=0 MISMATCH=0`
  against `pasmo`, `PASS=32 FAIL=0` against the live `sjasmplus`
  oracle (not the baked-in fallback bytes).

## [0.11.0] - 2026-09-04

A full clock-count review, cross-checked against reference Z80 timing
data (map.grauw.nl/resources/z80instr.php for the base set; a Z80N
core built from FPGA-simulation measurements, not quoted references,
for the Next extensions). What started as a metadata-accuracy pass
surfaced a severe correctness bug and several genuinely missing
instructions along the way -- both are fixed here too, not left for
a separate pass.

### Fixed

- **`[encoder]` A real correctness bug, not a cycle-count issue**:
  `RLC`/`RRC`/`RL`/`RR`/`SLA`/`SRA`/`SRL`/`SLL`/`BIT`/`RES`/`SET` on a
  half-index register (`IXH`/`IXL`/`IYH`/`IYL`) previously assembled
  to truncated, invalid bytes (e.g. `RLC IXH` -> `DD CB 04`, missing
  the mandatory displacement byte `DD CB`/`FD CB` always requires)
  instead of being rejected. Confirmed against multiple independent
  sources that this combination has no valid Z80 encoding at all --
  `DD CB`/`FD CB` always addresses `(IX+d)`/`(IY+d)` memory, never a
  register directly. `ApplyHalfIndexPrefix` now rejects it explicitly.
- **`[encoder]` ~34 shared-registration cycle-count errors**: any
  template shared between `(HL)` and `(IX+d)`/`(IY+d)`, between a
  plain register and a half-index register, or between
  `BC`/`DE`/`HL`/`SP` and `IX`/`IY`, reported one fixed number that
  was only correct for one of the cases -- the `LD r,r'`/`LD r,n`/
  `LD r,(HL)` family, ALU-on-register/indirect, `INC`/`DEC` (register
  and indirect), 16-bit loads and arithmetic, `PUSH`/`POP`, `JP`
  indirect, `IN`/`OUT` register-port forms, and the `BIT`/`RES`/`SET`/
  rotate-shift indirect forms.
- **`[encoder]` 3 isolated wrong values**: `RET cc` reported 5 (the
  not-taken value) while `JR cc`/`CALL cc`/`DJNZ` all consistently
  report the taken/longer value -- inconsistent with the codebase's
  own established convention. `INIR`/`INDR`/`OTIR`/`OTDR` and
  `LDIR`/`LDDR`/`CPIR`/`CPDR` all reported 16 (the last-iteration
  value) instead of 21 (the repeat value), same inconsistency.
- **`[encoder]` 7 Z80N cycle-count errors**: `OUTINB`, `LDIX`, `LDWS`,
  `LDDX`, `LDIRX`, `LDPIRX`, and `LDDRX` were all wrongly sharing
  `SWAPNIB`'s 8 T-state value instead of their real
  16/14/16/21/21/21.
- **`[encoder]` `LD IX,(nn)` / `LD IY,(nn)` / `LD (nn),IX` /
  `LD (nn),IY` had no encoding at all** -- confirmed a genuine gap,
  not a design decision: `z80enc.EncodeLDrpAddr`/`EncodeLDAddrRp`
  simply never had an `IX`/`IY` case. Fixed (`DD 2A`/`FD 2A` and
  `DD 22`/`FD 22`, the same base opcode as the `HL` form under its
  own index prefix).
- **`[encoder]` `EX (SP),IX` / `EX (SP),IY` had no encoding at all**
  -- same category of gap, fixed the same way (`DD E3`/`FD E3`).
- **`[encoder]` `IN F,(C)`**: a second, real spelling for the same
  `ED70` opcode `IN (C)` already covered -- previously rejected by
  this codebase (a genuine gap, not a design decision: `F` wasn't in
  `register8Set` at all). `F` now parses only in this one context
  (`z80enc.EncodeIN` special-cases it); every other 8-bit-register use
  of `F` (`LD F,A`, `INC F`, etc.) is still correctly rejected.
- `[encoder]` `encodeLDrr`'s adapter was discarding `z80enc`'s
  specific error (e.g. "invalid 8-bit register") and substituting a
  hardcoded "registers I and R may only be used as..." message
  regardless of the actual problem -- fixed to propagate the real
  error.
- Removed a duplicated `LD` 16-bit registration block (four templates
  registered twice, verbatim) -- harmless dead code since the dispatch
  loop's first match always won, but confirmed exact and removed
  rather than left in place.

### Added

- **`[encoder]` `InstructionTemplate.CyclesFunc` and its accessor
  `CyclesFor`**: an optional per-template function computing the real
  T-state count from the actual resolved operands, for the ~34 cases
  above where a single fixed `Cycles` value cannot be correct. Purely
  additive -- the other ~120 templates never needed it and still use
  plain `Cycles`.
- **`[encoder]` `Encoder.EncodeWithCycles`**: returns the real cycle
  count alongside the encoded bytes -- the only way to observe
  `CyclesFor`'s result from outside this package, since `Encode`
  itself only ever returned bytes and which template matched wasn't
  otherwise exposed.
- **`[encoder]` `LDIRSCALE` (`ED B6`)**: a real, documented Z80N
  instruction (a plain repeating copy, despite what its name
  suggests) that was completely absent from this codebase -- added
  alongside the timing fixes above rather than left for a separate
  pass.
- `assembler/cycles_test.go`: 67 test cases (`TestCyclesFor`,
  `TestCyclesForZ80N`) exercising every fixed category via
  `EncodeWithCycles`, plus the `EX (SP),IX`/`LD IX,(nn)`-family gap
  fixes and the two isolated-value fixes.
- `pkg/z80enc`: regression coverage for the half-index/CB-prefix
  correctness fix (`TestApplyHalfIndexPrefix`) and the new
  `EncodeEXSPHL("IX")`/`("IY")` forms.
- `assembler/ddfdcb_copy_test.go`: `TestBareINOUT`/
  `TestFRegisterOnlyValidForINFC` cover the `IN F,(C)` fix and confirm
  `F` stays rejected everywhere else an 8-bit register could appear.

## [0.10.0] - 2026-09-03

`pkg/z80enc`'s extraction, complete. Every one of `encoder.go`'s
`encode*` functions (57 total) plus the half-index-register handling
(`applyHalfIndexPrefix`, called from the top-level dispatch loop
rather than registered per-mnemonic, but the same category of pure
opcode knowledge) now delegates to `pkg/z80enc` -- verified with a
tree-wide sweep confirming zero remaining functions still compute
bytes inline. What started as a pilot on the group that originally
motivated this (0.9.0's DDCB/FDCB fix) and an interim checkpoint
(0.9.1, 11 of 57) is now the whole encoder.

### Added

- `[encoder]` **`pkg/z80enc/reg16.go`**: the 16-bit register-pair
  family -- `EncodeRp16Imm`, `EncodeRp16Shift4`, `EncodeADDHLrp`,
  `EncodeStackOp`, `EncodeLDrpAddr`, `EncodeLDAddrRp`, `EncodeED16`.
- `[encoder]` **`pkg/z80enc/indirect.go`**: the indirect-memory family
  -- `EncodeIndirectByte`, `EncodeIndirectImm8`,
  `EncodeLDRegFromIndirect`, `EncodeLDIndirectFromReg`.
- `[encoder]` **`pkg/z80enc/branch.go`**: the jump/call/branch family
  -- `EncodeAddr16`, `EncodeCondAddr16`, `EncodeRelBranch`,
  `EncodeCondRelBranch`, `EncodeJPIndirect`, `EncodeCondOnly`,
  `EncodeRST`.
- `[encoder]` **`pkg/z80enc/misc.go`**: `EncodeEX`, `EncodeEXSPHL`,
  `EncodeIM`, `EncodeOUT`, `EncodeIN`, `ApplyHalfIndexPrefix`,
  `EncodeSimple`, `EncodeLDSPHL`, `EncodeINFC`, `EncodeOUTC0` -- the
  undocumented `IN (C)`/`OUT (C),0` forms included, since they're the
  same category of shareable opcode knowledge as everything else here.
- `[encoder]` **`pkg/z80enc/z80n.go`**: the whole Z80N extension
  family -- `EncodeFixedOperand`, `EncodeFixedOperandPair`,
  `EncodeZ80NJPc`, `EncodeZ80NTEST`, `EncodeZ80NADDrrA`,
  `EncodeZ80NADDrrnn`, `EncodeZ80NPUSHnn`, `EncodeZ80NNEXTREGnn`,
  `EncodeZ80NNEXTREGnA`.
- `[encoder]` `pkg/z80enc/reg8.go` gained `EncodeImm8` -- the
  ALU-immediate shape (`SUB n`/`AND n`/etc). Barely needs a function
  at all (base is already fixed per mnemonic, no register lookup
  involved); included for consistency with the rest of the package
  rather than because the arithmetic is complex.
- `[encoder]` `encoder.go`'s corresponding 46 functions (the remainder
  after 0.9.1's 11) are now thin adapters, each verified independently
  against hand-computed or documented values before being wired in,
  then reverified with a full build+test pass -- the same discipline
  every prior batch in this package followed, at the same
  per-function granularity, not relaxed for the larger remaining
  scope.
- `[encoder]` `pkg/z80enc` now carries 49 test functions of its own
  across all 8 source files (1602 lines total) -- every function a
  downstream consumer would import directly has standalone coverage,
  not just indirect coverage through `encoder.go`'s own test suite.
- **`release.sh` now runs a badcode scan automatically**, first,
  unconditionally, when `repoman` is on `PATH` -- matching
  `gorepoman`'s own `relcore` convention. Soft-skips with a clear
  warning if `repoman` itself isn't available, rather than either
  silently skipping or hard-failing the release over an optional
  tool. Verified all three paths directly (clean pass, soft-skip, and
  a real failure with a planted marker, confirming the correct exit
  code without any piping that could mask it) before trusting it.
  Found and fixed via this: the vendored, unused Python `repoman/`
  copy (removed in this same release) had been leaking an
  AI-attribution mention into every checkpoint shipped so far,
  uncaught, since this project's own release process never ran that
  gate before now.
- **`docs/ARCHITECTURE.md`**, new -- how the code itself is organised
  (the assembly pipeline, `pkg/z80enc`'s own file-by-file breakdown,
  `pkg/build`), distinct from `ZENAS_DESIGN.md`'s existing "why the
  language looks this way" scope. Filled a genuine gap (no doc
  described the code-level structure before this, not something that
  had gone stale) rather than replacing anything.
- **`README.md`** gained a "Library" section introducing `pkg/z80enc`
  -- previously undocumented there entirely.

### Fixed

- `[encoder]` `pkg/z80enc/ddfdcb.go`'s private `indexPrefix`
  (duplicating what's now the shared, exported `IndexPrefix` in
  `registers.go`) removed; `CopyRegister` exported too, matching the
  naming convention the rest of the package settled into once it grew
  past the original single-file pilot.
- **`docs/INSTRUCTION_SET.md` had a genuinely wrong claim**: its "Not
  implemented" section explicitly named `RLC (IX+d),B` and `SLL`/
  `SL1` as unencoded -- exactly the instructions this same release
  adds. Corrected, with a new section covering everything the base
  documented/half-register/Z80N counts don't include (those tools
  can't cross-check several of these at all), pointing to
  `tools/golden_vectors.txt` as the verification source.
- **`docs/Z80N_REFERENCE.md` contradicted itself**: its own status
  banner said "implemented and validated" while its body text, never
  reconciled after Z80N support actually shipped, still said "zenas
  does not currently support Z80N". Fixed.
- **`.gitignore` was missing `build/`, `.ed-journal.json`
  (`repoman`'s own edit journal), and `zenas-v*.zip`** -- all three
  were showing as untracked. Added.
- Stale artefacts removed: an old checkpoint zip, the `build/`
  directory's leftover binary.

## [0.9.1] - 2026-09-03

Interim checkpoint: `pkg/z80enc`'s extraction, still in progress
(11 of 57 encode functions moved so far -- see this entry's own notes
below for exactly which), released as its own version rather than
left as uncommitted work between checkpoints.

### Added

- `[encoder]` **`pkg/z80enc/registers.go`**: the shared register/
  condition-code mappings the rest of the package's encode functions
  build on -- `Reg8`, `Reg16`, `StackReg16`, `Condition`,
  `JRCondition`, `IndexPrefix` -- exported so a consumer isn't limited
  to the DDCB/FDCB group alone.
- `[encoder]` **`pkg/z80enc/reg8.go`**: `EncodeReg8Shift3`,
  `EncodeReg8Imm`, `EncodeReg8`, `EncodeCBReg8`, `EncodeCBBitReg8`,
  `EncodeLDrr` -- the 8-bit-register-operand shape family (`INC`/`DEC
  r`, `LD r,n`, `LD r,r'` including its `I`/`R` ED-prefixed special
  cases, `ALU r`, `CB r`, `BIT`/`RES`/`SET b,r`). Verified against
  hand-computed values before touching `encoder.go` at all.
- `[encoder]` `encoder.go`'s own `encodeLDrr`, `encodeLDrn`,
  `encodeINCr`, `encodeDECr`, `encodeCBr`, `encodeBITbr`, `encodeRESbr`
  are now thin adapters over the above, each verified with a full
  build+test pass before moving to the next. 11 of 57 encode functions
  now extracted in total (the 4 from the DDCB/FDCB pilot plus these 7);
  the remaining ~46 (16-bit register-pair ops, the rest of the
  indirect-memory family, jump/call/branch, `IN`/`OUT`, `EX`, `IM`,
  half-index handling, the whole Z80N extension set) have not been
  moved yet -- this entry is honestly a checkpoint mid-extraction, not
  a claim of completion.
- `[encoder]` `pkg/z80enc`'s own duplicate `indexPrefix` (private,
  only in the DDCB/FDCB file) consolidated into the new shared
  `IndexPrefix`; `CopyRegister` (the DDCB/FDCB copy-destination
  validator) is now exported too, matching the naming convention of
  everything else in the package.

### Fixed

- **The vendored, unused Python `repoman/` copy removed entirely.**
  Confirmed not referenced by `release.sh`, `syncver.sh`, or the
  `Makefile` anywhere -- dead weight, and its own `CHANGELOG.md`
  contained an AI-attribution mention that a manual `badcode` check
  caught (this project's own `release.sh` doesn't run that gate
  automatically the way `gorepoman relcore`-based projects do, so this
  had been present in every checkpoint shipped so far without being
  caught). `release.sh`'s zip-exclude list also gets `repoman/*` added
  defensively, matching its existing `build/*`/`attic/*` entries.

## [0.9.0] - 2026-09-02

### Added

- `[encoder]` **`pkg/z80enc`**: pure, dependency-free Z80 opcode-encoding
  functions -- currently the DDCB/FDCB group (`EncodeCBIndirect`,
  `EncodeCBBitIndirect`, `EncodeDDFDCBCopy`, `EncodeDDFDCBBitCopy`),
  taking plain Go primitives (register names as strings, displacements
  as `int8`, bit numbers as `uint8`) with zero dependency on zenas's
  own `ResolvedOperand` type. Extracted specifically so a fork with its
  own front end can import the real opcode arithmetic directly instead
  of re-deriving it from release notes each time a fix lands -- the
  exact problem the DDCB/FDCB fix itself caused for one such fork,
  since the logic ported cleanly but the code could not. `encoder.go`'s
  own `encodeCBHL`/`encodeCBbHL`/`encodeDDFDCBCopy`/`encodeDDFDCBbCopy`
  are now thin adapters over this package, not duplicate logic --
  verified independently before and after the refactor, with zero
  behaviour change. Other instruction groups have not been moved here
  yet; this is a pilot on the group that motivated it, not a claim
  that the whole encoder has been restructured.
- `[encoder]` **`tools/golden_vectors.txt`**: verified mnemonic/bytes
  pairs as plain data (`SOURCE|HEXBYTES`, one instruction per line),
  for a downstream fork to check its own encoder against directly,
  without hand-transcribing bytes from release notes. 531 lines: every
  form in `tools/instr_coverage_list.txt` (zenas's own assembled
  output for the documented instruction set, already cross-checked
  against pasmo as part of this project's own release process -- this
  file exports that already-trusted result rather than re-running the
  check) plus the undocumented instructions added in this release,
  which pasmo cannot cross-check at all. Regenerate with
  `tools/gen_golden_vectors.sh`.
- `[encoder]` **The DDCB/FDCB instruction group**: `RLC`/`RRC`/`RL`/`RR`/`SLA`/
  `SRA`/`SRL` and `BIT`/`RES`/`SET n` on `(IX+d)`/`(IY+d)`, previously
  entirely unencodable -- confirmed no DDCB/FDCB handling existed
  anywhere in the encoder before this. zen80 already executes this
  group correctly; zenas simply had no way to assemble it, forcing a
  byte- and cycle-costly `LD`-preserves-flags workaround at every call
  site that needed flag-preserving bit access on an indexed operand
  (reported by the zenpoint project: 21 such sites). The opcode byte
  is identical to the existing `(HL)` form's own; only the index
  prefix and displacement byte are inserted before it, so the fix
  extends `encodeCBHL`/`encodeCBbHL` directly rather than duplicating
  them.
- `[encoder]` **The DDCB/FDCB undocumented copy-to-register forms**: `RLC (IX+d),B`,
  `SET n,(IX+d),A`, etc. -- the operation is performed on the indexed
  memory operand exactly as the documented form, and the result is
  also copied into the named register. Verified against the exact
  worked examples multiple independent Z80 documentation sources
  agree on (`DDCB0100` = `RLC (IX+1),B`, `DDCB02FF` = `SET 7,(IX+2),A`),
  not just internally-consistent opcode arithmetic. `IXH`/`IXL`/`IYH`/
  `IYL` are explicitly rejected as copy destinations -- confirmed real
  hardware always copies into plain H/L here, regardless of which
  DD/FD prefix is active for the indexed memory operand itself, so
  accepting the half-index names would be misleading notation for
  identical bytes. `BIT` has no copy form and none was registered for
  it -- confirmed (same sources) that all 8 register-field encodings
  of a DDCB/FDCB `BIT` instruction are identical, since `BIT` never
  writes a result anywhere but the flags.
- `[encoder]` **`SLL`/`SLI`/`SL1`** (synonyms for the same opcode, `CB 30`-`37`):
  the undocumented 8th shift, confirmed "quite commonly used" despite
  never being officially documented. All forms: register, `(HL)`,
  `(IX+d)`, `(IY+d)`, and its own copy-to-register variants -- nearly
  free to add, since it reuses the exact encode functions `SLA`
  already used, just a different opcode base.
- `[encoder]` **The bare undocumented `IN (C)` and `OUT (C),0`** (`ED70`/`ED71`).
  `IN (C)` reads the port and sets flags exactly like the documented
  `IN r,(C)` forms, but discards the byte read. `OUT (C),0` outputs a
  literal zero, no source register. (`IN F,(C)`, a second widely-used
  alias for the same `ED70` opcode, was not added -- it would need
  special-casing `F` as a pseudo-register for this one instruction;
  `IN (C)` alone already covers the opcode.)

### Known, minor, and inconsequential right now

- The cycle-count metadata for the new `(IX+d)`/`(IY+d)` DDCB/FDCB
  registrations inherits the existing `(HL)` registration's value
  (15) rather than the correct 23 T-states for the indexed forms,
  since both share one `encodeCBHL`/`encodeCBbHL`-based registration.
  Checked whether this matters: it doesn't, currently -- confirmed
  `template.Cycles` is stored but never read anywhere in this
  codebase; the CLI's own cycle-estimate output uses a separate,
  explicitly-disclaimed "very rough, byte-count-based" heuristic
  instead. Noted here rather than silently left inaccurate, in case
  something wires the real field up to real output later.

### Fixed

- `[lexer]` **A real hex-literal parsing bug, reported by the zenpoint project**:
  a traditional-notation hex literal whose first significant digit is
  itself a letter -- `0BCH`, `0FFh`, `0DEADh` (the leading `0` exists
  specifically so it doesn't read as an identifier) -- was misread as
  a broken `0b`-prefixed binary literal, or split into a stray `0d`
  radix-marker token plus a leftover identifier. Three separate call
  sites shared the same root flaw (checking "does this start with
  0b/0d?" without ever checking "does it end in H?"): the lexer's own
  token dispatch, a second, independent bug in the exported
  `ParseNumber`, and the spaced-hexdump `0d` radix marker (`.DB 0d 222
  173`), the last found and fixed while verifying the first two, not
  in the original report. All three fixed with the same disambiguation
  (a hex-digit run terminated by `H`/`h` is a hex literal, not a radix
  marker or binary prefix); genuine `0b`-binary literals and the
  legitimate spaced-hexdump forms are unaffected -- verified directly,
  not just re-running the existing suite.

## [0.8.1] - 2026-08-27

### Added

- **`--zxnext`**: recognised everywhere `--next` is (assemble, run,
  assert) as a plain synonym -- sjasmplus's own spelling for enabling
  the Z80N instruction set. No behaviour change from `--next`; purely
  a compatibility alias so a command line written for sjasmplus works
  unmodified.

## [0.8.0] - 2026-08-21

### Added

- **`OPT ZXNEXT` directive**: a source file can now enable Z80N (ZX
  Spectrum Next) instructions itself, the same as the command line's
  `--next` / the library's `EnableZ80N()` -- so a source that needs Z80N
  instructions carries that requirement in itself, rather than depending on
  whoever invokes the assembler to remember the right flag. This matches
  the inline-directive convention SNasm and other Z80N-aware assemblers
  already use, closing a real cross-assembler compatibility gap: a source
  written with SNasm's own `OPT ZXNEXT` line previously had to be edited
  before it would assemble under zenas at all (an unrelated but genuinely
  unrecognised directive, not merely a different way of spelling the same
  thing) -- it now assembles unchanged. Unknown `OPT` options, and a bare
  `OPT` with no option name, are both clear assembly errors rather than
  silently ignored.
- First test coverage for `assembler` package (`assembler/opt_test.go`,
  five cases covering the new directive) -- there was none before this.

## [0.7.5] - 2026-07-05

A documentation release. No changes to the assembler, emulator, or output.

### Added

- **Design rationale** (`docs/ZENAS_DESIGN.md`): the single idea connecting
  the C-style macro layer, packages, build tags, and the test harness — the
  typed linkage layer ("the seam"), what it deliberately includes and omits,
  the practical guidelines that follow for structuring seam-styled programs,
  and the commitments zenas development makes as a consequence.
- **Seam comparison** (`docs/SEAM_COMPARISON.md`): a scored assessment of how
  far other assemblers' native facilities go towards the same design, covering
  the conventional Z80 assemblers and the macro-engine/table-driven class
  (fasmg, customasm, Retro Assembler), with what each class teaches.

### Changed

- **README rework.** The introduction now leads with the assemble–run–assert
  capability and a three-line example of the whole loop; the two previously
  inconsistent documentation lists are consolidated into one complete list
  (adding `docs/RUNTIME.md` and `docs/DIALECT_COMPATIBILITY.md`, which were
  previously unlinked); the byte-for-byte pasmo claim gains a scope note
  pointing at the measured dialect-compatibility reference; and an explicit
  Requirements section states the Go 1.25 minimum for source builds.

## [0.7.3] - 2026-06-29

### Added

- **Packaged-program tutorial** (`docs/PACKAGED_PROGRAM_TUTORIAL.md`): a worked
  walkthrough that builds and tests a real program using packages, the traditional
  macro system, and the `.EXPECT`/`.MATCH` test harness.
- **C-style variable assignment.** A C-style function body may declare a variable
  (`uint8_t total;`) and assign to it — either a literal (`total = 42;`) or the
  result of a call (`total = math.add(20, 22);`). Storage is allocated once after
  the code (never in the execution path), and the value is stored via the
  accumulator, so the variable can be read back with `LD A, (total)`.
- **Symbol names in `.EXPECT` / `--expect` memory targets.** A memory assertion's
  address may now be a symbol (a label or variable) as well as a numeric address:
  `--expect="(total)=42"` and `.EXPECT (total)=42` both work, resolving the symbol
  against the assembled program.
- **`.MACRO_MODE INLINE | SINGLETON` directive.** `INLINE` (the default) expands a
  macro's body at every call. `SINGLETON` emits the body once as a routine and
  turns each instantiation into a `CALL` — smaller when a sizeable body is reused.
  Parameters are passed through fixed memory slots: each call writes its arguments
  to per-parameter slots and then `CALL`s, and the body reads them from the slots
  (so a parameter `value` is read as `(value)`). A parameterised singleton is not
  re-entrant (the slots are fixed locations); zenas detects recursion among
  singleton macros and rejects it, directing you to `INLINE`. See
  `examples/example15-macro-mode.asm`.

### Fixed

- **C-style functions now produce runnable programs.** A `return` previously
  emitted a `RET` even though function bodies are expanded inline, so an inlined
  return aborted the *caller* and a `main()` calling another function stopped
  early. A `return` now delivers its value in the convention's return register
  (`A`) without a `RET`; standalone calls also load their arguments into the
  convention's parameter registers, which they did not before. A C-style program
  with function calls now runs to completion and computes correctly.
- **Identifier collisions are now impossible.** A macro parameter, and a C-style
  function, variable, or parameter, may no longer be named after a register or
  condition code (`a`, `b`, `hl`, `nz`, ...); such a name was substituted into
  the body and silently assembled as that register. A macro (traditional or
  C-style) whose name collides with an instruction mnemonic must now be placed in
  a package and called qualified (`math.add`) — a bare mnemonic-named macro could
  never be reached and is rejected with a message directing the author to a
  package. All such cases now fail at definition time with a clear error instead
  of producing wrong code or a cryptic encoding failure. Two bundled examples
  that used register-named C-style parameters were updated accordingly.
- **IN/OUT no longer crash the run harness.** Running code that uses port I/O
  previously dereferenced a nil I/O device and panicked the process. The harness
  now attaches a null I/O device (reads return 0xFF, writes are discarded), so
  such programs run to completion.

## [0.7.2] - 2026-06-29

### Added

- **Zenas programming guide** (`docs/ZENAS_PROGRAMMING.md`): an orientation for
  users coming from another assembler — where zenas is familiar, where it
  differs, and what it uniquely enables (the in-process execution, assertion, and
  go-style test layer, C-style typed macros, packages, build-tag bitmasks, and
  scoped dialect modes).

### Changed

- README now documents the `run`, `assert`, and `test` commands with usage
  examples; previously only `assemble` and `build` were shown.
- Manual command synopsis lists all subcommands (`assemble`, `build`, `run`,
  `assert`, `test`, `version`, `help`); previously it listed only `assemble`.

## [0.7.1] - 2026-06-29

### Changed

- README reworked to lead with the commands and their usage, with the
  capability summary moved below.

### Removed

- Internal design, status, and roadmap notes that documented work in progress
  during development and are superseded by the user-facing manual: the macro
  status assessment, the package and pasmo-dialect design notes, the encoder
  strategy note, the deferred-work roadmap, and the library-reuse assessment.
  User-facing reference (the manual, instruction-set and Z80N references, the
  runtime and dialect-compatibility guides) is unchanged.

## [0.7.0] - 2026-06-29

### Added

- **`build` command**: assembles the source and packages the result into
  loadable artifacts, complementing `assemble` (which emits a raw binary).
  Outputs are selected with `--tap`, `--tzx`, `--sna`, and `--z80`:
  - Snapshots (`.sna`, `.z80` v3) carry full machine state and run immediately
    at the entry point set by `--start`. The snapshot is overlaid on the booted
    state of the model chosen with `--model` (48k, 128k, plus2, plus2a, plus3).
  - Tape images (`.tap`, `.tzx`) encode the code as a CODE block. `--loader`
    prepends a BASIC auto-run loader so `LOAD ""` runs the code; the loader is
    generated as ASCII BASIC and tokenised via the zentools tokeniser.
  - The recommended workflow is to use `.z80`/`.sna` snapshots for development
    testing and tapes for wider distribution.
- `--start` accepts an address (`0x8000`, `$8000`, `32768`) or a source label;
  `--sp` overrides the stack pointer (default `0xFF00`) with a collision warning
  when it would sit inside or just above the code; `-o` sets the output basename.
- Spectrum tape names, +3DOS 8.3 names, and host filenames are handled as three
  separate namespaces rather than being conflated.

### Changed

- Adds a direct dependency on `github.com/ha1tch/zentools` for the tape,
  snapshot, and BASIC tokenisation formats.

## [0.6.3] - 2026-06-28

### Changed

- Cleaned up the examples directory. Removed six developer scratch files
  (`simple_test*.asm`, `debug_test.asm`, `debug_main_only.asm`) whose `_test.asm`
  names collided with the test-file convention and which no longer assembled.
  Rewrote stale and developer-facing comments across the remaining examples to be
  accurate and user-facing (e.g. C-style examples no longer carry "may not work
  yet" notes for features that now work, and instruction examples no longer use
  internal "Test Pass N" headers).

## [0.6.2] - 2026-06-28

### Added

- Radix-input data forms for `.DB`/`.DW`/`.DM`. A `0x` (hex) or `0d` (decimal)
  marker introduces values the directive emits by its own width and order:
  `.DB 0x600DF00D` and `.DB 0x 60 0D F0 0D` give the bytes `60 0D F0 0D`;
  `.DW 0x 600D F00D` gives little-endian words; `.DB 0d 96 13 240 13` is the
  decimal equivalent. A group that does not fit the directive's unit is an error
  rather than being silently re-split. Useful for entering binary blobs.
- `.MATCH <location>, <data>` test directive: asserts that a span of memory at an
  address or symbol matches the bytes produced by an ordinary `.DB`/`.DW`/`.DM`
  data operand (which is assembled through the real data path, so all data forms
  work). Reports the first differing offset on failure. Test-only.

### Changed

- Test discovery now keys on the `test_` label prefix. Every `test_*` routine is
  a test and must carry at least one `.EXPECT`/`.MATCH`; one with none fails. An
  `.EXPECT`/`.MATCH` on a non-`test_` label is an assembly error. A `*_test.asm`
  file with no `test_` routines emits a warning (and passes).
- A test routine may carry several `.EXPECT`/`.MATCH` directives, all checked as
  one test.

## [0.6.1] - 2026-06-28

### Added

- Go-style test files. A file named `*_test.asm` may include the program under
  test, define `test_*`-style routines, and annotate each with an `.EXPECT`
  directive stating expected post-conditions. `zenas assert file_test.asm` (with
  no `--expect`) discovers every annotated routine, runs each via its label,
  checks its expectations, and reports go-test-style PASS/FAIL with a summary and
  a non-zero exit on failure.
- `.EXPECT` directive: binds to the nearest preceding label, emits no bytes, and
  uses the same assertion vocabulary as `--expect` (registers, flags, memory). It
  is only permitted in a `*_test.asm` file - an assembly error elsewhere - so
  test expectations can never appear in a production build.
- `AssemblyResult.Tests` exposes the collected test specifications.

### Fixed

- `run` and `assert` now resolve `INCLUDE` paths relative to the input file
  (previously only relative to the working directory).
- `run` and `assert` no longer panic when assembly fails during include
  expansion; the error is reported cleanly.

## [0.6.0] - 2026-06-28

### Added

- In-process execution of assembled code via a bundled Z80/Z80N emulator core
  (github.com/ha1tch/zen80), with no separate emulator or file round-trip.
- `zenas run <file>`: assemble and execute, reporting final CPU and memory state.
  Options: `--max-steps=N` (instruction cap / infinite-loop guard, default
  1,000,000), `--call=<label>` (run a subroutine and stop on return),
  `--preload=ADDR,FILE` (load a binary into memory before running; repeatable),
  `--trace` (print each instruction as it runs), `--hex` and `--dump=START:LEN`
  (memory dump), `--json[=basic|standard|detailed|full]`, and `--next` (Z80N).
- `zenas assert <file> --expect "..."`: run, then check final state and exit
  non-zero on failure. Checks cover registers (`A`, `BC`, `HL`, `AF`, the shadow
  pairs `AF_`/`BC_`/`DE_`/`HL_`, `IX`, `IY`, `SP`, `PC`, and the 8-bit
  registers), flags (`CF` `ZF` `SF` `HF` `PF` `NF`), and memory bytes
  (`(0xC000)=0x42`). A run that hits the instruction cap fails as a test.
- Registers start zeroed for reproducible runs.
- `AssemblyResult.Origin` exposes the first ORG (the load address).

### Documentation

- New `docs/RUNTIME.md` guide for the run/assert features, and a manual section.

### Notes

- This adds zenas's first external dependency (github.com/ha1tch/zen80 v0.1.0).

## [0.5.2] - 2026-06-28

### Added

- Pasmo dialect: `#` hexadecimal prefix (`#80` == `0x80`), alongside the existing
  `&`, `0x`, and `h`-suffix forms.
- Bare unquoted `INCLUDE` filenames (`INCLUDE if.asm`), the pasmo style, in
  addition to the quoted form. Works in any dialect.
- Pasmo dialect: `@` is accepted within identifiers.

### Changed

- Pasmo no-colon labels are now recognised at any indentation, not only column 1,
  matching pasmo (the first identifier on a line is a label unless it is a
  mnemonic, directive, or known macro name). This also makes indented pasmo
  source (e.g. tab-indented labels) assemble.

### Notes

- Measured pasmo sample-suite coverage rose from 6/34 to 7/34. The remaining
  failures are dominated by the `PROC`/`LOCAL`/`PUBLIC` scoping subsystem and the
  `?:` ternary operator, which are larger features deliberately not included here.
  See docs/DIALECT_COMPATIBILITY.md.

## [0.5.1] - 2026-06-28

### Added

- Character literals usable in expressions. A single-character literal such as
  `'A'` may now appear as a term in an arithmetic expression (`DEFB 'A'+1`,
  `'Z'-'A'`), evaluating to the character code. This works in both native and
  pasmo dialects and applies to instruction operands and data directives alike.
  Multi-character strings remain string literals.
- Pasmo dialect: column-1 labels without colons. In `.pasmo` mode an identifier
  starting in column 1 is treated as a label even without a trailing colon (e.g.
  `loop    NOP`); a column-1 instruction mnemonic or directive is unaffected,
  matching pasmo. Native mode is unchanged.
- Source dialect modes. A `.pasmo` directive switches the lexer into the pasmo
  dialect for the source that follows, and `.zenas` switches back; the switch is
  scoped and may be repeated. In the pasmo dialect `$` is the location counter
  (current address) - while `$8000` is still a hex literal - and `DEFM` is
  accepted as a string/byte directive. Because includes are spliced as text
  before lexing, wrapping an `INCLUDE` in `.pasmo`/`.zenas` assembles a legacy
  pasmo include in place without converting it. The dialect mechanism is built
  general (a `Dialect` lexer state), so further dialects can be added beside it.
- Tests for the location counter, DEFM, and dialect return-to-native.

### Notes

- The pasmo dialect currently covers the location counter and DEFM; no-colon
  labels, pasmo macro syntax, and PROC/LOCAL scoping are planned.

## [0.5.0] - 2026-06-28

### Added

- Package concept for macros. A `.PACKAGE name` directive groups subsequently
  defined macros, so two libraries may each define a macro of the same name.
  Calls may be qualified (`math.add`); an unqualified call resolves only when one
  package defines the name and is an error (with a "qualify it" message) when
  more than one does. A real instruction is never shadowed by a macro of the same
  name - `ADD` is always the instruction, and a macro `add` is reached as
  `package.add`. This resolves the macro-versus-instruction name collision.
  Internally the macro is keyed by a structured `(package, name)` pair, so the
  package can later become the unit a per-architecture tier is selected in.
- Tests for qualified disambiguation, macro/instruction coexistence, ambiguous
  bare call (error), and unambiguous bare call.

## [0.4.2] - 2026-06-28

### Fixed

- C-style functions can now be written on a single physical line (e.g.
  `void f() { asm { LD A,1; } }`). Previously the C statement terminator `;` was
  lexed as a Z80 comment, swallowing the closing braces and the rest of the line.
  In C-style mode `;` is now a statement terminator inside a brace-delimited block
  and remains a comment at file level, so `;`-style header comments still work.

### Added

- C-style return-width contract. A function's return type fixes its return width:
  `return <expr>;` checks that the value's width matches the declared return
  width and emits `RET`. Returning a value from a `void` function, a bare
  `return;` in a typed function, and falling off the end of a typed function with
  no `return` are all errors - the declared width must be delivered. Value
  placement remains the primitive tier's concern; zenas emits the `RET` and
  enforces the width only.
- Built-in tests for the return contract (width match, width mismatch, value from
  void, missing return, void bare return).

### Added

- A built-in test for a single-line C-style function and call.

## [0.4.1] - 2026-06-28

### Added

- Parameter width markers on macros, shared by traditional and C-style. A
  parameter may carry `uint8_t`/`uint16_t` (aliases `uint8`/`byte`,
  `uint16`/`word`): `MACRO STORE(uint16_t addr, uint8_t value)`. The marker is a
  size-compatibility contract on the signature, not a typed variable.
- Width-signature checking. When a parameter declares a width, a literal
  argument of a different width is an error in either direction - a wider
  argument would be truncated, a narrower one would not fill the promised width.
  Arguments whose width is unknowable (symbols, expressions) are trusted; untyped
  parameters are not checked. This makes the C-style `uint8_t`-style markers
  load-bearing rather than cosmetic.
- Macro tests for width-marker match, width mismatch (error), and untyped pass-through.

## [0.4.0] - 2026-06-28

### Fixed

- Macro bodies can now contain nested macro calls with multiple (or zero)
  arguments. Previously a body line like `INNER(a, b)` failed to parse because
  the body parser treated `(a, b)` as an instruction operand. The body parser is
  now macro-aware: a call to a known macro is parsed as an argument list. This
  also makes C-style functions with more than one parameter work (they transpile
  to nested traditional macro calls), including the previously-failing
  `examples/example14-cstyle-involved.asm`.

### Added

- Macro tests for nested multi-argument and nested zero-argument calls.
- A "Macros" section in `docs/MANUAL.md` covering definitions, calls, nesting,
  local-label uniqueness, the parameter-name/register collision caveat, and the
  scope of the C-style mode.

### Notes

- C-style macros remain structured-assembly sugar over traditional macros, with
  parameters passed by textual substitution. A real calling convention was
  assessed and declined; z88dk is the mature option for compiling C to Z80.

## [0.3.0] - 2026-06-28

### Fixed

- Traditional macros now accept multi-argument and zero-argument calls. Calls
  like `TWO(5, 6)` and `Z()` previously failed because the call site was parsed
  as an instruction operand, where `(a, b)` and `()` are not valid; single-
  argument calls also produced wrong bytes. Traditional macro calls are now
  intercepted at the token level and parsed as proper argument lists.
- Local labels inside a macro body are now unique per expansion. A macro
  containing a loop label, called more than once, previously produced a corrupt
  relative-jump displacement; each expansion's labels now resolve to their own
  addresses. (Two causes: a label pre-definition to 0 that broke jump
  arithmetic, and a unique-label counter that diverged between assembly passes.)
- The C-style macro converter no longer prints debug output. It previously
  emitted hundreds of `[DEBUG]` lines on every C-style assembly.

### Added

- Built-in macro tests (`zenas test`): single-argument, multi-argument,
  zero-argument, and local-label-uniqueness cases.

### Notes

- These fixes cover Phase 1 of the macro work (traditional macros). C-style
  function *parameters* remain unimplemented; C-style support is limited to
  parameterless functions.

## [0.2.2] - 2026-06-28

### Added

- `INCBIN "file"[,skip[,length]]` directive: inserts the raw bytes of a binary
  file at the current address, with optional skip (offset) and length (slice),
  resolved relative to the including file like INCLUDE. Output matches sjasmplus
  byte-for-byte. Out-of-range skip/length and missing files are clear errors.
- `--tag NAME` flag: selects a build tag, the way Go build tags select variants.
  Each tag defines `ZENAS_TAG_NAME` (presence) and `ZENAS_TAGBIT_NAME` (its bit,
  assigned in command-line order), and contributes to the composite bitmask
  `ZENAS_TAGS`. `ZENAS_TAGS` is always defined (0 with no tags), so source
  referencing it compiles in every configuration. Up to 16 distinct tags occupy
  bits. Accepts `--tag NAME` and `--tag=NAME`; tags compose.
- `IF` conditions now support the boolean operators `AND`, `OR`, `NOT` and
  parentheses (with short-circuit evaluation), scoped to `IF` so they do not
  affect operand or `DB` expressions. Existing `IF`/`IFDEF`/`IFNDEF` forms are
  unchanged.
- `examples/example-incbin-tags.asm` demonstrating INCBIN and composable tags.

### Changed

- `zenas help` is now concise (core usage and common options); the full option,
  charset and output reference moved to `zenas help --all`. Removed dead help
  code and corrected stale usage strings.
- Added [`docs/MANUAL.md`](docs/MANUAL.md), a reference for the command line,
  source language, directives, conditionals, build tags, INCLUDE/INCBIN, Z80N,
  and character sets.

## [0.2.1] - 2026-06-28

### Changed

- Added `tools/sjasmplus_compare.sh` and a `make test-sjasmplus` target: an
  optional cross-assembler check that builds sjasmplus from source and verifies
  every documented, half-register, and Z80N form byte-for-byte against it.
- Documentation refreshed to the current coverage: 468 documented Z80 + 48
  undocumented IX/IY half-register + 29 Z80N instructions (`docs/INSTRUCTION_SET.md`,
  README). The stale "core subset / in progress" status was corrected.
- The Z80N coverage checker (`tools/check_z80n_coverage.sh`) now uses **sjasmplus**
  as a live oracle when it is installed, assembling each instruction under
  `DEVICE ZXSPECTRUMNEXT` and comparing byte-for-byte against `zenas --next`;
  it falls back to the baked-in reference bytes otherwise. All 31 Z80N forms match
  sjasmplus, as do all 516 base-Z80 forms - so zenas now agrees with two
  independent reference assemblers (pasmo and sjasmplus).

## [0.2.0] - 2026-06-28

### Added

- Z80N (ZX Spectrum Next) instruction set, off by default and enabled with the
  `--next` flag or its alias `--cpu=Z80N` (the default is `--cpu=Z80`). Covers all
  31 Next-specific forms - SWAPNIB, MIRROR, TEST, the barrel-shift group
  (BSLA/BSRA/BSRL/BSRF/BRLC), MUL, ADD rr,A / ADD rr,nn, PUSH nn (big-endian
  operand), OUTINB, NEXTREG (both forms), PIXELDN/PIXELAD/SETAE, JP (C), and the
  DMA block-copy group (LDIX/LDWS/LDDX/LDIRX/LDPIRX/LDDRX). Opcodes are taken from
  the official SpecNext reference; see docs/Z80N_REFERENCE.md. A dedicated
  coverage checker (tools/check_z80n_coverage.sh) verifies all encodings.

### Changed

- When several instruction encodings match the same operand types (e.g. JP (HL)
  and the Z80N JP (C)), the encoder now tries each in turn and only reports an
  error if none apply, rather than failing on the first.

## [0.1.2] - 2026-06-28

### Fixed

- Illegal undocumented half-register combinations are now rejected instead of
  silently emitting a different instruction: mixing an IX half with an IY half
  (`LD IXH,IYL`) and combining a half register with an indexed memory operand
  (`LD (IX+d),IXH`). All legal `IXH`/`IXL`/`IYH`/`IYL` forms continue to encode
  correctly. The instruction-coverage check now also exercises the half-register
  set (516 forms, all matching the reference assembler).

## [0.1.1] - 2026-06-28

### Added

- Conditional `CALL cc,nn` (all eight conditions), `JP (HL)` / `JP (IX)` /
  `JP (IY)`, `LD (IX+d),n` / `LD (IY+d),n`, the interrupt/refresh register loads
  `LD A,I` / `LD A,R` / `LD I,A` / `LD R,A`, and the block-I/O group
  `INI/IND/INIR/INDR/OUTI/OUTD/OTIR/OTDR`.
- IX/IY index displacements now accept symbol expressions (`LD A,(IX+OFFSET)`,
  `(IX+N*2)`), evaluated at assembly time, and are range-checked to a signed byte.
- `docs/INSTRUCTION_SET.md` coverage table and a reproducible checker
  (`tools/check_instr_coverage.sh` + `tools/gen_instr_coverage.py`) that assembles
  a representative of every Z80 instruction family and diffs the bytes against a
  reference assembler. The full set (468 forms) matches byte-for-byte.

### Fixed

- `INC IX` / `DEC IX` / `INC IY` / `DEC IY` were emitted without the DD/FD prefix
  (encoding as `INC HL` / `DEC HL`), producing wrong machine code. Now prefixed.
- An out-of-range IX/IY displacement is now a clear error instead of silently
  wrapping to a signed byte.
- `LD A,I` / `LD A,R` / `LD I,A` / `LD R,A` previously mis-encoded as ordinary
  register transfers; they now emit the correct ED-prefixed opcodes.

## [0.1.0] - 2026-06-28

### Added

- Undocumented half-index registers IXH/IXL/IYH/IYL (e.g. `LD IXH,A`, `DEC IXH`),
  encoded as the H/L-form instruction with a DD/FD prefix.
- Accumulator rotates `RLCA`, `RRCA`, `RLA`, `RRA`.
- `DS` / `DEFS` (define-space) recognised as directives.
- Character literals in value contexts (`FONT_FIRST EQU '!'`) evaluate to the
  charset-mapped character code.
- Unary `+`/`-` in operand and directive expressions.

### Fixed

- **Relative jumps (`JR`, `JR cc`, `DJNZ`) computed a wrong displacement** - the
  target was left as an absolute address and truncated to one byte, so e.g.
  `JR NZ,loop` emitted `20 00` instead of the correct signed offset. The
  displacement is now `target - (address + 2)`, range-checked to -128..127.
- **`ORG` with a symbol argument** (`ORG KERNEL_ORG`) placed all code at a
  zero-based origin, because pass 1 returned a dummy 0 for every symbol including
  already-defined ones. Backward references now resolve to their real value in
  pass 1, so `ORG` and similar address math are correct.
- A bare `;` line (and `;` not followed by space/letter) is now a comment.
  Previously it produced a stray semicolon token and a parse error.

### Milestone

- zenas assembles a full real-world operating-system kernel byte-identically
  to pasmo (6530 bytes, all 400 symbols matching). This was the readiness
  acceptance test.

## [0.0.7] - 2026-06-28

### Added

- IX/IY indexed addressing. All forms used by real Z80 code, verified
  byte-identical to pasmo: `LD r,(IX+d)` / `LD (IX+d),r`, `LD IX,nn`, `ADD IX,rr`,
  `PUSH`/`POP IX`, `LD SP,IX`, and ALU/INC/DEC on indexed operands (`CP (IX+d)`,
  `INC (IX+d)`). IY variants use the FD prefix. Displacements accept constant
  expressions (`(IX+1+3)`).
- ALU and INC/DEC operations on `(HL)` (`ADD A,(HL)`, `AND (HL)`, `CP (HL)`,
  `OR (HL)`, `SUB (HL)`, `INC (HL)`, `DEC (HL)`, etc.), which were previously
  unimplemented and are the base form the indexed variants build on.

### Fixed

- `LD IX,nn` / `LD IY,nn` previously encoded silently as `LD HL,nn` (missing the
  DD/FD prefix), producing wrong machine code. They now emit the correct prefixed
  encoding.

## [0.0.6] - 2026-06-28

### Added

- Operand expressions: instruction operands now accept symbol arithmetic with the
  full `+ - * /` grammar and correct precedence, in three positions: immediate
  operands (`LD HL,kernel_end+255`, `ADD A,ATOM_HDR+7`, `LD HL,base+N*3`), indirect
  operands (`LD (rowcount+1),A`, `LD HL,(base+2)`), and IX/IY displacements
  (`LD D,(IX+1+3)`, folded to a constant at parse time). 8-bit immediate operands
  remain value-checked, so a symbol+offset that exceeds one byte is still rejected
  for an 8-bit slot. Directive arguments (`DW base+10`) already supported this.

## [0.0.5] - 2026-06-28

### Added

- Conditional assembly: `IF`, `IFDEF`, `IFNDEF`, `ELSE`, `ENDIF`, implemented with
  a per-pass condition stack (nesting supported). `IF` takes an expression (true
  when non-zero); `IFDEF`/`IFNDEF` test whether a symbol is defined. A symbol that
  is undefined in an `IF` expression evaluates to 0 (false) consistently in both
  passes, so branch selection never diverges between passes. An unterminated `IF`
  or a stray `ELSE`/`ENDIF` is reported as an error.
- `--define=NAME[=VALUE]` pre-defines a symbol before assembly (default value 1;
  VALUE accepts decimal, `0x` hex, or `$` hex). Combined with `IF`/`IFDEF`, this
  gives Go-build-tag-style selection driven from the command line - e.g. the
  multi-target build can choose a layout with `--define=TARGET_48K`.

## [0.0.4] - 2026-06-28

### Changed

- zenas is now case-sensitive for user symbols (labels and constants) and remains
  case-insensitive for instruction mnemonics, registers, condition codes, and
  directives - matching pasmo and the wider Z80 toolchain. Previously every
  identifier was folded to uppercase, which collapsed distinct mixed-case symbols
  (e.g. a runtime byte `line_h` and a compile-time constant `LINE_H`) into one and
  could silently miscompile. The symbol file now preserves original case.

## [0.0.3] - 2026-06-28

### Added

- Symbol-file output: `--sym` writes a pasmo-format `.sym` file (lines of the form
  `NAME EQU 0XXXXH`, sorted by name) alongside the binary; `--sym=path`
  sets an explicit path. Without the flag, no symbol file is written. This is the
  format the downstream memory-map and release tooling parses.


### Changed

- Help output now leads with the version (`zenas <version> - Z80 Assembler ...`),
  and lists the `version` command. The `version` command and its `-v` / `--version`
  forms report the version string.

## [0.0.2] - 2026-06-28

### Added

- Instruction encoder coverage extended to close the gaps real-world source
  exercises. New / fixed forms, all verified byte-correct:
  - Block transfer/search: `LDIR`, `LDDR`, `LDI`, `LDD`, `CPIR`, `CPDR`, `CPI`,
    `CPD`.
  - Port I/O register form: `IN r,(C)`, `OUT (C),r` (the parser now recognises
    `(C)` as a port operand rather than an undefined symbol).
  - Exchange: `EX DE,HL`, `EX AF,AF'`, `EX (SP),HL`, `EXX`.
  - 16-bit arithmetic: `ADC HL,rr`, `SBC HL,rr`.
  - CB-prefixed `(HL)` forms: `BIT/RES/SET n,(HL)` and `RLC/RRC/RL/RR/SLA/SRA/SRL
    (HL)`.
  - Misc: `NEG`, `RETI`, `RETN`, `RRD`, `RLD`, `IM 0/1/2`, `RST n`.

### Fixed

- Lexer now accepts a trailing apostrophe on prime (shadow) register names
  (`AF'`), so `EX AF,AF'` lexes correctly. Ordinary character literals such as
  `'A'` are unaffected (the rule is restricted to register identifiers).

## [0.0.1] - 2026-06-28

First standalone release of zenas as its own repository, extracted from the zen80
project where the assembler was originally developed. The version line starts
fresh at 0.0.1 for the standalone project.

### Added

- Standalone Go module `github.com/ha1tch/zenas` (previously a subpackage of
  `github.com/ha1tch/zen80`).
- Project scaffolding in the house style: `VERSION`, `pkg/version/version.go`,
  `syncver.sh`, `Makefile`, and `release.sh`.
- `zenas version` / `--version` command, wired to the version package.
- `INCLUDE` directive (pasmo-style textual inclusion). Included files are spliced
  at the directive site before lexing, so forward references across the include
  boundary resolve. Paths resolve relative to the file containing the INCLUDE;
  nested includes resolve relative to their own file. Include cycles are detected
  and reported. Both bare `INCLUDE` and dotted `.INCLUDE` are accepted, with
  optional trailing comments. `AssembleFile` is now implemented and sets the
  include base directory automatically.

### Fixed

- Bare (non-dotted) `NAME EQU value` now parses and binds correctly. Two fixes:
  the parser now recognises a label followed by a bare directive identifier (not
  only the dotted `.EQU` form), and the assembler binds the EQU value to the label
  for both `EQU` and `.EQU`. Previously `NAME EQU value` either failed to parse or
  bound the label to the location counter instead of the constant.

### Changed

- Module import path updated from `github.com/ha1tch/zen80/assembler` to
  `github.com/ha1tch/zenas/assembler`.
- Historical material (older source snapshots, the legacy macro parser) moved to
  `attic/`.

### Notes

- The assembler is functional for a core Z80 subset: two-pass assembly, forward
  references, a working traditional macro system, multiple number formats
  (`0x`, `%`, decimal), string and multi-value `DB`/`DW`, bare and dotted
  directives, `INCLUDE`, and a symbol table.
- Known gaps are tracked in the project issue list.
