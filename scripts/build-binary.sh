#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:?usage: scripts/build-binary.sh agent|controller}"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required. Install Go 1.22+ first." >&2
  exit 1
fi

case "$TARGET" in
  agent)
    OUT_DIR="$ROOT_DIR/dist/agent"
    NAME="topo42-agent-linux-x64"
    PACKAGE="./cmd/topo42-agent"
    ;;
  controller)
    OUT_DIR="$ROOT_DIR/dist/controller"
    NAME="topo42-controller-linux-x64"
    PACKAGE="./cmd/topo42-controller"
    ;;
  *)
    echo "usage: scripts/build-binary.sh agent|controller" >&2
    exit 1
    ;;
esac

mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$OUT_DIR/$NAME" "$PACKAGE"
(cd "$OUT_DIR" && sha256sum "$NAME") > "$OUT_DIR/$NAME.sha256"
echo "$OUT_DIR/$NAME"
