package users

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// NextRandomUserID returns a cryptographically random uint64 for a new user row.
// Values 1, 2, 3, … remain valid for migrated bolt-era data; 0 means “unset” and is never persisted.
func NextRandomUserID() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("random user id: %w", err)
	}
	id := binary.BigEndian.Uint64(b[:])
	if id == 0 {
		return NextRandomUserID()
	}
	return id, nil
}
