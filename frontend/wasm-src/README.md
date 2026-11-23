# OpenSW Go → WebAssembly source

Insipired in the work of [cehaho/OpenSW](https://github.com/cehaho/OpenSW).

Standalone Go module for the OpenSW CAD readers used by the browser worker.
It is intentionally kept under `frontend/` and out of the backend module so no
server-side CAD parsing code ships in the Filebrowser binary.

Building requires Go 1.24 or newer: `wasm_exec.js` is copied from
`$GOROOT/lib/wasm/`, a location introduced in Go 1.24 (older versions use
`misc/wasm`).

Build (from the repository root):

```bash
make build-opensw-wasm
```

That produces `frontend/public/wasm/opensw.wasm` and copies Go's
`wasm_exec.js` alongside it. Both are static assets served by Vite at
`/public/static/wasm/`; there is no backend endpoint involved.

The Docker image builds the same module in the `wasmbuilder` stage before the
frontend build.
