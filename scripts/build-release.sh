#!/usr/bin/env bash
set -euo pipefail
out=${1:?Supply an output directory}
mkdir -p "$out"
for arch in amd64 arm64; do
  case "$arch" in
    amd64) prefix=x86_64-linux-gnu; target=x86_64-unknown-linux-gnu ;;
    arm64) prefix=aarch64-linux-gnu; target=aarch64-unknown-linux-gnu ;;
  esac
  CC="$prefix-gcc" OBJCOPY="$prefix-objcopy" bash scripts/build-native.sh "$target"
  CC="$prefix-gcc" CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -tags nativekv,netgo,osusergo \
    -trimpath -ldflags='-s -w -linkmode external -extldflags "-static"' \
    -o "$out/th0ughts-linux-$arch" ./cmd/th0ughts
  if readelf -l "$out/th0ughts-linux-$arch" | grep -q INTERP; then
    echo "Release binary requires a dynamic loader" >&2; exit 1
  fi
done
(cd "$out" && sha256sum th0ughts-linux-{amd64,arm64} > SHA256SUMS && sha256sum -c SHA256SUMS)
test "$("$out/th0ughts-linux-$(go env GOHOSTARCH)" version)" = "$(cat updates/VERSION)"
