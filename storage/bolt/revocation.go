package bolt

import (
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"
)

// RevokedToken represents a revoked JWT token in the database.
type RevokedToken struct {
	JTI       string    `storm:"id"`
	ExpiresAt time.Time `storm:"index"`
}

type revocationBackend struct {
	db    *storm.DB
	cache sync.Map
}

func newRevocationBackend(db *storm.DB) *revocationBackend {
	rb := &revocationBackend{db: db}
	// Warm in-memory cache with non-expired tokens from db
	var tokens []RevokedToken
	now := time.Now()
	if err := db.Select(q.Gt("ExpiresAt", now)).Find(&tokens); err == nil {
		for _, t := range tokens {
			rb.cache.Store(t.JTI, t.ExpiresAt)
		}
	}
	// Run an asynchronous cleanup of expired tokens
	go rb.cleanupExpired()
	return rb
}

func (rb *revocationBackend) Revoke(jti string, expiresAt time.Time) error {
	if jti == "" {
		return nil
	}
	rb.cache.Store(jti, expiresAt)
	return rb.db.Save(&RevokedToken{
		JTI:       jti,
		ExpiresAt: expiresAt,
	})
}

func (rb *revocationBackend) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	if expVal, ok := rb.cache.Load(jti); ok {
		if exp, ok := expVal.(time.Time); ok {
			if time.Now().Before(exp) {
				return true
			}
			rb.cache.Delete(jti)
			return false
		}
		return true
	}

	var token RevokedToken
	if err := rb.db.One("JTI", jti, &token); err == nil {
		if time.Now().Before(token.ExpiresAt) {
			rb.cache.Store(jti, token.ExpiresAt)
			return true
		}
		_ = rb.db.DeleteStruct(&token)
		return false
	}
	return false
}

func (rb *revocationBackend) cleanupExpired() {
	now := time.Now()
	var expired []RevokedToken
	if err := rb.db.Select(q.Lte("ExpiresAt", now)).Find(&expired); err == nil {
		for _, t := range expired {
			_ = rb.db.DeleteStruct(&t)
		}
	}
}
