//go:build race

package preview

// fillCenterMatchFullMatrix is false under -race: the imaging.Fill parity matrix is CPU-heavy
// and exceeds CI's 30s package timeout when the race detector is on.
const fillCenterMatchFullMatrix = false