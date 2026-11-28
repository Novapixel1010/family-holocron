package store

import "time"

// AccessToken is a personal access token (PAT). It acts as its owner: same role,
// same temporary-password lock and the same attribution on every change.
// Only TokenHash, the SHA-256 hex digest, is stored; the plaintext is never kept.
type AccessToken struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"-"`
	Description string     `json:"description"`
	TokenHash   string     `json:"-"`
	ExpiresAt   *time.Time `json:"expiresAt"` // nil: never expires
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Expired reports whether the token has passed its expiry at t.
func (a *AccessToken) Expired(t time.Time) bool {
	return a.ExpiresAt != nil && !t.Before(*a.ExpiresAt)
}
