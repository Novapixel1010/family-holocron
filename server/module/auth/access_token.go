package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Personal access tokens (PATs), modeled on usememos/memos. A token is sent as
// "Authorization: Bearer fh_pat_…" and acts as its owner: the same role, the same
// temporary-password lock and the same attribution in the audit log.
const (
	AccessTokenPrefix = "fh_pat_"
	// accessTokenRandLen base62 characters ≈ 190 bits of entropy.
	accessTokenRandLen = 32
	// MaxAccessTokenDays bounds expiresInDays.
	MaxAccessTokenDays = 3650
	// accessTokenTouchEvery throttles last_used_at writes so busy clients don't
	// turn every request into a database write.
	accessTokenTouchEvery = time.Minute

	RouteListAccessTokens  = "GET /api/v1/user/access-tokens"
	RouteCreateAccessToken = "POST /api/v1/user/access-tokens"
	RouteDeleteAccessToken = "DELETE /api/v1/user/access-tokens/{id}"
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// NewAccessToken returns a fresh plaintext token: fh_pat_ + 32 random base62 characters.
func NewAccessToken() (string, error) {
	var b strings.Builder
	b.Grow(len(AccessTokenPrefix) + accessTokenRandLen)
	b.WriteString(AccessTokenPrefix)
	limit := big.NewInt(int64(len(base62)))
	for range accessTokenRandLen {
		n, err := rand.Int(rand.Reader, limit) // uniform, no modulo bias
		if err != nil {
			return "", err
		}
		b.WriteByte(base62[n.Int64()])
	}
	return b.String(), nil
}

func validAccessTokenFormat(token string) bool {
	rest, ok := strings.CutPrefix(token, AccessTokenPrefix)
	if !ok || len(rest) != accessTokenRandLen {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if !strings.ContainsRune(base62, rune(rest[i])) {
			return false
		}
	}
	return true
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
// Other authorization schemes are ignored (and fall back to the session cookie).
func bearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, token, _ := strings.Cut(h, " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(token), true // an empty token is still a (bad) bearer token
}

var (
	errInvalidToken = apiutil.Errorf(http.StatusUnauthorized, "invalid_token", "This access token is invalid or has been revoked.")
	errExpiredToken = apiutil.Errorf(http.StatusUnauthorized, "token_expired", "This access token has expired.")
)

// authenticateAccessToken resolves a bearer token to its owner and returns the
// request context carrying that user, or a 401 error.
func (m *Module) authenticateAccessToken(ctx context.Context, token string) (context.Context, error) {
	if !validAccessTokenFormat(token) {
		return nil, errInvalidToken
	}
	at, u, err := m.Store.GetAccessToken(ctx, HashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return nil, errInvalidToken
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if at.Expired(now) {
		return nil, errExpiredToken
	}
	if u.Status != store.UserActive {
		return nil, apiutil.Errorf(http.StatusUnauthorized, "disabled", "The account that owns this access token has been disabled.")
	}
	if at.LastUsedAt == nil || now.Sub(*at.LastUsedAt) >= accessTokenTouchEvery {
		// At most one small write per token per minute; a failure here must not fail the request.
		tctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if err := m.Store.TouchAccessToken(tctx, at.ID); err != nil && m.Logger != nil {
			m.Logger.Warn("record access token use", "err", err)
		}
		cancel()
	}
	ctx = apiutil.WithUser(ctx, u, "")
	return apiutil.WithAccessToken(ctx, at.ID), nil
}

func (m *Module) listAccessTokens(w http.ResponseWriter, r *http.Request) error {
	list, err := m.Store.ListAccessTokens(r.Context(), apiutil.UserFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	return apiutil.OK(w, list)
}

// createdAccessToken is the creation response: the only time the plaintext is returned.
type createdAccessToken struct {
	*store.AccessToken
	Token string `json:"token"`
}

func (m *Module) createAccessToken(w http.ResponseWriter, r *http.Request) error {
	// A token can't mint tokens: otherwise a leaked short-lived token could create a
	// never-expiring one. Creating a token needs a signed-in browser session.
	if apiutil.AccessTokenFrom(r.Context()) != 0 {
		return apiutil.Errorf(http.StatusForbidden, "session_required", "Access tokens can only be created from a signed-in browser session.")
	}
	var in struct {
		Description   string `json:"description"`
		ExpiresInDays *int   `json:"expiresInDays"` // null: never expires
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	at := &store.AccessToken{UserID: u.ID, Description: in.Description}
	if in.ExpiresInDays != nil {
		if *in.ExpiresInDays < 1 || *in.ExpiresInDays > MaxAccessTokenDays {
			return store.Invalid("expiresInDays", "must be between 1 and 3650 days, or empty for no expiry")
		}
		exp := time.Now().Add(time.Duration(*in.ExpiresInDays) * 24 * time.Hour).Truncate(time.Second).UTC()
		at.ExpiresAt = &exp
	}
	token, err := NewAccessToken()
	if err != nil {
		return err
	}
	at.TokenHash = HashToken(token)
	created, err := m.Store.CreateAccessToken(r.Context(), u.Actor(), at)
	if err != nil {
		return err
	}
	return apiutil.Created(w, createdAccessToken{AccessToken: created, Token: token})
}

func (m *Module) deleteAccessToken(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	if err := m.Store.DeleteAccessToken(r.Context(), u.Actor(), u.ID, id); errors.Is(err, store.ErrNotFound) {
		return apiutil.NotFound("access token")
	} else if err != nil {
		return err
	}
	return apiutil.NoContent(w)
}
