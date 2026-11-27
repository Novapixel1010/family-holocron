package sqlite

import (
	"context"
	"strings"

	"github.com/novapixel1010/family-holocron/store"
)

const userCols = `id, username, display_name, email, password_hash, role, status, created_at, updated_at`

func scanUser(r rowScanner) (*store.User, error) {
	u := &store.User{}
	var c, up int64
	if err := r.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &c, &up); err != nil {
		return nil, notFound(err)
	}
	u.CreatedAt, u.UpdatedAt = unix(c), unix(up)
	return u, nil
}

// CountUsers implements store.Driver.
func (d *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user`).Scan(&n)
	return n, err
}

func validateUser(u *store.User) error {
	u.Username = strings.TrimSpace(u.Username)
	u.DisplayName = strings.TrimSpace(u.DisplayName)
	u.Email = strings.TrimSpace(u.Email)
	if len(u.Username) < 2 || len(u.Username) > 64 || strings.ContainsAny(u.Username, " \t\n/\\") {
		return store.Invalid("username", "must be 2–64 characters without spaces or slashes")
	}
	if u.Role == "" {
		u.Role = store.RoleEditor
	}
	if !u.Role.Valid() {
		return store.Invalid("role", "unknown role")
	}
	if u.Status == "" {
		u.Status = store.UserActive
	}
	if u.PasswordHash == "" {
		return store.Invalid("password", "is required")
	}
	if len(u.DisplayName) > 200 {
		return store.Invalid("displayName", "is too long")
	}
	return nil
}

func (t *tx) insertUser(ctx context.Context, u *store.User) (int64, error) {
	ts := now()
	res, err := t.ExecContext(ctx, `INSERT INTO user (username, display_name, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, u.Username, u.DisplayName, u.Email, u.PasswordHash, u.Role, u.Status, ts, ts)
	if isUnique(err) {
		return 0, &store.ValidationError{Message: "That username is already taken.", Fields: map[string]string{"username": "is already taken"}}
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateFirstAdmin creates the initial admin account; it fails with ErrConflict once any user exists.
func (d *DB) CreateFirstAdmin(ctx context.Context, u *store.User) (*store.User, error) {
	u.Role = store.RoleAdmin
	u.Status = store.UserActive
	if err := validateUser(u); err != nil {
		return nil, err
	}
	var id int64
	err := d.withTx(ctx, func(t *tx) error {
		var n int
		if err := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM user`).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return store.ConflictError("setup has already been completed")
		}
		var err error
		if id, err = t.insertUser(ctx, u); err != nil {
			return err
		}
		return t.audit(ctx, store.Actor{UserID: id, Username: u.Username}, store.AuditEntry{
			Action: store.ActionCreate, EntityType: store.EntityUser, EntityID: id, EntityLabel: u.Username,
			Summary: "Initial administrator created during setup",
		})
	})
	if err != nil {
		return nil, err
	}
	return d.GetUser(ctx, id)
}

// CreateUser implements store.Driver.
func (d *DB) CreateUser(ctx context.Context, actor store.Actor, u *store.User) (*store.User, error) {
	if err := validateUser(u); err != nil {
		return nil, err
	}
	var id int64
	err := d.withTx(ctx, func(t *tx) error {
		var err error
		if id, err = t.insertUser(ctx, u); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{
			Action: store.ActionCreate, EntityType: store.EntityUser, EntityID: id, EntityLabel: u.Username,
			Changes: map[string]store.FieldChange{"role": {To: u.Role}},
		})
	})
	if err != nil {
		return nil, err
	}
	return d.GetUser(ctx, id)
}

// GetUser implements store.Driver.
func (d *DB) GetUser(ctx context.Context, id int64) (*store.User, error) {
	return scanUser(d.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM user WHERE id = ?`, id))
}

// GetUserByUsername implements store.Driver.
func (d *DB) GetUserByUsername(ctx context.Context, username string) (*store.User, error) {
	return scanUser(d.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM user WHERE username = ?`, strings.TrimSpace(username)))
}

// ListUsers implements store.Driver.
func (d *DB) ListUsers(ctx context.Context) ([]*store.User, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT `+userCols+` FROM user ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*store.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUser implements store.Driver. It refuses to demote or disable the last active admin.
func (d *DB) UpdateUser(ctx context.Context, actor store.Actor, id int64, upd store.UpdateUser) (*store.User, error) {
	err := d.withTx(ctx, func(t *tx) error {
		before, err := scanUser(t.QueryRowContext(ctx, `SELECT `+userCols+` FROM user WHERE id = ?`, id))
		if err != nil {
			return err
		}
		after := *before
		changes := map[string]store.FieldChange{}
		if upd.DisplayName != nil {
			after.DisplayName = strings.TrimSpace(*upd.DisplayName)
			if len(after.DisplayName) > 200 {
				return store.Invalid("displayName", "is too long")
			}
		}
		if upd.Email != nil {
			after.Email = strings.TrimSpace(*upd.Email)
		}
		if upd.Role != nil {
			if !upd.Role.Valid() {
				return store.Invalid("role", "unknown role")
			}
			after.Role = *upd.Role
		}
		if upd.Status != nil {
			if *upd.Status != store.UserActive && *upd.Status != store.UserDisabled {
				return store.Invalid("status", "unknown status")
			}
			after.Status = *upd.Status
		}
		if upd.PasswordHash != nil {
			after.PasswordHash = *upd.PasswordHash
			changes["password"] = store.FieldChange{From: "•••", To: "•••"}
		}
		wasActiveAdmin := before.Role == store.RoleAdmin && before.Status == store.UserActive
		isActiveAdmin := after.Role == store.RoleAdmin && after.Status == store.UserActive
		if wasActiveAdmin && !isActiveAdmin {
			var n int
			if err := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM user WHERE role = 'admin' AND status = 'active' AND id <> ?`, id).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return store.ConflictError("at least one active administrator is required")
			}
		}
		for k, v := range store.Diff(before, &after) {
			changes[k] = v
		}
		if len(changes) == 0 {
			return nil
		}
		if _, err := t.ExecContext(ctx, `UPDATE user SET display_name = ?, email = ?, password_hash = ?, role = ?, status = ?, updated_at = ? WHERE id = ?`,
			after.DisplayName, after.Email, after.PasswordHash, after.Role, after.Status, now(), id); err != nil {
			return err
		}
		if after.Status == store.UserDisabled {
			if _, err := t.ExecContext(ctx, `DELETE FROM session WHERE user_id = ?`, id); err != nil {
				return err
			}
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityUser, EntityID: id, EntityLabel: before.Username, Changes: changes})
	})
	if err != nil {
		return nil, err
	}
	return d.GetUser(ctx, id)
}

