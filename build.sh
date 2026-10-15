#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"
target=${1:-$(go env GOOS)}
arch=${2:-amd64}

# Go may add indirect requirements while resolving modules. Keep that generated
# local graph out of the source tree; go.sum and binaries are ignored as well.
mod_backup=$(mktemp)
cp go.mod "$mod_backup"
restore_mod() {
	cp "$mod_backup" go.mod
	rm -f "$mod_backup"
}
trap restore_mod EXIT HUP INT TERM

case "$target" in
  windows)
    GOFLAGS=-mod=mod CGO_ENABLED=1 GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w -H=windowsgui' -o win-socks5-bridge-native.exe .
    GOFLAGS=-mod=mod CGO_ENABLED=1 GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o win-socks5-bridge-cli.exe .
    ;;
  linux)
    GOFLAGS=-mod=mod CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o socks5-bridge-linux .
    ;;
  *)
    printf 'Unsupported target: %s (choose windows or linux)\n' "$target" >&2
    exit 2
    ;;
esac
