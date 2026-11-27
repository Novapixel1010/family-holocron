package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/store"
)

var personSelect = `SELECT p.id, p.uid, p.given_name, p.surname, p.nickname, p.sex,
	p.birth_date, p.birth_sort, p.birth_place, p.death_date, p.death_sort, p.death_place,
	p.deceased, p.about, p.portrait_media_id, ` + attrSelect("p") + ` FROM person p` + attrJoin("p")

func scanPerson(r rowScanner) (*store.Person, error) {
	p := &store.Person{}
	var a attrScan
	var portrait sql.NullInt64
	dest := []any{&p.ID, &p.UID, &p.GivenName, &p.Surname, &p.Nickname, &p.Sex,
		&p.BirthDate, &p.BirthSort, &p.BirthPlace, &p.DeathDate, &p.DeathSort, &p.DeathPlace,
		&p.Deceased, &p.About, &portrait}
	if err := r.Scan(append(dest, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	if portrait.Valid {
		p.PortraitMediaID = &portrait.Int64
	}
	p.Attribution = a.value()
	return p, nil
}

func (t *tx) getPerson(ctx context.Context, id int64) (*store.Person, error) {
	return scanPerson(t.QueryRowContext(ctx, personSelect+` WHERE p.id = ?`, id))
}

func (t *tx) insertPerson(ctx context.Context, actor store.Actor, in store.PersonInput) (int64, error) {
	uid := in.UID
	if uid == "" {
		uid = newUID()
	}
	ts := now()
	res, err := t.ExecContext(ctx, `INSERT INTO person (uid, given_name, surname, nickname, sex,
		birth_date, birth_sort, birth_place, death_date, death_sort, death_place, deceased, about, portrait_media_id,
		created_by_user_id, updated_by_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, in.GivenName, in.Surname, in.Nickname, in.Sex,
		in.BirthDate, gendate.Parse(in.BirthDate).SortKey(), in.BirthPlace,
		in.DeathDate, gendate.Parse(in.DeathDate).SortKey(), in.DeathPlace, in.Deceased, in.About,
		nullIDPtr(in.PortraitMediaID), nullID(actor.UserID), nullID(actor.UserID), ts, ts)
	if isUnique(err) {
		return 0, store.Invalid("uid", "a person with this identifier already exists")
	}
	if isFK(err) {
		return 0, store.Invalid("portraitMediaId", "unknown media item")
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreatePerson implements store.Driver.
func (d *DB) CreatePerson(ctx context.Context, actor store.Actor, in store.PersonInput) (*store.Person, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var p *store.Person
	err := d.withTx(ctx, func(t *tx) error {
		id, err := t.insertPerson(ctx, actor, in)
		if err != nil {
			return err
		}
		if p, err = t.getPerson(ctx, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{
			Action: store.ActionCreate, EntityType: store.EntityPerson, EntityID: id, EntityLabel: p.DisplayName(),
			PersonIDs: []int64{id}, Changes: store.Diff(nil, p, "uid", "birthSort", "deathSort"),
		})
	})
	return p, err
}

// GetPerson implements store.Driver.
func (d *DB) GetPerson(ctx context.Context, id int64) (*store.Person, error) {
	return scanPerson(d.db.QueryRowContext(ctx, personSelect+` WHERE p.id = ?`, id))
}

// ListPersons implements store.Driver. It returns the page and the total match count.
func (d *DB) ListPersons(ctx context.Context, f store.FindPerson) ([]*store.Person, int, error) {
	var where []string
	var args []any
	if f.IDs != nil {
		if len(f.IDs) == 0 {
			return nil, 0, nil
		}
		ph, a := inClause(f.IDs)
		where = append(where, "p.id IN ("+ph+")")
		args = append(args, a...)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		for _, term := range strings.Fields(q) {
			like := likeEscape(term)
			where = append(where, `(p.given_name LIKE ? ESCAPE '\' OR p.surname LIKE ? ESCAPE '\' OR p.nickname LIKE ? ESCAPE '\'
				OR p.birth_place LIKE ? ESCAPE '\' OR p.death_place LIKE ? ESCAPE '\' OR p.birth_date LIKE ? ESCAPE '\')`)
			args = append(args, like, like, like, like, like, like)
		}
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM person p`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := ` ORDER BY p.surname COLLATE NOCASE, p.given_name COLLATE NOCASE, p.id`
	switch f.Sort {
	case store.SortByBirth:
		order = ` ORDER BY p.birth_sort = '', p.birth_sort, p.surname COLLATE NOCASE, p.id`
	case store.SortByUpdated:
		order = ` ORDER BY p.updated_at DESC, p.id DESC`
	}
	q := personSelect + cond + order
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, max(f.Offset, 0))
	}
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*store.Person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// UpdatePerson implements store.Driver (full replacement of editable fields).
func (d *DB) UpdatePerson(ctx context.Context, actor store.Actor, id int64, in store.PersonInput) (*store.Person, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var p *store.Person
	err := d.withTx(ctx, func(t *tx) error {
		before, err := t.getPerson(ctx, id)
		if err != nil {
			return err
		}
		_, err = t.ExecContext(ctx, `UPDATE person SET given_name = ?, surname = ?, nickname = ?, sex = ?,
			birth_date = ?, birth_sort = ?, birth_place = ?, death_date = ?, death_sort = ?, death_place = ?,
			deceased = ?, about = ?, portrait_media_id = ?, updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.GivenName, in.Surname, in.Nickname, in.Sex,
			in.BirthDate, gendate.Parse(in.BirthDate).SortKey(), in.BirthPlace,
			in.DeathDate, gendate.Parse(in.DeathDate).SortKey(), in.DeathPlace,
			in.Deceased, in.About, nullIDPtr(in.PortraitMediaID), nullID(actor.UserID), now(), id)
		if isFK(err) {
			return store.Invalid("portraitMediaId", "unknown media item")
		}
		if err != nil {
			return err
		}
		if p, err = t.getPerson(ctx, id); err != nil {
			return err
		}
		changes := store.Diff(before, p, "uid", "birthSort", "deathSort")
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{
			Action: store.ActionUpdate, EntityType: store.EntityPerson, EntityID: id, EntityLabel: p.DisplayName(),
			PersonIDs: []int64{id}, Changes: changes,
		})
	})
	return p, err
}

