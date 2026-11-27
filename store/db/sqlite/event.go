package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/store"
)

var eventSelect = `SELECT e.id, e.type, e.title, e.date, e.sort_date, e.place, e.description, e.relationship_id, ` +
	attrSelect("e") + ` FROM event e` + attrJoin("e")

func scanEvent(r rowScanner) (*store.Event, error) {
	e := &store.Event{Participants: []store.EventParticipant{}}
	var a attrScan
	var rel sql.NullInt64
	dest := []any{&e.ID, &e.Type, &e.Title, &e.Date, &e.SortDate, &e.Place, &e.Description, &rel}
	if err := r.Scan(append(dest, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	if rel.Valid {
		e.RelationshipID = &rel.Int64
	}
	e.Attribution = a.value()
	return e, nil
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// loadParticipants fills participants for the given events.
func loadParticipants(ctx context.Context, q querier, events []*store.Event) error {
	if len(events) == 0 {
		return nil
	}
	byID := map[int64]*store.Event{}
	ids := make([]int64, 0, len(events))
	for _, e := range events {
		byID[e.ID] = e
		ids = append(ids, e.ID)
	}
	ph, args := inClause(ids)
	rows, err := q.QueryContext(ctx, `SELECT ep.event_id, ep.person_id, ep.role, p.given_name, p.surname
		FROM event_participant ep JOIN person p ON p.id = ep.person_id
		WHERE ep.event_id IN (`+ph+`) ORDER BY ep.event_id, ep.rowid`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eid int64
		var ep store.EventParticipant
		var given, sur string
		if err := rows.Scan(&eid, &ep.PersonID, &ep.Role, &given, &sur); err != nil {
			return err
		}
		ep.Name = (&store.Person{GivenName: given, Surname: sur}).DisplayName()
		byID[eid].Participants = append(byID[eid].Participants, ep)
	}
	return rows.Err()
}

func (t *tx) getEvent(ctx context.Context, id int64) (*store.Event, error) {
	e, err := scanEvent(t.QueryRowContext(ctx, eventSelect+` WHERE e.id = ?`, id))
	if err != nil {
		return nil, err
	}
	return e, loadParticipants(ctx, t, []*store.Event{e})
}

func (t *tx) writeParticipants(ctx context.Context, eventID int64, parts []store.EventParticipant) error {
	if _, err := t.ExecContext(ctx, `DELETE FROM event_participant WHERE event_id = ?`, eventID); err != nil {
		return err
	}
	for _, p := range parts {
		_, err := t.ExecContext(ctx, `INSERT INTO event_participant (event_id, person_id, role) VALUES (?, ?, ?)`, eventID, p.PersonID, p.Role)
		if isFK(err) {
			return store.Invalid("participants", "one of the selected people no longer exists")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func eventLabel(e *store.Event) string {
	if e.Title != "" {
		return e.Title
	}
	label := strings.ToUpper(e.Type[:1]) + e.Type[1:]
	if len(e.Participants) > 0 {
		names := make([]string, 0, len(e.Participants))
		for _, p := range e.Participants {
			names = append(names, p.Name)
		}
		label += " — " + strings.Join(names, ", ")
	}
	return label
}

func participantIDs(e *store.Event) []int64 {
	ids := make([]int64, 0, len(e.Participants))
	for _, p := range e.Participants {
		ids = append(ids, p.PersonID)
	}
	return ids
}

func (t *tx) insertEvent(ctx context.Context, actor store.Actor, in store.EventInput) (int64, error) {
	ts := now()
	res, err := t.ExecContext(ctx, `INSERT INTO event (type, title, date, sort_date, place, description, relationship_id,
		created_by_user_id, updated_by_user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Type, in.Title, in.Date, gendate.Parse(in.Date).SortKey(), in.Place, in.Description, nullIDPtr(in.RelationshipID),
		nullID(actor.UserID), nullID(actor.UserID), ts, ts)
	if isFK(err) {
		return 0, store.Invalid("relationshipId", "unknown relationship")
	}
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, t.writeParticipants(ctx, id, in.Participants)
}

// CreateEvent implements store.Driver.
func (d *DB) CreateEvent(ctx context.Context, actor store.Actor, in store.EventInput) (*store.Event, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var e *store.Event
	err := d.withTx(ctx, func(t *tx) error {
		id, err := t.insertEvent(ctx, actor, in)
		if err != nil {
			return err
		}
		if e, err = t.getEvent(ctx, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntityEvent, EntityID: id,
			EntityLabel: eventLabel(e), PersonIDs: participantIDs(e), Changes: store.Diff(nil, e, "sortDate")})
	})
	return e, err
}

// GetEvent implements store.Driver.
func (d *DB) GetEvent(ctx context.Context, id int64) (*store.Event, error) {
	e, err := scanEvent(d.db.QueryRowContext(ctx, eventSelect+` WHERE e.id = ?`, id))
	if err != nil {
		return nil, err
	}
	return e, loadParticipants(ctx, d.db, []*store.Event{e})
}

// ListEvents implements store.Driver, ordered chronologically (undated last).
func (d *DB) ListEvents(ctx context.Context, f store.FindEvent) ([]*store.Event, error) {
	var where []string
	var args []any
	if f.IDs != nil {
		if len(f.IDs) == 0 {
			return []*store.Event{}, nil
		}
		ph, a := inClause(f.IDs)
		where = append(where, "e.id IN ("+ph+")")
		args = append(args, a...)
	}
	if f.PersonID != nil {
		where = append(where, "e.id IN (SELECT event_id FROM event_participant WHERE person_id = ?)")
		args = append(args, *f.PersonID)
	}
	if len(f.Types) > 0 {
		ph, a := inClause(f.Types)
		where = append(where, "e.type IN ("+ph+")")
		args = append(args, a...)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := likeEscape(q)
		where = append(where, `(e.title LIKE ? ESCAPE '\' OR e.place LIKE ? ESCAPE '\' OR e.description LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like)
	}
	if f.From != "" {
		where = append(where, "e.sort_date <> '' AND e.sort_date >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "e.sort_date <> '' AND e.sort_date <= ?")
		args = append(args, f.To+"~")
	}
	q := eventSelect
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := d.db.QueryContext(ctx, q+` ORDER BY e.sort_date = '', e.sort_date, e.id`, args...)
	if err != nil {
		return nil, err
	}
	out := []*store.Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, loadParticipants(ctx, d.db, out)
}

// UpdateEvent implements store.Driver.
func (d *DB) UpdateEvent(ctx context.Context, actor store.Actor, id int64, in store.EventInput) (*store.Event, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var e *store.Event
	err := d.withTx(ctx, func(t *tx) error {
		before, err := t.getEvent(ctx, id)
		if err != nil {
			return err
		}
		_, err = t.ExecContext(ctx, `UPDATE event SET type = ?, title = ?, date = ?, sort_date = ?, place = ?, description = ?,
			relationship_id = ?, updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.Type, in.Title, in.Date, gendate.Parse(in.Date).SortKey(), in.Place, in.Description, nullIDPtr(in.RelationshipID),
			nullID(actor.UserID), now(), id)
		if isFK(err) {
			return store.Invalid("relationshipId", "unknown relationship")
		}
		if err != nil {
			return err
		}
		if err := t.writeParticipants(ctx, id, in.Participants); err != nil {
			return err
		}
		if e, err = t.getEvent(ctx, id); err != nil {
			return err
		}
		changes := store.Diff(before, e, "sortDate")
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityEvent, EntityID: id,
			EntityLabel: eventLabel(e), PersonIDs: append(participantIDs(before), participantIDs(e)...), Changes: changes})
	})
	return e, err
}

// DeleteEvent implements store.Driver.
func (d *DB) DeleteEvent(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		e, err := t.getEvent(ctx, id)
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `DELETE FROM event WHERE id = ?`, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntityEvent, EntityID: id,
			EntityLabel: eventLabel(e), PersonIDs: participantIDs(e), Changes: store.Diff(e, nil, "sortDate")})
	})
}

// ---- notes -------------------------------------------------------------------

var noteSelect = `SELECT n.id, n.person_id, n.title, n.body, ` + attrSelect("n") + ` FROM note n` + attrJoin("n")

func scanNote(r rowScanner) (*store.Note, error) {
	n := &store.Note{}
	var a attrScan
	if err := r.Scan(append([]any{&n.ID, &n.PersonID, &n.Title, &n.Body}, a.dest()...)...); err != nil {
		return nil, notFound(err)
	}
	n.Attribution = a.value()
	return n, nil
}

func noteLabel(n *store.Note) string {
	if n.Title != "" {
		return n.Title
	}
	body := strings.TrimSpace(n.Body)
	if len([]rune(body)) > 60 {
		body = string([]rune(body)[:60]) + "…"
	}
	return body
}

// CreateNote implements store.Driver.
func (d *DB) CreateNote(ctx context.Context, actor store.Actor, personID int64, in store.NoteInput) (*store.Note, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var n *store.Note
	err := d.withTx(ctx, func(t *tx) error {
		if _, err := t.getPerson(ctx, personID); err != nil {
			return err
		}
		ts := now()
		res, err := t.ExecContext(ctx, `INSERT INTO note (person_id, title, body, created_by_user_id, updated_by_user_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, personID, in.Title, in.Body, nullID(actor.UserID), nullID(actor.UserID), ts, ts)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if n, err = scanNote(t.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, id)); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionCreate, EntityType: store.EntityNote, EntityID: id,
			EntityLabel: noteLabel(n), PersonIDs: []int64{personID}, Changes: store.Diff(nil, n)})
	})
	return n, err
}

// GetNote implements store.Driver.
func (d *DB) GetNote(ctx context.Context, id int64) (*store.Note, error) {
	return scanNote(d.db.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, id))
}

// ListNotes implements store.Driver (newest first).
func (d *DB) ListNotes(ctx context.Context, personID int64) ([]*store.Note, error) {
	rows, err := d.db.QueryContext(ctx, noteSelect+` WHERE n.person_id = ? ORDER BY n.created_at DESC, n.id DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*store.Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UpdateNote implements store.Driver.
func (d *DB) UpdateNote(ctx context.Context, actor store.Actor, id int64, in store.NoteInput) (*store.Note, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var n *store.Note
	err := d.withTx(ctx, func(t *tx) error {
		before, err := scanNote(t.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, id))
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE note SET title = ?, body = ?, updated_by_user_id = ?, updated_at = ? WHERE id = ?`,
			in.Title, in.Body, nullID(actor.UserID), now(), id); err != nil {
			return err
		}
		if n, err = scanNote(t.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, id)); err != nil {
			return err
		}
		changes := store.Diff(before, n)
		if len(changes) == 0 {
			return nil
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionUpdate, EntityType: store.EntityNote, EntityID: id,
			EntityLabel: noteLabel(n), PersonIDs: []int64{n.PersonID}, Changes: changes})
	})
	return n, err
}

// DeleteNote implements store.Driver.
func (d *DB) DeleteNote(ctx context.Context, actor store.Actor, id int64) error {
	return d.withTx(ctx, func(t *tx) error {
		n, err := scanNote(t.QueryRowContext(ctx, noteSelect+` WHERE n.id = ?`, id))
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `DELETE FROM note WHERE id = ?`, id); err != nil {
			return err
		}
		return t.audit(ctx, actor, store.AuditEntry{Action: store.ActionDelete, EntityType: store.EntityNote, EntityID: id,
			EntityLabel: noteLabel(n), PersonIDs: []int64{n.PersonID}, Changes: store.Diff(n, nil)})
	})
}
