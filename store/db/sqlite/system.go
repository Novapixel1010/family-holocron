package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/novapixel1010/family-holocron/store"
)

// ---- audit -------------------------------------------------------------------

// ListAudit implements store.Driver (newest first).
func (d *DB) ListAudit(ctx context.Context, f store.FindAudit) ([]*store.AuditEntry, error) {
	var where []string
	var args []any
	if f.EntityType != "" {
		where = append(where, "a.entity_type = ?")
		args = append(args, f.EntityType)
	}
	if f.EntityID != nil {
		where = append(where, "a.entity_id = ?")
		args = append(args, *f.EntityID)
	}
	if f.PersonID != nil {
		where = append(where, "a.id IN (SELECT audit_id FROM audit_person WHERE person_id = ?)")
		args = append(args, *f.PersonID)
	}
	if f.UserID != nil {
		where = append(where, "a.user_id = ?")
		args = append(args, *f.UserID)
	}
	if f.BeforeID > 0 {
		where = append(where, "a.id < ?")
		args = append(args, f.BeforeID)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := `SELECT a.id, a.created_at, u.id, u.username, u.display_name, a.username, a.action, a.entity_type, a.entity_id,
		a.entity_label, a.person_ids, a.changes, a.summary FROM audit_log a LEFT JOIN user u ON u.id = a.user_id`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY a.id DESC LIMIT %d", limit)
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*store.AuditEntry{}
	for rows.Next() {
		e := &store.AuditEntry{}
		var created int64
		var uid sql.NullInt64
		var uname, udisp sql.NullString
		var pids, changes string
		if err := rows.Scan(&e.ID, &created, &uid, &uname, &udisp, &e.Username, &e.Action, &e.EntityType, &e.EntityID,
			&e.EntityLabel, &pids, &changes, &e.Summary); err != nil {
			return nil, err
		}
		e.CreatedAt = unix(created)
		if uid.Valid {
			e.User = &store.UserRef{ID: uid.Int64, Username: uname.String, DisplayName: udisp.String}
		}
		_ = json.Unmarshal([]byte(pids), &e.PersonIDs)
		if e.PersonIDs == nil {
			e.PersonIDs = []int64{}
		}
		_ = json.Unmarshal([]byte(changes), &e.Changes)
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecordAudit writes a standalone audit entry (backups, restores, logins, plugin actions).
func (d *DB) RecordAudit(ctx context.Context, actor store.Actor, e store.AuditEntry) error {
	return d.withTx(ctx, func(t *tx) error { return t.audit(ctx, actor, e) })
}

// ---- settings & plugin storage -----------------------------------------------

// GetSetting implements store.Driver.
func (d *DB) GetSetting(ctx context.Context, name string) (string, bool, error) {
	var v string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM system_setting WHERE name = ?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// SetSetting implements store.Driver.
func (d *DB) SetSetting(ctx context.Context, actor store.Actor, name, value string) error {
	return d.withTx(ctx, func(t *tx) error {
		var old sql.NullString
		if err := t.QueryRowContext(ctx, `SELECT value FROM system_setting WHERE name = ?`, name).Scan(&old); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old.Valid && old.String == value {
			return nil
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO system_setting (name, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, name, value, now()); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntitySetting, EntityLabel: name,
			Changes: map[string]store.FieldChange{"value": {From: old.String, To: value}}})
	})
}

// PluginGet implements store.Driver.
func (d *DB) PluginGet(ctx context.Context, pluginID, key string) (string, bool, error) {
	var v string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM plugin_kv WHERE plugin_id = ? AND key = ?`, pluginID, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// PluginSet implements store.Driver.
func (d *DB) PluginSet(ctx context.Context, pluginID, key, value string) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO plugin_kv (plugin_id, key, value, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(plugin_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, pluginID, key, value, now())
	return err
}

// PluginDelete implements store.Driver.
func (d *DB) PluginDelete(ctx context.Context, pluginID, key string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM plugin_kv WHERE plugin_id = ? AND key = ?`, pluginID, key)
	return err
}

var pluginIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,40}$`)

// ApplyPluginMigrations runs not-yet-applied plugin migrations in version order.
func (d *DB) ApplyPluginMigrations(ctx context.Context, pluginID string, ms []store.PluginMigration) error {
	if !pluginIDRe.MatchString(pluginID) {
		return fmt.Errorf("invalid plugin id %q", pluginID)
	}
	ms = slices.Clone(ms)
	slices.SortFunc(ms, func(a, b store.PluginMigration) int { return a.Version - b.Version })
	for _, m := range ms {
		err := d.withTx(ctx, func(t *tx) error {
			var n int
			if err := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM plugin_migration WHERE plugin_id = ? AND version = ?`, pluginID, m.Version).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return nil
			}
			if _, err := t.ExecContext(ctx, m.SQL); err != nil {
				return fmt.Errorf("plugin %s migration %d: %w", pluginID, m.Version, err)
			}
			_, err := t.ExecContext(ctx, `INSERT INTO plugin_migration (plugin_id, version, applied_at) VALUES (?, ?, ?)`, pluginID, m.Version, now())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ---- stats ---------------------------------------------------------------------

