# OpenSW Go → WebAssembly source

Standalone Go module for the OpenSW CAD readers used by the browser worker.
It is intentionally kept under `frontend/` and out of the backend module so no
server-side CAD parsing code ships in the Filebrowser binary.

Build (from the repository root):

```bash
make build-opensw-wasm
```

That produces `frontend/public/wasm/opensw.wasm` and copies Go's
`wasm_exec.js` alongside it. Both are static assets served by Vite at
`/public/static/wasm/`; there is no backend endpoint involved.

The Docker image builds the same module in the `wasmbuilder` stage before the
frontend build.
