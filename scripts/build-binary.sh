#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
mkdir -p "$ROOT_DIR/dist"

for target in agent controller; do
  name="topo42-$target-linux-x64"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$ROOT_DIR/dist/$name" "./cmd/topo42-$target"
  (cd "$ROOT_DIR/dist" && sha256sum "$name") > "$ROOT_DIR/dist/$name.sha256"
  echo "$ROOT_DIR/dist/$name"
done
