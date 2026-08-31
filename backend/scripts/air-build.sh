#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

LDFLAGS='-w -s -X github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=commitSHA -X github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=version'
OUT=./tmp/filebrowser
USE_MUPDF=1

case "$(uname -s)" in
MINGW*|MSYS*|CYGWIN*)
	OUT=./tmp/filebrowser.exe
	if [[ -n "${CC:-}" ]]; then
		:
	elif command -v zig >/dev/null 2>&1; then
		export CC="zig cc -target x86_64-windows-gnu -lc"
	elif [[ -x /c/msys64/ucrt64/bin/gcc.exe ]]; then
		export CC=/c/msys64/ucrt64/bin/gcc.exe
	else
		USE_MUPDF=0
		echo "Windows: building without MuPDF (install zig or MSYS2 UCRT gcc for PDF previews)." >&2
	fi
	;;
esac

if [[ "$USE_MUPDF" -eq 1 ]]; then
	export CGO_ENABLED=1
	go run ./scripts/setup-gofitz-cgo
	go build -o "$OUT" --tags=mupdf --ldflags="$LDFLAGS" .
else
	export CGO_ENABLED=0
	go build -o "$OUT" --ldflags="$LDFLAGS" .
fi
