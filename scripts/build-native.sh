#!/usr/bin/env bash
# Build the native archive for the current host, or an explicit Rust target.
set -euo pipefail
cd "$(dirname "$0")/.."
args=()
out=${CARGO_TARGET_DIR:-native/target}
if [[ $# == 1 ]]; then args=(--target "$1"); out="$out/$1"; fi
cargo build --locked --release --manifest-path native/Cargo.toml "${args[@]}"
mkdir -p native/lib
# Rust ships LLVM IR alongside native machine code in some standard library objects.
# Keep only the native sections consumed by the Go external linker.
"${OBJCOPY:-objcopy}" --remove-section=.llvmbc --remove-section=.llvmcmd "$out/release/libth0ughts_kv.a" native/lib/libth0ughts_kv.a
