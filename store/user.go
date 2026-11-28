package store

import "time"

// Role is a user's permission level.
type Role string

const (
	RoleAdmin  Role = "admin"  // everything, incl. users, backup & restore, plugins
	RoleEditor Role = "editor" // create / edit / delete genealogical records
	RoleViewer Role = "viewer" // read-only
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r == RoleAdmin || r == RoleEditor || r == RoleViewer }

// Rank orders roles for access checks.
func (r Role) Rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// UserStatus is active or disabled.
type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User is an account that can sign in.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"` // empty: no local password (e.g. a future identity-provider account)
	// MustChangePassword is set when an admin assigns a temporary password; until the
	// user changes it, only the change-password, me and sign-out routes are allowed.
	MustChangePassword bool       `json:"mustChangePassword"`
	HasLocalPassword   bool       `json:"hasLocalPassword"`
	Role               Role       `json:"role"`
	Status             UserStatus `json:"status"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// Ref returns the compact attribution reference for u.
func (u *User) Ref() *UserRef {
	return &UserRef{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName}
}

// Actor returns the attribution actor for u.
func (u *User) Actor() Actor { return Actor{UserID: u.ID, Username: u.Username} }

// UpdateUser is a partial update; nil fields are left unchanged.
type UpdateUser struct {
	DisplayName        *string
	Email              *string
	PasswordHash       *string
	MustChangePassword *bool
	Role               *Role
	Status             *UserStatus
}

// Session is a signed-in browser session.
type Session struct {
	TokenHash  string
	UserID     int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	UserAgent  string
}
