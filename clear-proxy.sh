#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if [ -x ./win-socks5-bridge-cli.exe ]; then
      exec ./win-socks5-bridge-cli.exe -clear-proxy
    fi
    ;;
esac
exec go run . -clear-proxy
