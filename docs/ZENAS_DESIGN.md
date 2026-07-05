# The logic of zenas

## Why this document exists

If you read the zenas manual as a catalogue of features, some entries look
eccentric. An assembler with a C-flavoured macro syntax that has no `if`, no
loops, and no expressions. A namespacing system for macros. A typed signature
check that verifies only the *width* of a return value. A test harness with its
own file-naming discipline. Each is usable on its own, and the
[programming guide](ZENAS_PROGRAMMING.md) explains each on its own terms — but
they were not designed on their own terms. They are parts of a single idea, and
zenas makes considerably more sense once you can see it.

This document explains that idea. It is not required reading: zenas works
perfectly well as a plain Z80/Z80N assembler, and if that is all you want from
it, nothing here changes how you use it. But if you have ever wondered *why*
the C-style layer refuses to grow control flow, or what packages are really
for, this is the answer.

## The problem: portable logic without another language

Suppose you have a body of application logic — game rules, a data structure,
a protocol — implemented over Z80 assembly, and you would like that logic to
be *portable*: movable to another machine, or another memory map, or
eventually another CPU, without rewriting it.

The conventional answer is to adopt a higher-level language. Write the logic
in C and compile it with SDCC or z88dk; keep assembly for the hot paths. This
works, but look at what it costs. You have imported a compiler, a runtime, a
calling convention, a linker model, and a build system — a massive dependency,
adopted in its entirety, to obtain portability you only needed at one level.
Your *computation* was never the problem; Z80 code computes fine. What you
wanted to make portable was the *composition* — the layer where routines are
named, combined, and called. The language brought you a portable everything
when you needed a portable seam.

