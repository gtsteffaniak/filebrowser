//go:build !(js && wasm)

// Stub so `go build ./...` on other targets still sees a valid main.
package main

func main() {}
