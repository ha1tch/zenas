#!/usr/bin/env bash
# release.sh - zenas release hygiene automation
#
# Single-pass release preparation:
#   1. Validates version string and CHANGELOG entry
#   2. Syncs VERSION + pkg/version/version.go
#   3. Builds the project
#   4. Runs tests ONCE
#   5. Verifies all version strings are consistent
#   6. Cuts a checkpoint zip
#
# Usage:
#   ./release.sh <version>            e.g. ./release.sh 0.1.0
#   ./release.sh <version> --no-zip   dry run, no checkpoint
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CUT_ZIP=true
VERSION=""

for arg in "$@"; do
    case "$arg" in
        --no-zip)  CUT_ZIP=false ;;
        --help|-h) sed -n '3,14p' "$0" | sed 's/^# \?//'; exit 0 ;;
        --*)       echo "Unknown option: $arg" >&2; exit 1 ;;
        *)
            if [ -z "$VERSION" ]; then VERSION="$arg"
            else echo "Unexpected argument: $arg" >&2; exit 1; fi ;;
    esac
done

[ -z "$VERSION" ] && { echo "Usage: $0 <version> [--no-zip]" >&2; exit 1; }

step() { echo ""; echo "-- $1"; }
ok()   { echo "   OK $1"; }
warn() { echo "   !! $1"; }
fail() { echo "   FAIL $1" >&2; exit 1; }

# 1. Validate version format
step "Version: $VERSION"
echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$' \
    || fail "Invalid version format. Expected X.Y.Z or X.Y.Z-suffix"
ok "Format valid"

# 1b. Dirty tree warning
if git rev-parse --git-dir > /dev/null 2>&1; then
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
        warn "Working tree has uncommitted changes -- checkpoint will not correspond to a clean commit"
    else
        GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
        ok "Git tree clean at $GIT_SHA"
    fi
fi

# 2. Validate CHANGELOG entry
step "CHANGELOG entry for [$VERSION]"
grep -q "^## \[$VERSION\]" CHANGELOG.md \
    || fail "no CHANGELOG.md entry for [$VERSION]"
ok "CHANGELOG entry present"

# 3. Sync version
step "Sync VERSION + version.go"
./syncver.sh set "$VERSION" >/dev/null
./syncver.sh check >/dev/null && ok "versions in sync"

# 4. Build
step "Build"
mkdir -p build
go build -o build/zenas . || fail "build failed"
ok "built build/zenas"

# 5. Test
step "Test"
go vet ./... || fail "go vet failed"
go test ./... -count=1 || fail "tests failed"
ok "vet + tests passed"

# 6. Verify reported version matches
step "Verify reported version"
REPORTED=$(build/zenas version | awk '{print $2}')
[ "$REPORTED" = "$VERSION" ] || fail "binary reports $REPORTED, expected $VERSION"
ok "binary reports $VERSION"

# 7. Cut checkpoint zip
if $CUT_ZIP; then
    step "Checkpoint zip"
    ZIP="zenas-v${VERSION}-checkpoint.zip"
    rm -f "$ZIP"
    # ship source + project files; exclude build artefacts, VCS, attic, and OS cruft
    zip -rq "$ZIP" . \
        -x '.git/*' -x 'build/*' -x 'attic/*' \
        -x '*.DS_Store' -x 'zenas-v*.zip' -x "$ZIP"
    ok "wrote $ZIP ($(wc -c < "$ZIP") bytes)"
else
    step "Checkpoint zip"
    ok "skipped (--no-zip)"
fi

echo ""
echo "Release $VERSION prepared."
