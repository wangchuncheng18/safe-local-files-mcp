#!/usr/bin/env sh
set -eu

PLUGIN_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${1:-"$PLUGIN_ROOT/bin"}
mkdir -p "$OUT"
cd "$PLUGIN_ROOT"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT/safe-local-files" ./cmd/safe-local-files
echo "Built $OUT/safe-local-files"