There is a well-known family of alternative answers. RPython (the language
PyPy is written in), Red/System (Red's system dialect), PreScheme and others
all take a rich language and carve out a restricted subset with semantics
simple enough to translate to low-level code. This works too, but it works
*top-down*: you still carry the host language as a dependency, and the
subset's definition tends to live in the translator rather than in any
specification.

zenas takes the same problem *bottom-up*. Instead of asking "how little of a
big language do we need?", it asks: **what is the minimal typed interface an
assembler must natively provide so that logic written against it is
retargetable — with no upper language existing at all?** If that interface is
small enough to build into the assembler, then the restricted subset becomes
unnecessary, because the subset's only job was ever to reach an interface like
this one. You can think of it as an intermediate representation that is built
in rather than compiled down to: a seam, native to the assembler, that a
systems language *could* target but that you can also write against directly.

Z80 is the first substrate for this experiment deliberately: small enough that
results are visible early, and awkward enough — few registers, a
non-orthogonal instruction set — that a convention which survives it has been
genuinely stressed rather than merely demonstrated.

## The seam: what the C-style layer is, and is not

The interface — the seam — is what `.MACRO_STYLE C` gives you. It consists of
exactly this much:

- **Typed signatures.** Functions declare `uint8_t`, `uint16_t`, or `void`,
  and zenas checks the return *width*: an 8-bit function must return an 8-bit
  value, a `void` function must return nothing, a non-void function that never
  returns is an error.
- **Calls**, including qualified calls into packages: `math.add(20, 22)`.
- **Variables and assignment**: declare `uint8_t total;` and assign it a
  literal or the result of a call.

And that is all. There are no expressions, no conditionals, no loops. This is
not a roadmap with gaps in it; the omissions are the design. The C-style layer
is a **linkage language**, not a programming language. Computation belongs
inside routines — where you write assembly, as you always did. The seam exists
only to *compose* routines: to name them, type their boundaries, and connect
them. Everything below a call is target-specific by nature and stays in
assembly; everything at the call level is target-independent by construction
and is exactly the layer worth keeping portable.

This also explains the "half calling convention". Width checking is the
portable half of the contract: it is what the interface can promise on any
target. *Where* a value lives — which register, which memory slot — is the
other half, and it is deliberately left to the implementation on each side of
the seam, because that is precisely the part that must be free to differ
between targets.

So when the layer declines to grow an `if` statement, it is not being timid.
A construct added at the seam is a construct every future target must honour;
a construct kept inside a routine costs nothing. The pressure to add control
flow to C-syntax code is real — the syntax invites it — but yielding to it is
how a linkage language decays into a poor compiler. The subset-language
projects mentioned above tended to fail by *drift*: the subset grew until it
was a dialect. The mirror risk here is *seam accretion*, and the defence is
the same in both cases: hold the line.

## How the rest of zenas serves the seam

Once you see the seam, the other features stop being a miscellany.

**Packages are swappable implementations.** `.PACKAGE` is not namespacing for
tidiness. A package is the unit of *retargeting*: a set of routines
implementing an interface for one target. Application logic calls `math.add`
and `screen.plot`; which `math` and which `screen` are linked in is a build
decision, not a source change. Two packages may implement the same signatures
for different machines, and the qualified-call syntax means neither collides
with the other or with instruction mnemonics.

**Build tags select the variant.** `--tag plus3 --tag debug` is how a build
names its target configuration. Tags compose as a bitmask and are testable in
`IF` conditions, so the choice of primitive package — and any conditional
scaffolding around it — is driven by a single, structured notion of "which
variant is this" rather than a loose bag of defines.

**The test harness is the measuring instrument.** `zenas run` and
`zenas assert` are pleasant conveniences for any assembly programmer, but for
seam-structured programs they become something stronger. Write your
application-level tests in a `*_test.asm` file with `.EXPECT` assertions; run
that *same suite* against the program linked with primitive package A and
again with primitive package B. If both pass, you have demonstrated
observational equivalence of two implementations of the interface — which is
the property that makes an interface trustworthy. The retargeting story and
the testing story are not two features; together they are one experimental
apparatus.

**Scoped dialects are the on-ramp.** `.pasmo`/`.zenas` let you ingest existing
source mid-stream without converting it. You do not have to believe in any of
the above to start using zenas on a codebase you already have; the seam is
there when you want it.

## Practical consequences for structuring a program

If you want to write in the seam-structured style, three guidelines follow
from the design.

**Separate the layers.** Keep application logic in C-style functions that
contain only declarations, assignments, and calls. Keep `asm { }` bodies — and
traditional macros with raw assembly — inside your primitive packages. Today
this separation is a discipline rather than something zenas enforces; the
portability of your application layer is exactly as good as your adherence to
it.

**Assert on memory, not registers, at the application level.** `.EXPECT`
can check registers and flags (`A=15, CF=0`), and for testing a primitive
routine that is the right tool. But registers and flags are target vocabulary.
Application-level tests should assert on symbol-resolved memory —
`.EXPECT (total)=42` — because those assertions state the contract in terms
that survive retargeting. Register assertions belong in the primitive
package's own tests.

**Make primitives coarse.** Crossing the seam has a cost: arguments move
through the calling convention, and under `.MACRO_MODE SINGLETON` through
fixed memory slots. A primitive that does one instruction's worth of work
pays the toll without amortising it. Design primitive routines to do a
meaningful unit of work per call — the interface is a boundary, and good
boundaries are crossed infrequently. (Note also SINGLETON's documented
constraint: fixed parameter slots are not re-entrant, so a singleton macro
must not be reachable from an interrupt handler that might interrupt itself.)

## What to expect from zenas development

Two commitments follow from all of this, and it is fairer to state them than
to leave you to infer them from declined feature requests.

First, **the seam will stay small**. Proposals to add expressions, arithmetic,
or control flow to the C-style layer will be evaluated as what they are:
changes to the portable interface that every target must then support. The
default answer is that the construct belongs inside a routine, not at the
seam. If genuine use ever demonstrates that the seam is missing something a
linkage language needs, that is a finding worth acting on — but the burden of
proof sits with the addition, not with the refusal.

Second, **this is a research direction, honestly labelled**. The hypothesis is
that width-typed calls, assignment, and packaged names constitute a *stable*
portable interface — that real programs can be written against it without
constant pressure to widen it, and that a program's application layer really
can move between targets by swapping primitive packages alone. Z80 across
different machines is the first test; harder tests come later. The hypothesis
may yet be falsified by experience, and if it is, the honest result will be
documented rather than papered over. In the meantime, everything the
experiment needs — the seam, the packages, the tags, the harness — is also,
feature by feature, just a useful assembler. That is not an accident either.
