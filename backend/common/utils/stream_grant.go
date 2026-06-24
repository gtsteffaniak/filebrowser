package utils

// StreamGrant authorizes inline streaming of a single file for a specific viewer.
// UserID is internal only and never exposed to clients.
type StreamGrant struct {
	UserID    uint64
	ShareHash string
	Source    string
	Path      string
	ExpiresAt int64
}
