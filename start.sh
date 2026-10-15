#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if [ -x ./win-socks5-bridge-native.exe ]; then
      exec ./win-socks5-bridge-native.exe "$@"
    fi
    ;;
esac
if [ -x ./socks5-bridge-linux ]; then
	exec ./socks5-bridge-linux "$@"
fi
exec env GOFLAGS=-mod=mod go run . "$@"
