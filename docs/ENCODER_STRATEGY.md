# ENCODER_STRATEGY.md - growing the encoder from zen80's decode algebra

This note records a strategy for closing zenas's instruction-encoding gaps
systematically, by adopting the same bit-field decomposition that the zen80
emulator uses to *decode* opcodes, run in reverse to *encode* them.

## The current encoder

zenas's encoder (`assembler/encoder.go`) is a template table: each instruction
form is registered with `addInstruction(mnemonic, operandTypes, baseOpcode,
prefix, cycles, encodeFunc)`, pointing at a hand-written `encodeFunc`
(`encodeLDrr`, `encodeALUr`, `encodeADDHLrp`, ...). There are on the order of 66
such registrations. Adding a missing instruction means writing another function
and another row. This is correct but does not scale: every gap is manual work, and
the gaps (EX, EXX, 16-bit ADC/SBC, `(C)` ports, `(HL)` bit ops) are exactly the
forms not yet hand-coded.

## What zen80 provides

zen80's `z80/decode.go` does not use an opcode table. It decomposes each opcode
into bit-fields and dispatches on them:

```
x := opcode >> 6        // bits 7-6   (block: 0..3)
y := (opcode >> 3) & 7  // bits 5-3
z := opcode & 7         // bits 2-0
p := y >> 1             // bits 5-4
q := y & 1              // bit 3
```

then routes through `executeBlock0..3` (and the CB/ED/DDFD prefix files) using
`x`, `y`, `z`, `p`, `q`. This is the standard Z80 "algebraic" opcode structure.

The significance for zenas: this decomposition **is** the encoding rule, read
backwards. Decode extracts `y` and `z` *out of* an opcode; encode computes the
opcode *from* operand register-numbers via the same fields. zenas already holds
the register-number maps (`register8Map`, `register16Map`, `conditionMap`) that
are the inverse of zen80's `getRegister8` / register helpers.

## The strategy: encode by block, not by instruction

Replace families of hand-written functions with a few block-level encoders that
compute the opcode arithmetically. The opcode forms below follow the standard Z80
encoding and were cross-checked against zen80's decode source.

- **Block 1 - `LD r,r'`**: `opcode = 0x40 | (dst << 3) | src`, where `dst`/`src`
  are 8-bit register numbers (B=0, C=1, D=2, E=3, H=4, L=5, (HL)=6, A=7). The
  single exception `0x76` is `HALT` (the `(HL),(HL)` slot). One encoder covers all
  49 register-to-register loads plus the `(HL)` source/dest forms.

- **Block 2 - ALU on register**: `opcode = 0x80 | (op << 3) | src`, where `op`
  selects ADD/ADC/SUB/SBC/AND/XOR/OR/CP (0..7). One encoder covers the whole ALU
  register family; the immediate forms are `0xC6 | (op << 3)` followed by the
  byte.

- **Rotates/bit ops (CB prefix)**: `0xCB` then `(group | (y << 3) | reg)` -
  RLC/RRC/RL/RR/SLA/SRA/SLL/SRL by `y`; `BIT/RES/SET n,r` as
  `0x40/0x80/0xC0 | (bit << 3) | reg`. This closes `RES n,(HL)` and the other
  `(HL)` bit-op forms (reg = 6) that the current encoder rejects.

- **16-bit ADC/SBC (ED prefix)**: `0xED` then `0x42 | (p << 4) | (q << 3)`, with
  `q=0` for SBC and `q=1` for ADC, `p` selecting BC/DE/HL/SP. zen80 decodes this at
  `prefix_ed.go` "SBC/ADC HL,rp". One encoder covers all eight.

- **`IN r,(C)` / `OUT (C),r` (ED prefix)**: `0xED` then `0x40 | (y << 3) | z`
  family (`z=0` IN, `z=1` OUT). Closes the port-`(C)` forms zenas currently parses
  as an undefined symbol `C`.

- **Fixed single-opcode instructions**: `EX AF,AF'` = `0x08` (block 0, z=0, y=1);
  `EXX` = `0xD9`; `EX DE,HL` = `0xEB`; `EX (SP),HL` = `0xE3`. These are not a
  family - just individual opcodes to register. zen80 decodes them in block 0 and
  block 3.

## Scope and limits

This strategy addresses the **encoder only**. It does not help the parser, where
zenas's most load-bearing gaps for real programs live: `INCLUDE`, bare
(non-dotted) directives, operand expressions (`label + N*3`), and conditional
assembly. Emulators do not parse assembly text, so zen80 contributes nothing
there. The parser work is independent and should be sequenced alongside.

Two encoder areas need care beyond the clean block algebra:

- **IX/IY via DD/FD prefixes**: add a prefix byte and, for indexed forms, a
  displacement byte; the `DDCB`/`FDCB` double-prefix puts the displacement before
  the opcode. zen80 handles these in `prefix_ddfd.go` and is worth mirroring
  closely rather than deriving fresh.
- **Undocumented opcodes**: not required by the current target programs and out of
  scope for now.

## Why this is worth doing

zen80 is validated against the zexdoc test suite, so encoding via the same
bit-field decomposition yields bytes that match what zen80 decodes - which is the
byte-identical property the readiness checklist requires. Converting the encoder
to block-level arithmetic closes the listed instruction gaps in a small number of
structured functions rather than a dozen-plus hand-written ones, and keeps zenas
and zen80 in algebraic lockstep.

See [`ZENAS_READINESS.md`](ZENAS_READINESS.md) for the full gap inventory and the
acceptance criteria.
