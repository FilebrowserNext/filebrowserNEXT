package storage

import (
	"time"

	"github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/users"
)

// RevocationStore manages revoked JWT tokens (JTIs).
type RevocationStore interface {
	Revoke(jti string, expiresAt time.Time) error
	IsRevoked(jti string) bool
}

// MFAStore keeps encrypted multi-factor authentication records. Implementations
// must not expose their contents through the regular user API.
type MFAStore interface {
	Get(userID uint) ([]byte, error)
	Save(userID uint, ciphertext []byte) error
	Delete(userID uint) error
}

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Users      users.Store
	Share      *share.Storage
	Auth       *auth.Storage
	Settings   *settings.Storage
	Revocation RevocationStore
	MFA        MFAStore
}