// CreateSession implements store.Driver.
func (d *DB) CreateSession(ctx context.Context, s *store.Session) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO session (token_hash, user_id, created_at, expires_at, last_seen_at, user_agent) VALUES (?, ?, ?, ?, ?, ?)`,
		s.TokenHash, s.UserID, s.CreatedAt.Unix(), s.ExpiresAt.Unix(), s.LastSeenAt.Unix(), truncate(s.UserAgent, 300))
	return err
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// GetSession returns a non-expired session and its user.
func (d *DB) GetSession(ctx context.Context, tokenHash string) (*store.Session, *store.User, error) {
	row := d.db.QueryRowContext(ctx, `SELECT s.token_hash, s.user_id, s.created_at, s.expires_at, s.last_seen_at, s.user_agent,
		u.id, u.username, u.display_name, u.email, u.password_hash, u.role, u.status, u.created_at, u.updated_at
		FROM session s JOIN user u ON u.id = s.user_id WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now())
	s := &store.Session{}
	u := &store.User{}
	var sc, se, sl, uc, uu int64
	err := row.Scan(&s.TokenHash, &s.UserID, &sc, &se, &sl, &s.UserAgent,
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &uc, &uu)
	if err != nil {
		return nil, nil, notFound(err)
	}
	s.CreatedAt, s.ExpiresAt, s.LastSeenAt = unix(sc), unix(se), unix(sl)
	u.CreatedAt, u.UpdatedAt = unix(uc), unix(uu)
	return s, u, nil
}

// TouchSession updates last_seen_at.
func (d *DB) TouchSession(ctx context.Context, tokenHash string) error {
	_, err := d.db.ExecContext(ctx, `UPDATE session SET last_seen_at = ? WHERE token_hash = ?`, now(), tokenHash)
	return err
}

// DeleteSession implements store.Driver.
func (d *DB) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM session WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions removes all of a user's sessions except one.
func (d *DB) DeleteUserSessions(ctx context.Context, userID int64, except string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM session WHERE user_id = ? AND token_hash <> ?`, userID, except)
	return err
}

// DeleteExpiredSessions implements store.Driver.
func (d *DB) DeleteExpiredSessions(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM session WHERE expires_at <= ?`, now())
	return err
}
