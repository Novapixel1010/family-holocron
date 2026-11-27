// Package auth implements first-admin setup, sign-in, cookie sessions and user management.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// CookieName is the session cookie.
const CookieName = "holocron_session"

// MinPasswordLength is enforced on setup, create and change.
const MinPasswordLength = 8

// Module is the auth domain module.
type Module struct {
	Store         *store.Store
	Logger        *slog.Logger
	SessionTTL    time.Duration
	SecureCookies bool
	limiter       *limiter
	// BcryptCost may be lowered in tests.
	BcryptCost int
	dummyOnce  sync.Once
	dummy      []byte
}

func (m *Module) dummyHash() []byte {
	m.dummyOnce.Do(func() {
		m.dummy, _ = bcrypt.GenerateFromPassword([]byte("holocron-timing-equaliser"), m.BcryptCost)
	})
	return m.dummy
}

// New creates the module.
func New(s *store.Store, logger *slog.Logger, ttl time.Duration, secure bool) *Module {
	return &Module{Store: s, Logger: logger, SessionTTL: ttl, SecureCookies: secure, limiter: newLimiter(10, 15*time.Minute), BcryptCost: bcrypt.DefaultCost}
}

// HashPassword hashes a password with bcrypt.
func (m *Module) HashPassword(pw string) (string, error) {
	if len(pw) < MinPasswordLength {
		return "", store.Invalid("password", "must be at least 8 characters")
	}
	if len(pw) > 72 {
		return "", store.Invalid("password", "must be at most 72 characters")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), m.BcryptCost)
	return string(h), err
}

// HashToken returns the stored form of a session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Middleware resolves the session cookie and attaches the user to the request context.
func (m *Module) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err == nil && c.Value != "" {
			th := HashToken(c.Value)
			sess, u, err := m.Store.GetSession(r.Context(), th)
			if err == nil && u.Status == store.UserActive {
				if time.Since(sess.LastSeenAt) > 5*time.Minute {
					_ = m.Store.TouchSession(r.Context(), th)
				}
				r = r.WithContext(apiutil.WithUser(r.Context(), u, th))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Register mounts auth routes.
func (m *Module) Register(rt *apiutil.Router) {
	rt.Handle("POST /api/v1/auth/setup", apiutil.Public, m.setup)
	rt.Handle("POST /api/v1/auth/login", apiutil.Public, m.login)
	rt.Handle("POST /api/v1/auth/logout", apiutil.Public, m.logout)
	rt.Handle("GET /api/v1/auth/me", apiutil.Viewer, m.me)
	rt.Handle("PATCH /api/v1/auth/me", apiutil.Viewer, m.updateMe)
	rt.Handle("POST /api/v1/auth/me/password", apiutil.Viewer, m.changePassword)
	rt.Handle("GET /api/v1/users", apiutil.Viewer, m.listUsers)
	rt.Handle("POST /api/v1/users", apiutil.Admin, m.createUser)
	rt.Handle("PATCH /api/v1/users/{id}", apiutil.Admin, m.updateUser)
}

type credentials struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

func (m *Module) setup(w http.ResponseWriter, r *http.Request) error {
	var in credentials
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	hash, err := m.HashPassword(in.Password)
	if err != nil {
		return err
	}
	u, err := m.Store.CreateFirstAdmin(r.Context(), &store.User{Username: in.Username, DisplayName: in.DisplayName, Email: in.Email, PasswordHash: hash})
	if err != nil {
		return err
	}
	if err := m.startSession(w, r, u); err != nil {
		return err
	}
	return apiutil.Created(w, u)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (m *Module) login(w http.ResponseWriter, r *http.Request) error {
	var in credentials
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	key := clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(in.Username))
	if !m.limiter.allow(key) {
		return apiutil.Errorf(http.StatusTooManyRequests, "rate_limited", "Too many failed sign-in attempts. Please wait a few minutes and try again.")
	}
	invalid := apiutil.Errorf(http.StatusUnauthorized, "invalid_credentials", "Incorrect username or password.")
	u, err := m.Store.GetUserByUsername(r.Context(), in.Username)
	if errors.Is(err, store.ErrNotFound) {
		// Spend comparable time to avoid user enumeration by timing.
		_ = bcrypt.CompareHashAndPassword(m.dummyHash(), []byte(in.Password))
		m.limiter.fail(key)
		return invalid
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		m.limiter.fail(key)
		return invalid
	}
	if u.Status != store.UserActive {
		return apiutil.Errorf(http.StatusForbidden, "disabled", "This account has been disabled. Ask an administrator for help.")
	}
	m.limiter.reset(key)
	if err := m.startSession(w, r, u); err != nil {
		return err
	}
	return apiutil.OK(w, u)
}

func (m *Module) startSession(w http.ResponseWriter, r *http.Request, u *store.User) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now()
	if err := m.Store.CreateSession(r.Context(), &store.Session{
		TokenHash: HashToken(token), UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(m.SessionTTL), LastSeenAt: now, UserAgent: r.UserAgent(),
	}); err != nil {
		return err
	}
	_ = m.Store.DeleteExpiredSessions(r.Context())
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: m.SecureCookies,
		SameSite: http.SameSiteLaxMode, Expires: now.Add(m.SessionTTL),
	})
	return nil
}

