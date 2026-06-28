# Closing the "deferred by design" items

Three things were deferred deliberately during the macro and width-marker work,
not skipped by oversight. This is an assessment of what each would actually take,
grounded in the current code, and in what order they make sense.

The three are: return-width checking, behavioural contracts between primitive
tiers, and a second backend. They are not independent - the order matters,
because two of them only become well-defined once earlier ones exist.

## 1. Return-width checking - DONE

**Resolved.** This was deferred on the assumption that checking return width
required a return *contract* (a defined location a value lands in), which is the
front edge of a calling convention - the thing deliberately declined. That
assumption was wrong in a useful way: the return *width* is the portable half of
a return contract, exactly as a parameter width marker is the portable half of an
argument contract. The width can be fixed by the signature and checked without
deciding where the value lives.

The rule implemented: a function's return type is its return-width contract.

- `return <expr>;` checks that the value's width matches the declared return
  width (literal widths checked; symbols/expressions trusted), and emits `RET`.
- `return <expr>;` in a `void` function is an error.
- A bare `return;` is valid only in `void`; in a typed function it is an error.
- Falling off the end of a typed function (no `return` at all) is an error - the
  same failure as a bare return in a typed function: the declared width goes
  undelivered.

What it deliberately does **not** do: place the returned value in a result
location. `return` emits the control-flow `RET` and enforces the width contract;
where the 8- or 16-bit value lives is the primitive tier's concern. This keeps
the feature on the portable side of the line and out of calling-convention
territory, consistent with the rest of the design.

The full calling convention (register/stack argument and result placement)
remains out of scope, by the same reasoning as before.

## 2. Behavioural contracts between primitive tiers

**Why it matters.** This is the real one. The portability model is: application
logic is written against a vocabulary of primitive macros (`rotate`, `shift`,
`negate`, ...), and each target architecture supplies its own implementation of
that vocabulary. Portability holds *only if every implementation of a primitive
behaves identically* - same effect, same flags touched, same registers
preserved or clobbered. Width markers check that arguments are the right size.
They do **not** check that `rotate` on the Z80 tier and `rotate` on a future
6502 tier agree about, say, whether the carry flag survives. A `--tag`-selected
matrix build is exactly the situation where such a divergence would compile
cleanly on both targets and produce a silent cross-target bug.

**What exists today.** Nothing. There is no notion of a behavioural contract
anywhere in the code - no clobber lists, no flag-effect annotations, no
preservation declarations. The width marker is the only boundary contract.

**What it would take.** A primitive would need to *declare* its behavioural
contract, and the assembler (or a checker) would need to verify each
implementation against the declaration. The lightweight, in-scope version is
declaration plus cross-implementation consistency, not full verification:

- A primitive declares what it touches: e.g. `rotate` clobbers `A`, preserves
  `BC/DE/HL`, defines carry. This is a small annotation vocabulary, not a
  semantics engine.
- When the same primitive name is defined for multiple tiers (selected by tag),
  the assembler checks the declarations *match* across tiers. It cannot prove the
  asm honours the declaration without a model of the machine, but it can ensure
  the contracts are at least stated identically - which catches the most likely
  error: two tiers drifting apart in what they promise.
- The harder version - verifying the actual instructions honour the declared
  clobber/preserve set - needs per-architecture machine models (which
  instructions touch which flags/registers). That is real work and is its own
  project, but it is the same machine knowledge a second backend needs anyway, so
  it is not wasted.

**Recommendation.** This is the highest-value deferred item, because it is the
thing that makes the portability claim *trustworthy* rather than aspirational.
But it is only well-defined once there is a primitive vocabulary to attach
contracts to. So it follows the vocabulary, it does not precede it. The first
concrete step is small and worth taking early: define the contract-annotation
vocabulary (clobbers / preserves / defines-flag) as declarations, even before any
checking, so primitives are written with their contracts stated from day one.
Checking can come later; the discipline of stating the contract is most of the
value.

## 3. A second backend

**Why it matters.** It is the only real test of the entire portability thesis.
Everything reads as portable today partly because nothing has had to port. A
second target - even a partial one - is where leaky abstractions reveal
themselves.

**What exists today.** zenas is Z80/Z80N throughout. `encoder.go` (~1,190 lines)
is Z80-specific; Z80N is layered on as an extension of the same encoder, not as a
separate target. There is no architecture abstraction: no notion of a target, no
pluggable encoder, no per-architecture instruction table boundary.

**What it would take.** Less than it first appears, *because of how the layers are
arranged*. The portability story does not require zenas itself to assemble two
architectures. It requires:

- the application tier (logic over primitives) to be architecture-neutral - which
  it already is, by construction;
- a per-architecture primitive tier - hand-written asm, selected by `--tag`;
- an assembler for each target.

So a second backend does **not** mean making zenas multi-architecture. It means
having an assembler for the second architecture and a second primitive tier, with
`--tag` selecting between them. zenas could remain the Z80 assembler; a sibling
tool (or a future zenas with a target abstraction) handles the other. The
`--tag` mechanism that selects the tier is already built. This is the payoff of
the two-tier design: the second target is a primitive file plus an assembler for
that target, not a rewrite of this one.

The cheapest possible proof is narrow: take two or three primitives (a `rotate`,
a counted `loop`, a `call`), implement them for one other architecture, and show
the *same* application source producing correct output for both via tags. That
tests the thesis end-to-end without committing to a full second instruction set.

**Recommendation.** This is the eventual decisive test, but it is correctly last.
It depends on the primitive vocabulary existing (item 2's subject) and is most
informative once a handful of primitives have real contracts to honour. Until
then it would be testing an abstraction that has not been built yet.

## Order

The three are a dependency chain, not a menu:

1. **Define the primitive vocabulary** (not itself a deferred item - it is the
   next real design work). Everything else attaches to it.
2. **State behavioural contracts** as declarations on those primitives (item 2,
   first half). Small, high-value, do it as the vocabulary is written.
3. **Check contracts across tiers** (item 2, second half) once more than one tier
   exists.
4. **Second backend** (item 3) as the end-to-end proof, which also supplies the
   machine model that full contract-checking would later use.

Return-width (item 1) is done: the return type is a width contract, checked at
the return site, emitting `RET` while leaving value placement to the primitive
tier. The full calling convention remains deliberately out of scope.

The through-line: none of the remaining items are blocked by missing assembler
machinery. They are blocked by the primitive vocabulary not existing yet. That is
the real next piece of work, and it is a design task, not a bug-fix - which is
consistent with where the whole effort has been heading.
