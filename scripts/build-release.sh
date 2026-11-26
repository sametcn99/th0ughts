#!/usr/bin/env bash
set -euo pipefail
out=${1:?Supply an output directory}
mkdir -p "$out"
for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags="-s -w" \
    -o "$out/th0ughts-linux-$arch" ./cmd/th0ughts
done
(cd "$out" && sha256sum th0ughts-linux-{amd64,arm64} > SHA256SUMS && sha256sum -c SHA256SUMS)
if [[ $(go env GOHOSTOS) == linux ]]; then
  test "$("$out/th0ughts-linux-$(go env GOHOSTARCH)" version)" = "$(cat updates/VERSION)"
fi
