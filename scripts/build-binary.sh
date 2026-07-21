#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for target in agent controller; do
  out_dir="$ROOT_DIR/dist/$target"
  name="topo42-$target-linux-x64"
  mkdir -p "$out_dir"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$out_dir/$name" "./cmd/topo42-$target"
  (cd "$out_dir" && sha256sum "$name") > "$out_dir/$name.sha256"
  echo "$out_dir/$name"
done