// DeletePerson removes a person, their relationships, notes and media tags, and
// any events in which they were the only participant.
func (d *DB) DeletePerson(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		p, err := t.getPerson(ctx, id)
		if err != nil {
			return err
		}
		related := []int64{id}
		rows, err := t.QueryContext(ctx, `SELECT CASE WHEN person1_id = ? THEN person2_id ELSE person1_id END
			FROM relationship WHERE person1_id = ? OR person2_id = ?`, id, id, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var rid int64
			if err := rows.Scan(&rid); err != nil {
				rows.Close()
				return err
			}
			related = append(related, rid)
		}
		rows.Close()
		res, err := t.ExecContext(ctx, `DELETE FROM event WHERE id IN (
			SELECT event_id FROM event_participant GROUP BY event_id
			HAVING COUNT(*) = 1 AND MAX(person_id) = ?)`, id)
		if err != nil {
			return err
		}
		orphaned, _ := res.RowsAffected()
		if _, err := t.ExecContext(ctx, `DELETE FROM person WHERE id = ?`, id); err != nil {
			return err
		}
		summary := ""
		if orphaned > 0 {
			summary = fmt.Sprintf("Also removed %d event(s) that only involved this person", orphaned)
		}
		return t.audit(ctx, actor, store.AuditEntry{
			Action: store.ActionDelete, EntityType: store.EntityPerson, EntityID: id, EntityLabel: p.DisplayName(),
			PersonIDs: related, Changes: store.Diff(p, nil, "uid", "birthSort", "deathSort"), Summary: summary,
		})
	})
}

// ---- relationships ---------------------------------------------------------

var relSelect = `SELECT r.id, r.type, r.person1_id, r.person2_id, r.subtype, r.start_date, r.end_date, r.notes, ` +
	attrSelect("r") + ` FROM relationship r` + attrJoin("r")

