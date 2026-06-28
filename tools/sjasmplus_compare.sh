#!/usr/bin/env bash
#
# sjasmplus_compare.sh - optional cross-assembler verification.
#
# sjasmplus is the de facto ZX Spectrum Next assembler and the reference
# implementation of the Z80N extended instruction set (maintained by the same
# author as the official SpecNext opcode tables). This script verifies zenas
# against it for both the base Z80 set and the Z80N set.
#
# It is OPTIONAL: it needs a C++17 toolchain to build sjasmplus and is not part
# of the normal `make test`. Run it via `make test-sjasmplus` or directly.
#
# Usage:
#   tools/sjasmplus_compare.sh [--build] [--keep]
#
#   --build   build sjasmplus from source if no usable binary is found
#   --keep    keep the built sjasmplus binary and work directory
#
# Environment:
#   ZENAS       path to the zenas binary (default: build/zenas)
#   SJASMPLUS   path to an existing sjasmplus binary (skips building)
#   SJ_REF      git ref/tag of sjasmplus to build (default: master)

set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ZENAS="${ZENAS:-$ROOT/build/zenas}"
SJASMPLUS="${SJASMPLUS:-}"
SJ_REF="${SJ_REF:-master}"
DO_BUILD=0
KEEP=0
for arg in "$@"; do
	case "$arg" in
		--build) DO_BUILD=1 ;;
		--keep)  KEEP=1 ;;
		*) echo "unknown option: $arg" >&2; exit 2 ;;
	esac
done

work="$(mktemp -d)"
cleanup() { [ "$KEEP" -eq 1 ] || rm -rf "$work"; }
trap cleanup EXIT

# --- Locate or build sjasmplus -------------------------------------------------

if [ -z "$SJASMPLUS" ]; then
	if command -v sjasmplus >/dev/null 2>&1; then
		SJASMPLUS="$(command -v sjasmplus)"
	elif [ "$DO_BUILD" -eq 1 ]; then
		echo "-- building sjasmplus ($SJ_REF) from source"
		if ! command -v git >/dev/null 2>&1 || ! command -v make >/dev/null 2>&1 || ! command -v g++ >/dev/null 2>&1; then
			echo "SKIP: need git, make and g++ to build sjasmplus" >&2
			exit 0
		fi
		git clone --depth 1 --branch "$SJ_REF" https://github.com/z00m128/sjasmplus.git "$work/sjasmplus" >/dev/null 2>&1 \
			|| git clone --depth 1 https://github.com/z00m128/sjasmplus.git "$work/sjasmplus" >/dev/null 2>&1
		if [ ! -d "$work/sjasmplus" ]; then
			echo "SKIP: could not clone sjasmplus" >&2
			exit 0
		fi
		# USE_LUA=0: we only need the assembler, not the Lua scripting engine,
		# which avoids the LuaBridge submodule.
		( cd "$work/sjasmplus" && make USE_LUA=0 >/dev/null 2>&1 )
		if [ -x "$work/sjasmplus/sjasmplus" ]; then
			SJASMPLUS="$work/sjasmplus/sjasmplus"
			[ "$KEEP" -eq 1 ] && cp "$SJASMPLUS" "$ROOT/build/sjasmplus" && echo "   kept: build/sjasmplus"
		else
			echo "SKIP: sjasmplus build failed" >&2
			exit 0
		fi
	else
		echo "SKIP: sjasmplus not found (pass --build to build it from source)" >&2
		exit 0
	fi
fi

if [ ! -x "$ZENAS" ]; then
	echo "ERROR: zenas binary not found at $ZENAS (run 'make build' first)" >&2
	exit 1
fi

echo "-- zenas:     $ZENAS"
echo "-- sjasmplus: $SJASMPLUS ($("$SJASMPLUS" --version 2>&1 | head -1))"
echo

# --- Helpers -------------------------------------------------------------------

# bytes_zenas <flags> <instruction>  -> hex bytes, or empty on failure
bytes_zenas() {
	local flags="$1" ins="$2"
	printf '        ORG $8000\n        %s\n' "$ins" > "$work/z.asm"
	# shellcheck disable=SC2086
	"$ZENAS" assemble "$work/z.asm" "$work/z.bin" $flags >/dev/null 2>&1 || return 1
	od -An -tx1 "$work/z.bin" | tr -s ' ' | sed 's/^ //;s/ $//'
}

# bytes_sjasm <device> <instruction>  -> hex bytes, or empty on failure
bytes_sjasm() {
	local device="$1" ins="$2"
	printf '    DEVICE %s\n    ORG $8000\n    %s\n' "$device" "$ins" > "$work/s.asm"
	"$SJASMPLUS" --nologo --raw="$work/s.bin" "$work/s.asm" >/dev/null 2>&1 || return 1
	od -An -tx1 "$work/s.bin" | tr -s ' ' | sed 's/^ //;s/ $//'
}

rc=0

# --- Base Z80 ------------------------------------------------------------------

echo "== base Z80 (DEVICE ZXSPECTRUM48) =="
pass=0; fail=0
while IFS= read -r ins; do
	[ -z "$ins" ] && continue
	sj="$(bytes_sjasm ZXSPECTRUM48 "$ins")" || continue   # sjasmplus rejects -> skip
	[ -z "$sj" ] && continue
	zn="$(bytes_zenas '' "$ins")"
	if [ "$zn" = "$sj" ]; then
		pass=$((pass+1))
	else
		fail=$((fail+1)); printf "   DIFF %-18s zenas:%-14s sjasm:%-14s\n" "$ins" "${zn:-REJECT}" "$sj"
	fi
done < "$ROOT/tools/instr_coverage_list.txt"
echo "   base: PASS=$pass FAIL=$fail"
[ "$fail" -eq 0 ] || rc=1

# --- Z80N ----------------------------------------------------------------------

echo "== Z80N (DEVICE ZXSPECTRUMNEXT) =="
pass=0; fail=0
while IFS='|' read -r ins _want; do
	[ -z "$ins" ] && continue
	sj="$(bytes_sjasm ZXSPECTRUMNEXT "$ins")" || { fail=$((fail+1)); echo "   sjasm rejected: $ins"; continue; }
	[ -z "$sj" ] && continue
	zn="$(bytes_zenas '--next' "$ins")"
	if [ "$zn" = "$sj" ]; then
		pass=$((pass+1))
	else
		fail=$((fail+1)); printf "   DIFF %-16s zenas:%-12s sjasm:%-12s\n" "$ins" "${zn:-REJECT}" "$sj"
	fi
done < "$ROOT/tools/z80n_coverage.txt"
echo "   z80n: PASS=$pass FAIL=$fail"
[ "$fail" -eq 0 ] || rc=1

echo
if [ "$rc" -eq 0 ]; then
	echo "OK: zenas matches sjasmplus on every checked form."
else
	echo "FAIL: differences found (see above)."
fi
exit "$rc"