// Stats implements store.Driver.
func (d *DB) Stats(ctx context.Context) (*store.Stats, error) {
	s := &store.Stats{}
	err := d.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM person), (SELECT COUNT(*) FROM relationship), (SELECT COUNT(*) FROM event),
		(SELECT COUNT(*) FROM note), (SELECT COUNT(*) FROM media), (SELECT COUNT(*) FROM user),
		(SELECT COUNT(DISTINCT surname COLLATE NOCASE) FROM person WHERE surname <> '')`).
		Scan(&s.Persons, &s.Relationships, &s.Events, &s.Notes, &s.Media, &s.Users, &s.Surnames)
	return s, err
}

// ---- import ------------------------------------------------------------------

// Import writes a whole batch in one transaction with a single summary audit entry.
func (d *DB) Import(ctx context.Context, actor store.Actor, b *store.ImportBatch) (*store.ImportResult, error) {
	res := &store.ImportResult{Warnings: []string{}}
	err := d.withTx(ctx, func(t *tx) error {
		ids := map[string]int64{}
		for _, ip := range b.Persons {
			in := ip.Input
			in.UID = "" // never trust external ids for uniqueness
			if err := in.Normalize(); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped person %s: %v", ip.Ref, err))
				continue
			}
			id, err := t.insertPerson(ctx, actor, in)
			if err != nil {
				return fmt.Errorf("person %s: %w", ip.Ref, err)
			}
			ids[ip.Ref] = id
			res.Persons++
		}
		for _, r := range b.Relationships {
			p1, ok1 := ids[r.Person1Ref]
			p2, ok2 := ids[r.Person2Ref]
			if !ok1 || !ok2 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped %s relationship %s–%s: unknown person", r.Type, r.Person1Ref, r.Person2Ref))
				continue
			}
			in := store.RelationshipInput{Type: r.Type, Person1ID: p1, Person2ID: p2, Subtype: r.Subtype, StartDate: r.StartDate, EndDate: r.EndDate}
			if err := in.Normalize(); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped relationship %s–%s: %v", r.Person1Ref, r.Person2Ref, err))
				continue
			}
			if _, err := t.checkRel(ctx, in, 0); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped relationship %s–%s: %v", r.Person1Ref, r.Person2Ref, err))
				continue
			}
			if _, err := t.insertRel(ctx, actor, in); err != nil {
				if errors.Is(err, store.ErrConflict) {
					continue // duplicate link in source file
				}
				return err
			}
			res.Relationships++
		}
		for _, ie := range b.Events {
			in := ie.Input
			in.Participants = nil
			for _, ref := range ie.ParticipantRefs {
				if id, ok := ids[ref]; ok {
					in.Participants = append(in.Participants, store.EventParticipant{PersonID: id})
				}
			}
			if err := in.Normalize(); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped %s event: %v", ie.Input.Type, err))
				continue
			}
			if _, err := t.insertEvent(ctx, actor, in); err != nil {
				return err
			}
			res.Events++
		}
		for _, n := range b.Notes {
			pid, ok := ids[n.PersonRef]
			if !ok {
				continue
			}
			in := n.Input
			if err := in.Normalize(); err != nil {
				continue
			}
			ts := now()
			if _, err := t.ExecContext(ctx, `INSERT INTO note (person_id, title, body, created_by_user_id, updated_by_user_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`, pid, in.Title, in.Body, nullID(actor.UserID), nullID(actor.UserID), ts, ts); err != nil {
				return err
			}
			res.Notes++
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionImport, EntityType: store.EntitySystem, EntityLabel: b.Source,
			Summary: fmt.Sprintf("Imported %d people, %d relationships, %d events, %d notes from %s",
				res.Persons, res.Relationships, res.Events, res.Notes, b.Source)})
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ---- snapshot & restore ------------------------------------------------------

// SnapshotTo writes a consistent, compacted copy of the database to path (which must not exist).
func (d *DB) SnapshotTo(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("snapshot target %s already exists", path)
	}
	_, err := d.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}

func tableNames(ctx context.Context, q querier, schema string) ([]string, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM %s.sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%%' ORDER BY name`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func columnNames(ctx context.Context, q querier, schema, table string) ([]string, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info(%s, %s)`, quoteLit(table), quoteLit(schema)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func quoteLit(s string) string   { return `'` + strings.ReplaceAll(s, `'`, `''`) + `'` }

// RestoreFrom replaces every table's contents with those of the (already migrated)
// database at path in a single transaction. Tables that exist only in the backup
// are ignored; tables that exist only here are emptied.
func (d *DB) RestoreFrom(ctx context.Context, path string, keep store.KeepSession) error {
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS src`, path); err != nil {
		return fmt.Errorf("attach backup: %w", err)
	}
	defer conn.ExecContext(context.Background(), `DETACH DATABASE src`)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return err
	}
	mainTables, err := tableNames(ctx, tx, "main")
	if err != nil {
		return err
	}
	srcTables, err := tableNames(ctx, tx, "src")
	if err != nil {
		return err
	}
	// Remember the session to keep (deleting users cascades to sessions).
	var keepRow *store.Session
	if keep.TokenHash != "" {
		s := &store.Session{}
		var c, e, l int64
		err := tx.QueryRowContext(ctx, `SELECT token_hash, user_id, created_at, expires_at, last_seen_at, user_agent FROM main.session WHERE token_hash = ?`, keep.TokenHash).
			Scan(&s.TokenHash, &s.UserID, &c, &e, &l, &s.UserAgent)
		if err == nil {
			s.CreatedAt, s.ExpiresAt, s.LastSeenAt = unix(c), unix(e), unix(l)
			keepRow = s
		}
	}
	// Phase 1: empty everything.
	for _, t := range mainTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+quoteIdent(t)); err != nil {
			return fmt.Errorf("clear %s: %w", t, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM main.sqlite_sequence`); err != nil {
		return err
	}
	// Phase 2: copy rows (sessions are never restored).
	for _, t := range mainTables {
		if t == "session" || !slices.Contains(srcTables, t) {
			continue
		}
		mc, err := columnNames(ctx, tx, "main", t)
		if err != nil {
			return err
		}
		sc, err := columnNames(ctx, tx, "src", t)
		if err != nil {
			return err
		}
		var cols []string
		for _, c := range mc {
			if slices.Contains(sc, c) {
				cols = append(cols, quoteIdent(c))
			}
		}
		if len(cols) == 0 {
			continue
		}
		list := strings.Join(cols, ", ")
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO main.%s (%s) SELECT %s FROM src.%s`, quoteIdent(t), list, list, quoteIdent(t))); err != nil {
			return fmt.Errorf("restore %s: %w", t, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO main.sqlite_sequence (name, seq) SELECT name, seq FROM src.sqlite_sequence`); err != nil {
		return err
	}
	if keepRow != nil {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.user WHERE id = ? AND username = ? AND status = 'active'`, keep.UserID, keep.Username).Scan(&n); err != nil {
			return err
		}
		if n == 1 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO main.session (token_hash, user_id, created_at, expires_at, last_seen_at, user_agent) VALUES (?, ?, ?, ?, ?, ?)`,
				keepRow.TokenHash, keepRow.UserID, keepRow.CreatedAt.Unix(), keepRow.ExpiresAt.Unix(), keepRow.LastSeenAt.Unix(), keepRow.UserAgent); err != nil {
				return err
			}
		}
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA main.foreign_key_check`)
	if err != nil {
		return err
	}
	violations := 0
	for rows.Next() {
		violations++
	}
	rows.Close()
	if violations > 0 {
		return fmt.Errorf("backup has %d foreign key violation(s); restore aborted", violations)
	}
	return tx.Commit()
}
