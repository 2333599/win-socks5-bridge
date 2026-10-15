#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"
target=${1:-$(go env GOOS)}
arch=${2:-amd64}

# Go may add indirect requirements while resolving modules. Keep that generated
# local graph out of the source tree; go.sum and binaries are ignored as well.
mod_backup=$(mktemp)
build_tmp=$(mktemp -d)
cp go.mod "$mod_backup"
restore_mod() {
	cp "$mod_backup" go.mod
	rm -f "$mod_backup"
	rm -rf "$build_tmp"
}
trap restore_mod EXIT HUP INT TERM

case "$target" in
  windows)
    host=$(go env GOOS)
    wrapper="$build_tmp/gcc-wrapper"
    if [ "$host" = windows ]; then wrapper="$wrapper.exe"; fi
    mingw_gcc=${MINGW_GCC:-gcc}
    if [ "$host" = linux ] && [ -z "${MINGW_GCC:-}" ]; then
      mingw_gcc=x86_64-w64-mingw32-gcc
    fi
    GOFLAGS=-mod=mod CGO_ENABLED=0 go build -trimpath -o "$wrapper" ./cmd/gcc-wrapper
    GOFLAGS=-mod=mod CGO_ENABLED=1 CC="$wrapper" MINGW_GCC="$mingw_gcc" GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w -H=windowsgui' -o win-socks5-bridge-native.exe .
    GOFLAGS=-mod=mod CGO_ENABLED=1 CC="$wrapper" MINGW_GCC="$mingw_gcc" GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o win-socks5-bridge-cli.exe .
    ;;
  linux)
    GOFLAGS=-mod=mod CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o socks5-bridge-linux .
    ;;
  *)
    printf 'Unsupported target: %s (choose windows or linux)\n' "$target" >&2
    exit 2
    ;;
esac
