package utils

// ViewGrant authorizes inline viewing/streaming within a scoped context.
// Source is a storage source name for authenticated browsing, or a share hash on public shares.
type ViewGrant struct {
	Source    string
	ExpiresAt int64
	// SessionToken stays server-side. Cookie-less preview requests may use this
	// grant only on read-only view routes, after revalidating the originating session.
	SessionToken string `json:"-"`
}

// ShareAccessGrant tracks remaining uses for count-limited share download tokens.
type ShareAccessGrant struct {
	RemainingUses int
}
