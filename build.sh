#!/usr/bin/env bash
# Cross-compile zfs-tui for Linux amd64 and arm64.
set -euo pipefail

cd "$(dirname "$0")"

export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.24.2}"
export CGO_ENABLED=0

OUT="${OUT:-dist}"
LDFLAGS="-s -w"

rm -rf "$OUT"
mkdir -p "$OUT"

for goarch in amd64 arm64; do
	env GOOS=linux GOARCH="$goarch" \
		go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/zfs-tui-linux-$goarch" .
	echo "built $OUT/zfs-tui-linux-$goarch"
done
