package utils

// ViewGrant authorizes inline viewing/streaming within a scoped source.
// Source combines the storage source name and share hash (when applicable), e.g. "Downloads" or "abc123:Downloads".
type ViewGrant struct {
	Source    string
	ExpiresAt int64
}
