package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/novapixel1010/family-holocron/store"
)

// seededSetting records that the starter template was seeded into this database.
const seededSetting = "bio_templates.seeded"

var bioSelect = `SELECT b.id, b.name, b.description, b.content_md, b.sort_order, ` + attrSelect("b") +
	` FROM bio_templates b` + attrJoin("b")

func scanBio(r rowScanner) (*store.BioTemplate, error) {
	b := &store.BioTemplate{}
	var a attrScan
	if err := r.Scan(append([]any{&b.ID, &b.Name, &b.Description, &b.ContentMD, &b.SortOrder}, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	b.Attribution = a.value()
	return b, nil
}

func bioView(b *store.BioTemplate) map[string]any {
	if b == nil {
		return nil
	}
	return map[string]any{"name": b.Name, "description": b.Description, "content": b.ContentMD, "sortOrder": b.SortOrder}
}

func duplicateName(err error) error {
	if isUnique(err) {
		return &store.ValidationError{Message: "A template with this name already exists.", Fields: map[string]string{"name": "is already used by another template"}}
	}
	return err
}

// ListBioTemplates implements store.Driver.
func (d *DB) ListBioTemplates(ctx context.Context) ([]*store.BioTemplate, error) {
	rows, err := d.db.QueryContext(ctx, bioSelect+` ORDER BY b.sort_order, b.name COLLATE NOCASE, b.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*store.BioTemplate{}
	for rows.Next() {
		b, err := scanBio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBioTemplate implements store.Driver.
func (d *DB) GetBioTemplate(ctx context.Context, id int64) (*store.BioTemplate, error) {
	return scanBio(d.db.QueryRowContext(ctx, bioSelect+` WHERE b.id = ?`, id))
}

func (t *tx) insertBio(ctx context.Context, actor store.Actor, in store.BioTemplateInput) (int64, error) {
	ts := now()
	res, err := t.ExecContext(ctx, `INSERT INTO bio_templates (name, description, content_md, sort_order,
		created_by_user_id, updated_by_user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Description, in.ContentMD, in.SortOrder, nullID(actor.UserID), nullID(actor.UserID), ts, ts)
	if err != nil {
		return 0, duplicateName(err)
	}
	return res.LastInsertId()
}

// CreateBioTemplate implements store.Driver.
func (d *DB) CreateBioTemplate(ctx context.Context, actor store.Actor, in store.BioTemplateInput) (*store.BioTemplate, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var out *store.BioTemplate
	err := d.withTx(ctx, func(t *tx) error {
		id, err := t.insertBio(ctx, actor, in)
		if err != nil {
			return err
		}
		if out, err = scanBio(t.QueryRowContext(ctx, bioSelect+` WHERE b.id = ?`, id)); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntityBioTemplate, EntityID: id,
			EntityLabel: out.Name, Changes: store.Diff(nil, bioView(out))})
	})
	return out, err
}

// UpdateBioTemplate implements store.Driver.
func (d *DB) UpdateBioTemplate(ctx context.Context, actor store.Actor, id int64, in store.BioTemplateInput) (*store.BioTemplate, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var out *store.BioTemplate
	err := d.withTx(ctx, func(t *tx) error {
		before, err := scanBio(t.QueryRowContext(ctx, bioSelect+` WHERE b.id = ?`, id))
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE bio_templates SET name = ?, description = ?, content_md = ?, sort_order = ?,
			updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.Name, in.Description, in.ContentMD, in.SortOrder, nullID(actor.UserID), now(), id); err != nil {
			return duplicateName(err)
		}
		if out, err = scanBio(t.QueryRowContext(ctx, bioSelect+` WHERE b.id = ?`, id)); err != nil {
			return err
		}
		changes := store.Diff(bioView(before), bioView(out))
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityBioTemplate, EntityID: id,
			EntityLabel: out.Name, Changes: changes})
	})
	return out, err
}

// DeleteBioTemplate implements store.Driver.
func (d *DB) DeleteBioTemplate(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		before, err := scanBio(t.QueryRowContext(ctx, bioSelect+` WHERE b.id = ?`, id))
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `DELETE FROM bio_templates WHERE id = ?`, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntityBioTemplate, EntityID: id,
			EntityLabel: before.Name, Changes: store.Diff(bioView(before), nil)})
	})
}

// SeedBioTemplates implements store.Driver.
func (d *DB) SeedBioTemplates(ctx context.Context) error {
	return d.withTx(ctx, func(t *tx) error {
		var flag sql.NullString
		if err := t.QueryRowContext(ctx, `SELECT value FROM system_setting WHERE name = ?`, seededSetting).Scan(&flag); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if flag.Valid {
			return nil // seeded before; an admin may have deleted it on purpose
		}
		var n int
		if err := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM bio_templates`).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			in := store.DefaultBioTemplate
			if _, err := t.insertBio(ctx, store.SystemActor, in); err != nil {
				return err
			}
		}
		_, err := t.ExecContext(ctx, `INSERT INTO system_setting (name, value, updated_at) VALUES (?, 'true', ?)`, seededSetting, now())
		return err
	})
}
