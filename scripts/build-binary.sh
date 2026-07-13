#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:?usage: scripts/build-binary.sh agent|controller}"

if [[ "$TARGET" != agent && "$TARGET" != controller ]]; then
  echo "usage: scripts/build-binary.sh agent|controller" >&2
  exit 1
fi
OUT_DIR="$ROOT_DIR/dist/$TARGET"
NAME="topo42-$TARGET-linux-x64"
PACKAGE="./cmd/topo42-$TARGET"

mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$OUT_DIR/$NAME" "$PACKAGE"
(cd "$OUT_DIR" && sha256sum "$NAME") > "$OUT_DIR/$NAME.sha256"
echo "$OUT_DIR/$NAME"
