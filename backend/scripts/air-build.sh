#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

LDFLAGS='-w -s -X github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=commitSHA -X github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=version'
OUT=./tmp/filebrowser

case "$(uname -s)" in
MINGW*|MSYS*|CYGWIN*)
	OUT=./tmp/filebrowser.exe
	;;
esac

export CGO_ENABLED=0
go build -o "$OUT" --tags=debug --ldflags="$LDFLAGS" .
