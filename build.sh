#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"
target=${1:-$(go env GOOS)}
arch=${2:-amd64}

case "$target" in
  windows)
    CGO_ENABLED=1 GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w -H=windowsgui' -o win-socks5-bridge-native.exe .
    CGO_ENABLED=1 GOOS=windows GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o win-socks5-bridge-cli.exe .
    ;;
  linux)
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o socks5-bridge-linux .
    ;;
  *)
    printf 'Unsupported target: %s (choose windows or linux)\n' "$target" >&2
    exit 2
    ;;
esac
