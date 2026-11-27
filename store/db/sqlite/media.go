package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/novapixel1010/family-holocron/store"
)

var mediaSelect = `SELECT m.id, m.uid, m.storage_path, m.original_name, m.mime_type, m.size_bytes, m.sha256,
	m.title, m.description, m.taken_date, ` + attrSelect("m") + ` FROM media m` + attrJoin("m")

func scanMedia(r rowScanner) (*store.Media, error) {
	m := &store.Media{PersonIDs: []int64{}}
	var a attrScan
	dest := []any{&m.ID, &m.UID, &m.StoragePath, &m.OriginalName, &m.MimeType, &m.SizeBytes, &m.SHA256, &m.Title, &m.Description, &m.TakenDate}
	if err := r.Scan(append(dest, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	m.Attribution = a.value()
	return m, nil
}

func loadMediaPersons(ctx context.Context, q querier, items []*store.Media) error {
	if len(items) == 0 {
		return nil
	}
	byID := map[int64]*store.Media{}
	ids := make([]int64, 0, len(items))
	for _, m := range items {
		byID[m.ID] = m
		ids = append(ids, m.ID)
	}
	ph, args := inClause(ids)
	rows, err := q.QueryContext(ctx, `SELECT media_id, person_id FROM media_person WHERE media_id IN (`+ph+`) ORDER BY person_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var mid, pid int64
		if err := rows.Scan(&mid, &pid); err != nil {
			return err
		}
		byID[mid].PersonIDs = append(byID[mid].PersonIDs, pid)
	}
	return rows.Err()
}

func (t *tx) getMedia(ctx context.Context, id int64) (*store.Media, error) {
	m, err := scanMedia(t.QueryRowContext(ctx, mediaSelect+` WHERE m.id = ?`, id))
	if err != nil {
		return nil, err
	}
	return m, loadMediaPersons(ctx, t, []*store.Media{m})
}

func (t *tx) writeMediaPersons(ctx context.Context, mediaID int64, personIDs []int64) error {
	if _, err := t.ExecContext(ctx, `DELETE FROM media_person WHERE media_id = ?`, mediaID); err != nil {
		return err
	}
	for _, pid := range personIDs {
		_, err := t.ExecContext(ctx, `INSERT INTO media_person (media_id, person_id) VALUES (?, ?)`, mediaID, pid)
		if isFK(err) {
			return store.Invalid("personIds", fmt.Sprintf("person %d does not exist", pid))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func mediaLabel(m *store.Media) string {
	if m.Title != "" {
		return m.Title
	}
	return m.OriginalName
}

// CreateMedia implements store.Driver. StoragePath, MimeType, SizeBytes and SHA256 must be set by the caller.
func (d *DB) CreateMedia(ctx context.Context, actor store.Actor, m *store.Media) (*store.Media, error) {
	in := store.MediaInput{Title: m.Title, Description: m.Description, TakenDate: m.TakenDate, PersonIDs: m.PersonIDs}
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if m.UID == "" {
		m.UID = newUID()
	}
	var out *store.Media
	err := d.withTx(ctx, func(t *tx) error {
		ts := now()
		res, err := t.ExecContext(ctx, `INSERT INTO media (uid, storage_path, original_name, mime_type, size_bytes, sha256, title,
			description, taken_date, created_by_user_id, updated_by_user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.UID, m.StoragePath, m.OriginalName, m.MimeType, m.SizeBytes, m.SHA256, in.Title, in.Description, in.TakenDate,
			nullID(actor.UserID), nullID(actor.UserID), ts, ts)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if err := t.writeMediaPersons(ctx, id, in.PersonIDs); err != nil {
			return err
		}
		if out, err = t.getMedia(ctx, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntityMedia, EntityID: id,
			EntityLabel: mediaLabel(out), PersonIDs: out.PersonIDs, Changes: store.Diff(nil, out, "uid", "sha256")})
	})
	return out, err
}

// GetMedia implements store.Driver.
func (d *DB) GetMedia(ctx context.Context, id int64) (*store.Media, error) {
	m, err := scanMedia(d.db.QueryRowContext(ctx, mediaSelect+` WHERE m.id = ?`, id))
	if err != nil {
		return nil, err
	}
	return m, loadMediaPersons(ctx, d.db, []*store.Media{m})
}

// ListMedia implements store.Driver (newest first).
func (d *DB) ListMedia(ctx context.Context, f store.FindMedia) ([]*store.Media, int, error) {
	var where []string
	var args []any
	if f.PersonID != nil {
		where = append(where, "m.id IN (SELECT media_id FROM media_person WHERE person_id = ?)")
		args = append(args, *f.PersonID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := likeEscape(q)
		where = append(where, `(m.title LIKE ? ESCAPE '\' OR m.original_name LIKE ? ESCAPE '\' OR m.description LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media m`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := mediaSelect + cond + ` ORDER BY m.created_at DESC, m.id DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, max(f.Offset, 0))
	}
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	out := []*store.Media{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, loadMediaPersons(ctx, d.db, out)
}

// UpdateMedia implements store.Driver.
func (d *DB) UpdateMedia(ctx context.Context, actor store.Actor, id int64, in store.MediaInput) (*store.Media, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var out *store.Media
	err := d.withTx(ctx, func(t *tx) error {
		before, err := t.getMedia(ctx, id)
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE media SET title = ?, description = ?, taken_date = ?, updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.Title, in.Description, in.TakenDate, nullID(actor.UserID), now(), id); err != nil {
			return err
		}
		if err := t.writeMediaPersons(ctx, id, in.PersonIDs); err != nil {
			return err
		}
		if out, err = t.getMedia(ctx, id); err != nil {
			return err
		}
		changes := store.Diff(before, out, "uid", "sha256")
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityMedia, EntityID: id,
			EntityLabel: mediaLabel(out), PersonIDs: append(before.PersonIDs, out.PersonIDs...), Changes: changes})
	})
	return out, err
}

// DeleteMedia implements store.Driver. The caller removes the file afterwards.
func (d *DB) DeleteMedia(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		m, err := t.getMedia(ctx, id)
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `DELETE FROM media WHERE id = ?`, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntityMedia, EntityID: id,
			EntityLabel: mediaLabel(m), PersonIDs: m.PersonIDs, Changes: store.Diff(m, nil, "uid", "sha256")})
	})
}
