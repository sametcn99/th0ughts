#!/bin/sh
# Build the archive for the host or an explicit Rust target.
set -eu
cd "$(dirname "$0")/.."
out=${CARGO_TARGET_DIR:-native/target}
if [ "$#" -eq 1 ]; then
  out="$out/$1"
  set -- --target "$1"
fi
cargo build --locked --release --manifest-path native/Cargo.toml "$@"
mkdir -p native/lib storage
# The Go external linker consumes machine code, not Rust's bundled LLVM IR.
"${OBJCOPY:-objcopy}" --remove-section=.llvmbc --remove-section=.llvmcmd \
  "$out/release/libth0ughts_kv.a" native/lib/libth0ughts_kv.a
# Include the archive digest in the Go package's build-cache inputs.
digest=$(sha256sum native/lib/libth0ughts_kv.a)
printf '/* Native storage archive %s */\n' "${digest%% *}" > storage/nativekv_build.h
