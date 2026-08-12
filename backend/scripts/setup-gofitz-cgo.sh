#!/usr/bin/env sh
set -eu

GO_FITZ="$(go list -m -f '{{.Dir}}' github.com/gen2brain/go-fitz)"
rm -f internal/preview/gofitzinclude
ln -sfn "$GO_FITZ/include" internal/preview/gofitzinclude