func scanRel(rs rowScanner) (*store.Relationship, error) {
	r := &store.Relationship{}
	var a attrScan
	dest := []any{&r.ID, &r.Type, &r.Person1ID, &r.Person2ID, &r.Subtype, &r.StartDate, &r.EndDate, &r.Notes}
	if err := rs.Scan(append(dest, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	r.Attribution = a.value()
	return r, nil
}

func (t *tx) personName(ctx context.Context, id int64) (string, error) {
	p, err := t.getPerson(ctx, id)
	if err != nil {
		if err == store.ErrNotFound {
			return "", store.Invalid("person", fmt.Sprintf("person %d does not exist", id))
		}
		return "", err
	}
	return p.DisplayName(), nil
}

// checkRel verifies both people exist and that a parent link would not create a cycle.
func (t *tx) checkRel(ctx context.Context, in store.RelationshipInput, selfID int64) (string, error) {
	n1, err := t.personName(ctx, in.Person1ID)
	if err != nil {
		return "", err
	}
	n2, err := t.personName(ctx, in.Person2ID)
	if err != nil {
		return "", err
	}
	if in.Type == store.RelParent {
		// Person1 (parent) must not already be a descendant of Person2 (child).
		var hit int
		err := t.QueryRowContext(ctx, `WITH RECURSIVE descendant(id) AS (
				SELECT person2_id FROM relationship WHERE type = 'parent' AND person1_id = ? AND id <> ?
				UNION SELECT r.person2_id FROM relationship r JOIN descendant ON r.person1_id = descendant.id WHERE r.type = 'parent' AND r.id <> ?
			) SELECT COUNT(*) FROM descendant WHERE id = ?`, in.Person2ID, selfID, selfID, in.Person1ID).Scan(&hit)
		if err != nil {
			return "", err
		}
		if hit > 0 {
			return "", store.Invalid("person1Id", fmt.Sprintf("%s is a descendant of %s, so cannot be their parent", n1, n2))
		}
		return fmt.Sprintf("%s → parent of %s", n1, n2), nil
	}
	return fmt.Sprintf("%s & %s", n1, n2), nil
}

func (t *tx) insertRel(ctx context.Context, actor store.Actor, in store.RelationshipInput) (int64, error) {
	ts := now()
	res, err := t.ExecContext(ctx, `INSERT INTO relationship (type, person1_id, person2_id, subtype, start_date, end_date, notes,
		created_by_user_id, updated_by_user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Type, in.Person1ID, in.Person2ID, in.Subtype, in.StartDate, in.EndDate, in.Notes,
		nullID(actor.UserID), nullID(actor.UserID), ts, ts)
	if isUnique(err) {
		return 0, store.ConflictError("this relationship already exists")
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateRelationship implements store.Driver.
func (d *DB) CreateRelationship(ctx context.Context, actor store.Actor, in store.RelationshipInput) (*store.Relationship, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var r *store.Relationship
	err := d.withTx(ctx, func(t *tx) error {
		label, err := t.checkRel(ctx, in, 0)
		if err != nil {
			return err
		}
		id, err := t.insertRel(ctx, actor, in)
		if err != nil {
			return err
		}
		if r, err = scanRel(t.QueryRowContext(ctx, relSelect+` WHERE r.id = ?`, id)); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntityRelationship, EntityID: id,
			EntityLabel: label, PersonIDs: []int64{in.Person1ID, in.Person2ID}, Changes: store.Diff(nil, r)})
	})
	return r, err
}

// GetRelationship implements store.Driver.
func (d *DB) GetRelationship(ctx context.Context, id int64) (*store.Relationship, error) {
	return scanRel(d.db.QueryRowContext(ctx, relSelect+` WHERE r.id = ?`, id))
}

// ListRelationships implements store.Driver.
func (d *DB) ListRelationships(ctx context.Context, f store.FindRelationship) ([]*store.Relationship, error) {
	var where []string
	var args []any
	if f.PersonID != nil {
		where = append(where, "(r.person1_id = ? OR r.person2_id = ?)")
		args = append(args, *f.PersonID, *f.PersonID)
	}
	if f.Type != "" {
		where = append(where, "r.type = ?")
		args = append(args, f.Type)
	}
	if f.ChildIDs != nil {
		if len(f.ChildIDs) == 0 {
			return nil, nil
		}
		ph, a := inClause(f.ChildIDs)
		where = append(where, "r.type = 'parent' AND r.person2_id IN ("+ph+")")
		args = append(args, a...)
	}
	if f.ParentIDs != nil {
		if len(f.ParentIDs) == 0 {
			return nil, nil
		}
		ph, a := inClause(f.ParentIDs)
		where = append(where, "r.type = 'parent' AND r.person1_id IN ("+ph+")")
		args = append(args, a...)
	}
	q := relSelect
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := d.db.QueryContext(ctx, q+" ORDER BY r.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*store.Relationship{}
	for rows.Next() {
		r, err := scanRel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRelationship implements store.Driver.
func (d *DB) UpdateRelationship(ctx context.Context, actor store.Actor, id int64, in store.RelationshipInput) (*store.Relationship, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var r *store.Relationship
	err := d.withTx(ctx, func(t *tx) error {
		before, err := scanRel(t.QueryRowContext(ctx, relSelect+` WHERE r.id = ?`, id))
		if err != nil {
			return err
		}
		label, err := t.checkRel(ctx, in, id)
		if err != nil {
			return err
		}
		_, err = t.ExecContext(ctx, `UPDATE relationship SET type = ?, person1_id = ?, person2_id = ?, subtype = ?, start_date = ?,
			end_date = ?, notes = ?, updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.Type, in.Person1ID, in.Person2ID, in.Subtype, in.StartDate, in.EndDate, in.Notes, nullID(actor.UserID), now(), id)
		if isUnique(err) {
			return store.ConflictError("this relationship already exists")
		}
		if err != nil {
			return err
		}
		if r, err = scanRel(t.QueryRowContext(ctx, relSelect+` WHERE r.id = ?`, id)); err != nil {
			return err
		}
		changes := store.Diff(before, r)
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityRelationship, EntityID: id,
			EntityLabel: label, PersonIDs: []int64{before.Person1ID, before.Person2ID, in.Person1ID, in.Person2ID}, Changes: changes})
	})
	return r, err
}

// DeleteRelationship implements store.Driver.
func (d *DB) DeleteRelationship(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		r, err := scanRel(t.QueryRowContext(ctx, relSelect+` WHERE r.id = ?`, id))
		if err != nil {
			return err
		}
		n1, _ := t.personName(ctx, r.Person1ID)
		n2, _ := t.personName(ctx, r.Person2ID)
		if _, err := t.ExecContext(ctx, `DELETE FROM relationship WHERE id = ?`, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntityRelationship, EntityID: id,
			EntityLabel: fmt.Sprintf("%s %s %s", n1, map[string]string{"parent": "→ parent of", "spouse": "&"}[r.Type], n2),
			PersonIDs:   []int64{r.Person1ID, r.Person2ID}, Changes: store.Diff(r, nil)})
	})
}
