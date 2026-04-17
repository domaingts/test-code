#!/usr/bin/env bash
#
# check-go-version.sh
# Compares the installed Go version against the recorded version in
# references/go-version.md. Reports whether an update is available.
#
# Usage: bash .claude/skills/go/scripts/check-go-version.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$SKILL_DIR/references/go-version.md"

# Get installed Go version (e.g., "1.26.1")
INSTALLED_RAW=$(go version 2>/dev/null) || {
    echo "ERROR: Go is not installed or not in PATH"
    exit 1
}
INSTALLED=$(echo "$INSTALLED_RAW" | grep -oP 'go\K[0-9]+\.[0-9]+(\.[0-9]+)?')

if [ -z "$INSTALLED" ]; then
    echo "ERROR: Could not parse Go version from: $INSTALLED_RAW"
    exit 1
fi

# Get recorded version from go-version.md
RECORDED=$(grep -oP '^\| \*\*Version\*\* \| \K[0-9]+\.[0-9]+(\.[0-9]+)?' "$VERSION_FILE" 2>/dev/null) || {
    echo "WARNING: Could not read recorded version from $VERSION_FILE"
    echo "INSTALLED=$INSTALLED"
    exit 0
}

if [ "$INSTALLED" = "$RECORDED" ]; then
    echo "up-to-date (Go $INSTALLED)"
else
    # Compare versions using sort -V
    NEWER=$(printf '%s\n%s\n' "$RECORDED" "$INSTALLED" | sort -V | tail -1)
    if [ "$NEWER" = "$INSTALLED" ] && [ "$INSTALLED" != "$RECORDED" ]; then
        echo "UPDATE AVAILABLE: Go $RECORDED → Go $INSTALLED"
        exit 2
    else
        echo "WARNING: Recorded version ($RECORDED) is newer than installed ($INSTALLED)"
        echo "Consider upgrading Go or correcting the recorded version."
        exit 0
    fi
fi