func (m *Module) logout(w http.ResponseWriter, r *http.Request) error {
	if th := apiutil.SessionFrom(r.Context()); th != "" {
		if err := m.Store.DeleteSession(r.Context(), th); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	return apiutil.NoContent(w)
}

func (m *Module) me(w http.ResponseWriter, r *http.Request) error {
	return apiutil.OK(w, apiutil.UserFrom(r.Context()))
}

type profileUpdate struct {
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
}

func (m *Module) updateMe(w http.ResponseWriter, r *http.Request) error {
	var in profileUpdate
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	out, err := m.Store.UpdateUser(r.Context(), u.Actor(), u.ID, store.UpdateUser{DisplayName: in.DisplayName, Email: in.Email})
	if err != nil {
		return err
	}
	return apiutil.OK(w, out)
}

func (m *Module) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.CurrentPassword)) != nil {
		return &store.ValidationError{Message: "Your current password is incorrect.", Fields: map[string]string{"currentPassword": "is incorrect"}}
	}
	hash, err := m.HashPassword(in.NewPassword)
	if err != nil {
		var ve *store.ValidationError
		if errors.As(err, &ve) {
			return &store.ValidationError{Message: "New password " + ve.Fields["password"] + ".", Fields: map[string]string{"newPassword": ve.Fields["password"]}}
		}
		return err
	}
	if _, err := m.Store.UpdateUser(r.Context(), u.Actor(), u.ID, store.UpdateUser{PasswordHash: &hash}); err != nil {
		return err
	}
	// Sign out other devices.
	if err := m.Store.DeleteUserSessions(r.Context(), u.ID, apiutil.SessionFrom(r.Context())); err != nil {
		return err
	}
	return apiutil.NoContent(w)
}

func (m *Module) listUsers(w http.ResponseWriter, r *http.Request) error {
	users, err := m.Store.ListUsers(r.Context())
	if err != nil {
		return err
	}
	if users == nil {
		users = []*store.User{}
	}
	// Non-admins only need names for attribution; hide emails.
	if apiutil.UserFrom(r.Context()).Role != store.RoleAdmin {
		for _, u := range users {
			u.Email = ""
		}
	}
	return apiutil.OK(w, users)
}

func (m *Module) createUser(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		credentials
		Role store.Role `json:"role"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	hash, err := m.HashPassword(in.Password)
	if err != nil {
		return err
	}
	u, err := m.Store.CreateUser(r.Context(), apiutil.ActorFrom(r.Context()), &store.User{
		Username: in.Username, DisplayName: in.DisplayName, Email: in.Email, PasswordHash: hash, Role: in.Role,
	})
	if err != nil {
		return err
	}
	return apiutil.Created(w, u)
}

func (m *Module) updateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		DisplayName *string           `json:"displayName"`
		Email       *string           `json:"email"`
		Role        *store.Role       `json:"role"`
		Status      *store.UserStatus `json:"status"`
		Password    *string           `json:"password"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	upd := store.UpdateUser{DisplayName: in.DisplayName, Email: in.Email, Role: in.Role, Status: in.Status}
	if in.Password != nil && *in.Password != "" {
		hash, err := m.HashPassword(*in.Password)
		if err != nil {
			return err
		}
		upd.PasswordHash = &hash
	}
	u, err := m.Store.UpdateUser(r.Context(), apiutil.ActorFrom(r.Context()), id, upd)
	if err != nil {
		return err
	}
	if upd.PasswordHash != nil {
		_ = m.Store.DeleteUserSessions(r.Context(), id, apiutil.SessionFrom(r.Context()))
	}
	return apiutil.OK(w, u)
}

// limiter is a small in-memory failure counter for sign-in attempts.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) prune(key string) []time.Time {
	cut := time.Now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && h[i].Before(cut) {
		i++
	}
	h = h[i:]
	if len(h) == 0 {
		delete(l.hits, key)
	} else {
		l.hits[key] = h
	}
	return h
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key)) < l.max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.prune(key), time.Now())
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}
