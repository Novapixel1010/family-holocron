package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/novapixel1010/family-holocron/store"
)

// MaxAccessTokenDescription bounds a token's label.
const MaxAccessTokenDescription = 100

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func timePtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := unix(v.Int64)
	return &t
}

const accessTokenCols = `t.id, t.user_id, t.description, t.token_hash, t.expires_at, t.last_used_at, t.created_at`

func scanAccessToken(r rowScanner, extra ...any) (*store.AccessToken, error) {
	a := &store.AccessToken{}
	var exp, used sql.NullInt64
	var c int64
	if err := r.Scan(append([]any{&a.ID, &a.UserID, &a.Description, &a.TokenHash, &exp, &used, &c}, extra...)...); err != nil {
		return nil, notFound(err)
	}
	a.ExpiresAt, a.LastUsedAt, a.CreatedAt = timePtr(exp), timePtr(used), unix(c)
	return a, nil
}

// CreateAccessToken implements store.Driver. t.TokenHash must already be set.
func (d *DB) CreateAccessToken(ctx context.Context, actor store.Actor, t *store.AccessToken) (*store.AccessToken, error) {
	t.Description = strings.TrimSpace(t.Description)
	if t.Description == "" {
		return nil, store.Invalid("description", "is required")
	}
	if utf8.RuneCountInString(t.Description) > MaxAccessTokenDescription {
		return nil, store.Invalid("description", "must be at most 100 characters")
	}
	if len(t.TokenHash) != 64 {
		return nil, store.Invalid("token", "is malformed")
	}
	var id int64
	err := d.withTx(ctx, func(tx *tx) error {
		var username string
		if err := tx.QueryRowContext(ctx, `SELECT username FROM user WHERE id = ?`, t.UserID).Scan(&username); err != nil {
			return notFound(err)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO user_access_tokens (user_id, description, token_hash, expires_at, last_used_at, created_at)
			VALUES (?, ?, ?, ?, NULL, ?)`, t.UserID, t.Description, t.TokenHash, nullTime(t.ExpiresAt), now())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		changes := map[string]store.FieldChange{"accessToken": {From: nil, To: t.Description}}
		return tx.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityUser, EntityID: t.UserID,
			EntityLabel: username, Changes: changes, Summary: `Created access token "` + t.Description + `"`})
	})
	if err != nil {
		return nil, err
	}
	return scanAccessToken(d.db.QueryRowContext(ctx, `SELECT `+accessTokenCols+` FROM user_access_tokens t WHERE t.id = ?`, id))
}

// ListAccessTokens implements store.Driver (newest first).
func (d *DB) ListAccessTokens(ctx context.Context, userID int64) ([]*store.AccessToken, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT `+accessTokenCols+` FROM user_access_tokens t WHERE t.user_id = ? ORDER BY t.created_at DESC, t.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*store.AccessToken{}
	for rows.Next() {
		a, err := scanAccessToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteAccessToken implements store.Driver.
func (d *DB) DeleteAccessToken(ctx context.Context, actor store.Actor, userID, id int64) error {
	return d.withTx(ctx, func(tx *tx) error {
		var desc, username string
		err := tx.QueryRowContext(ctx, `SELECT t.description, u.username FROM user_access_tokens t JOIN user u ON u.id = t.user_id
			WHERE t.id = ? AND t.user_id = ?`, id, userID).Scan(&desc, &username)
		if err != nil {
			return notFound(err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_access_tokens WHERE id = ? AND user_id = ?`, id, userID); err != nil {
			return err
		}
		changes := map[string]store.FieldChange{"accessToken": {From: desc, To: nil}}
		return tx.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityUser, EntityID: userID,
			EntityLabel: username, Changes: changes, Summary: `Revoked access token "` + desc + `"`})
	})
}

// GetAccessToken implements store.Driver.
func (d *DB) GetAccessToken(ctx context.Context, tokenHash string) (*store.AccessToken, *store.User, error) {
	row := d.db.QueryRowContext(ctx, `SELECT `+accessTokenCols+`,
		u.id, u.username, u.display_name, u.email, u.password_hash, u.must_change_password, u.role, u.status, u.created_at, u.updated_at
		FROM user_access_tokens t JOIN user u ON u.id = t.user_id WHERE t.token_hash = ?`, tokenHash)
	u := &store.User{}
	var uc, uu int64
	a, err := scanAccessToken(row, &u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash, &u.MustChangePassword, &u.Role, &u.Status, &uc, &uu)
	if err != nil {
		return nil, nil, err
	}
	u.HasLocalPassword = u.PasswordHash != ""
	u.CreatedAt, u.UpdatedAt = unix(uc), unix(uu)
	return a, u, nil
}

// TouchAccessToken records that the token was just used.
func (d *DB) TouchAccessToken(ctx context.Context, id int64) error {
	_, err := d.db.ExecContext(ctx, `UPDATE user_access_tokens SET last_used_at = ? WHERE id = ?`, now(), id)
	return err
}
