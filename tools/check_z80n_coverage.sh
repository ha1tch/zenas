#!/usr/bin/env bash
# Verify Z80N instruction encodings. If sjasmplus is available it is used as the
# live oracle (the de facto ZX Spectrum Next assembler, and the reference
# implementation of Z80N encoding); each instruction is assembled under
# DEVICE ZXSPECTRUMNEXT and compared to zenas --next. Otherwise the expected bytes
# baked into z80n_coverage.txt (from the official SpecNext reference) are used.
# Usage: ZENAS=./build/zenas [SJASMPLUS=sjasmplus] tools/check_z80n_coverage.sh
set -u
ZENAS="${ZENAS:-./build/zenas}"
SJASMPLUS="${SJASMPLUS:-sjasmplus}"
DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0; FAIL=0; tmp=$(mktemp -d)
have_sj=0
command -v "$SJASMPLUS" >/dev/null 2>&1 && have_sj=1
if [ "$have_sj" -eq 1 ]; then
  echo "oracle: sjasmplus (live)"
else
  echo "oracle: baked-in expected bytes (sjasmplus not found)"
fi
while IFS='|' read -r ins want; do
  [ -z "$ins" ] && continue
  if [ "$have_sj" -eq 1 ]; then
    printf '    DEVICE ZXSPECTRUMNEXT\n    ORG $8000\n    %s\n' "$ins" > "$tmp/sj.asm"
    if "$SJASMPLUS" --nologo --raw="$tmp/sj.bin" "$tmp/sj.asm" >/dev/null 2>&1; then
      want=$(od -An -tx1 "$tmp/sj.bin" | tr -s ' ' | sed 's/^ //;s/ $//')
    fi
  fi
  printf '        ORG $8000\n        %s\n' "$ins" > "$tmp/c.asm"
  if ! "$ZENAS" assemble "$tmp/c.asm" "$tmp/c.bin" --next >/dev/null 2>&1; then
    FAIL=$((FAIL+1)); echo "MISSING  $ins  (want $want)"; continue
  fi
  got=$(od -An -tx1 "$tmp/c.bin" | tr -s ' ' | sed 's/^ //;s/ $//')
  if [ "$got" = "$want" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "MISMATCH $ins  got:$got want:$want"; fi
done < "$DIR/z80n_coverage.txt"
rm -rf "$tmp"
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
