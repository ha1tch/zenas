#!/usr/bin/env bash
# Assemble every instruction in instr_coverage_list.txt with zenas and a reference
# assembler (pasmo) and compare bytes. Exits non-zero if any form is missing or
# mismatched. Usage: ZENAS=./build/zenas REF=pasmo tools/check_instr_coverage.sh
set -u
ZENAS="${ZENAS:-./build/zenas}"
REF="${REF:-pasmo}"
DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0; MISSING=0; MISMATCH=0; tmp=$(mktemp -d)
while IFS= read -r ins; do
  [ -z "$ins" ] && continue
  printf '        ORG $8000\n        %s\n' "$ins" > "$tmp/c.asm"
  "$REF" --bin "$tmp/c.asm" "$tmp/p.bin" >/dev/null 2>&1 || continue
  p=$(od -An -tx1 "$tmp/p.bin" | tr -s ' ' | sed 's/^ //;s/ $//')
  if ! "$ZENAS" assemble "$tmp/c.asm" "$tmp/z.bin" >/dev/null 2>&1; then
    MISSING=$((MISSING+1)); echo "MISSING  $ins  (ref: $p)"; continue
  fi
  z=$(od -An -tx1 "$tmp/z.bin" | tr -s ' ' | sed 's/^ //;s/ $//')
  if [ "$z" = "$p" ]; then PASS=$((PASS+1)); else MISMATCH=$((MISMATCH+1)); echo "MISMATCH $ins  zenas:$z ref:$p"; fi
done < "$DIR/instr_coverage_list.txt"
rm -rf "$tmp"
echo "PASS=$PASS MISSING=$MISSING MISMATCH=$MISMATCH"
[ "$MISSING" -eq 0 ] && [ "$MISMATCH" -eq 0 ]
