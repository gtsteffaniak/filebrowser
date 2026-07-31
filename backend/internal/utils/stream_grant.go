package utils

// ViewGrant authorizes inline viewing/streaming within a scoped context.
// Source is a storage source name for authenticated browsing, or a share hash on public shares.
type ViewGrant struct {
	Source    string
	ExpiresAt int64
}
