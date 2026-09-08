#!/usr/bin/env bash
# gen_golden_vectors.sh - Export mnemonic,bytes golden test vectors as
# plain data, for a downstream fork to verify its own encoder against
# without hand-transcribing bytes from release notes.
#
# Source of truth: zenas's own assembled output for every line in
# instr_coverage_list.txt (the documented instruction set, already
# cross-checked against pasmo as part of this project's own release
# process -- see check_instr_coverage.sh) plus a curated list of the
# undocumented instructions zenas also supports, which pasmo itself
# doesn't recognise so can't cross-check.
#
# Output format: one instruction per line, "SOURCE|HEXBYTES", e.g.
#   LD A,B|78
#   RLC (IX+1), B|DDCB0100
#
# Usage: ZENAS=./build/zenas tools/gen_golden_vectors.sh > golden_vectors.txt
set -u
ZENAS="${ZENAS:-./build/zenas}"
DIR="$(cd "$(dirname "$0")" && pwd)"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

emit() {
	local ins="$1"
	printf '        ORG $8000\n        %s\n' "$ins" >"$tmp/c.asm"
	if ! "$ZENAS" assemble "$tmp/c.asm" "$tmp/z.bin" >/dev/null 2>&1; then
		echo "SKIP (assembly failed): $ins" >&2
		return
	fi
	local bytes
	bytes=$(od -An -tx1 "$tmp/z.bin" | tr -d ' \n' | tr 'a-f' 'A-F')
	echo "${ins}|${bytes}"
}

# Documented instruction set: every line in instr_coverage_list.txt.
while IFS= read -r ins; do
	[ -z "$ins" ] && continue
	emit "$ins"
done <"$DIR/instr_coverage_list.txt"

# Undocumented instructions: pasmo doesn't recognise these, so they
# can't be cross-checked the way the documented set is -- verified
# instead against multiple independent Z80 documentation sources
# during development (see CHANGELOG.md's [0.9.0] entry).
for ins in \
	"RLC (IX+1), B" \
	"SET 7, (IX+2), A" \
	"RES 0, (IY-5), H" \
	"SRL (IX+0), C" \
	"SLL (IY+10), L" \
	"BIT 7, (IY+0)" \
	"RES 3, (IX+10)" \
	"SET 0, (IY-5)" \
	"RLC (IX+5)" \
	"RRC (IY-3)" \
	"SLL B" \
	"SLI (HL)" \
	"SL1 (IX+5)" \
	"IN (C)" \
	"OUT (C), 0"; do
	emit "$ins"
done
