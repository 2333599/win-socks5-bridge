#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

upstream=${1:-${SOCKS5_UPSTREAM:-}}
if [ -z "$upstream" ]; then
	printf '上游 SOCKS5 地址 (例如 socks5://user:pass@host:1080): '
	IFS= read -r upstream
fi
if [ -z "$upstream" ]; then
	printf '%s\n' '上游地址不能为空。' >&2
	exit 2
fi

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if [ -x ./win-socks5-bridge-cli.exe ]; then
      exec ./win-socks5-bridge-cli.exe -listen "${SOCKS5_LISTEN:-127.0.0.1:1080}" -upstream "$upstream"
    fi
    ;;
esac
if [ -x ./socks5-bridge-linux ]; then
	exec ./socks5-bridge-linux -listen "${SOCKS5_LISTEN:-127.0.0.1:1080}" -upstream "$upstream"
fi
exec env GOFLAGS=-mod=mod go run . -listen "${SOCKS5_LISTEN:-127.0.0.1:1080}" -upstream "$upstream"
