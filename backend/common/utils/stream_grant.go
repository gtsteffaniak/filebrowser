package utils

// ViewGrant authorizes inline viewing/streaming of a single file for a specific viewer.
// UserID is internal only and never exposed to clients.
type ViewGrant struct {
	UserID    uint64
	ShareHash string
	Source    string
	Path      string
	ExpiresAt int64
}
