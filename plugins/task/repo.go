package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// repo implements task persistence on the plugin's own tables.
type repo struct{ db store.PluginDB }

const taskSelect = `SELECT t.id, t.title, t.description_md, t.status, t.priority, t.claimed_at,
	(SELECT COUNT(*) FROM plg_task_comments c WHERE c.task_id = t.id),
	cl.id, cl.username, cl.display_name,
	t.created_at, t.updated_at, cu.id, cu.username, cu.display_name, uu.id, uu.username, uu.display_name
	FROM plg_task_task t
	LEFT JOIN user cl ON cl.id = t.claimed_by_user_id
	LEFT JOIN user cu ON cu.id = t.created_by_user_id
	LEFT JOIN user uu ON uu.id = t.updated_by_user_id`

type scanner interface{ Scan(dest ...any) error }

func userRef(id sql.NullInt64, name, disp sql.NullString) *store.UserRef {
	if !id.Valid {
		return nil
	}
	return &store.UserRef{ID: id.Int64, Username: name.String, DisplayName: disp.String}
}

func scanTask(r scanner) (*Task, error) {
	t := &Task{Tags: []string{}, People: []PersonRef{}, Media: []MediaRef{}}
	var claimedAt sql.NullInt64
	var clID, cuID, uuID sql.NullInt64
	var clName, clDisp, cuName, cuDisp, uuName, uuDisp sql.NullString
	var created, updated int64
	err := r.Scan(&t.ID, &t.Title, &t.DescriptionMD, &t.Status, &t.Priority, &claimedAt, &t.CommentCount,
		&clID, &clName, &clDisp, &created, &updated, &cuID, &cuName, &cuDisp, &uuID, &uuName, &uuDisp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.ClaimedBy = userRef(clID, clName, clDisp)
	if claimedAt.Valid && t.ClaimedBy != nil {
		ts := time.Unix(claimedAt.Int64, 0).UTC()
		t.ClaimedAt = &ts
	}
	t.CreatedAt, t.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	t.CreatedBy, t.UpdatedBy = userRef(cuID, cuName, cuDisp), userRef(uuID, uuName, uuDisp)
	return t, nil
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// dbQuerier adapts PluginDB to querier for read paths.
type dbQuerier struct{ db store.PluginDB }

func (q dbQuerier) QueryContext(ctx context.Context, s string, a ...any) (*sql.Rows, error) {
	return q.db.Query(ctx, s, a...)
}
func (q dbQuerier) QueryRowContext(ctx context.Context, s string, a ...any) *sql.Row {
	return q.db.QueryRow(ctx, s, a...)
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// loadRelations fills tags, people and media for the given tasks in three queries.
func loadRelations(ctx context.Context, q querier, tasks []*Task) error {
	if len(tasks) == 0 {
		return nil
	}
	byID := map[int64]*Task{}
	args := make([]any, 0, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
		args = append(args, t.ID)
	}
	in := placeholders(len(tasks))
	rows, err := q.QueryContext(ctx, `SELECT task_id, tag FROM plg_task_tag WHERE task_id IN (`+in+`) ORDER BY tag COLLATE NOCASE`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var tag string
		if err := rows.Scan(&id, &tag); err != nil {
			rows.Close()
			return err
		}
		byID[id].Tags = append(byID[id].Tags, tag)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT tp.task_id, p.id, p.given_name, p.surname, p.birth_date, p.death_date
		FROM plg_task_person tp JOIN person p ON p.id = tp.person_id
		WHERE tp.task_id IN (`+in+`) ORDER BY p.surname COLLATE NOCASE, p.given_name COLLATE NOCASE`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var p store.Person
		if err := rows.Scan(&id, &p.ID, &p.GivenName, &p.Surname, &p.BirthDate, &p.DeathDate); err != nil {
			rows.Close()
			return err
		}
		byID[id].People = append(byID[id].People, PersonRef{ID: p.ID, Name: p.DisplayName(), BirthDate: p.BirthDate, DeathDate: p.DeathDate})
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT tm.task_id, m.id, m.title, m.original_name, m.mime_type
		FROM plg_task_media tm JOIN media m ON m.id = tm.media_id
		WHERE tm.task_id IN (`+in+`) ORDER BY m.id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var m MediaRef
		if err := rows.Scan(&id, &m.ID, &m.Title, &m.OriginalName, &m.MimeType); err != nil {
			return err
		}
		byID[id].Media = append(byID[id].Media, m)
	}
	return rows.Err()
}

func getTask(ctx context.Context, q querier, id int64) (*Task, error) {
	t, err := scanTask(q.QueryRowContext(ctx, taskSelect+` WHERE t.id = ?`, id))
	if err != nil {
		return nil, err
	}
	return t, loadRelations(ctx, q, []*Task{t})
}

// Get returns one task.
func (r *repo) Get(ctx context.Context, id int64) (*Task, error) {
	return getTask(ctx, dbQuerier{r.db}, id)
}

// List returns tasks: active work first, then by priority and most recently updated.
func (r *repo) List(ctx context.Context, f Filter) ([]*Task, error) {
	var where []string
	var args []any
	if len(f.Status) > 0 {
		where = append(where, "t.status IN ("+placeholders(len(f.Status))+")")
		for _, s := range f.Status {
			args = append(args, s)
		}
	}
	if f.Priority != "" {
		where = append(where, "t.priority = ?")
		args = append(args, f.Priority)
	}
	if f.PersonID != nil {
		where = append(where, "t.id IN (SELECT task_id FROM plg_task_person WHERE person_id = ?)")
		args = append(args, *f.PersonID)
	}
	if f.Tag != "" {
		where = append(where, "t.id IN (SELECT task_id FROM plg_task_tag WHERE tag = ?)")
		args = append(args, f.Tag)
	}
	if f.ClaimedBy != nil {
		where = append(where, "t.claimed_by_user_id = ?")
		args = append(args, *f.ClaimedBy)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
		where = append(where, `(t.title LIKE ? ESCAPE '\' OR t.description_md LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	query := taskSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += ` ORDER BY CASE t.status WHEN 'in_progress' THEN 0 WHEN 'open' THEN 1 WHEN 'blocked' THEN 2 ELSE 3 END,
		CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, t.updated_at DESC, t.id DESC LIMIT 1000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []*Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, loadRelations(ctx, dbQuerier{r.db}, out)
}

// Tags returns every tag in use with its task count.
func (r *repo) Tags(ctx context.Context) ([]TagCount, error) {
	rows, err := r.db.Query(ctx, `SELECT tag, COUNT(*) FROM plg_task_tag GROUP BY tag COLLATE NOCASE ORDER BY tag COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TagCount{}
	for rows.Next() {
		var t TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TagCount is a tag with usage count.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func nullID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// writeLinks replaces a task's tags, people and media links.
func writeLinks(ctx context.Context, tx *sql.Tx, id int64, in Input) error {
	for _, stmt := range []string{`DELETE FROM plg_task_tag WHERE task_id = ?`, `DELETE FROM plg_task_person WHERE task_id = ?`, `DELETE FROM plg_task_media WHERE task_id = ?`} {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return err
		}
	}
	for _, tag := range in.Tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plg_task_tag (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return err
		}
	}
	for _, pid := range in.PersonIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plg_task_person (task_id, person_id) VALUES (?, ?)`, id, pid); err != nil {
			if isFK(err) {
				return store.Invalid("personIds", fmt.Sprintf("person %d does not exist", pid))
			}
			return err
		}
	}
	for _, mid := range in.MediaIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plg_task_media (task_id, media_id) VALUES (?, ?)`, id, mid); err != nil {
			if isFK(err) {
				return store.Invalid("mediaIds", fmt.Sprintf("media item %d does not exist", mid))
			}
			return err
		}
	}
	return nil
}

func isFK(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

func auditEntry(action string, before, after *Task, summary string) store.AuditEntry {
	t := after
	if t == nil {
		t = before
	}
	return store.AuditEntry{
		Action: action, EntityType: EntityTask, EntityID: t.ID, EntityLabel: t.Title,
		PersonIDs: append(before.personIDs(), after.personIDs()...),
		Changes:   store.Diff(before.auditView(), after.auditView()), Summary: summary,
	}
}

// Create inserts a task. Creating it as in_progress claims it for the creator.
func (r *repo) Create(ctx context.Context, actor store.Actor, in Input) (*Task, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var out *Task
	err := r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		now := time.Now().Unix()
		var claimedBy, claimedAt any
		if in.Status == StatusInProgress && actor.UserID > 0 {
			claimedBy, claimedAt = actor.UserID, now
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO plg_task_task (title, description_md, status, priority, claimed_by_user_id, claimed_at,
			created_by_user_id, updated_by_user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.Title, in.DescriptionMD, in.Status, in.Priority, claimedBy, claimedAt, nullID(actor.UserID), nullID(actor.UserID), now, now)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if err := writeLinks(ctx, tx, id, in); err != nil {
			return err
		}
		if out, err = getTask(ctx, tx, id); err != nil {
			return err
		}
		return audit(auditEntry(store.ActionCreate, nil, out, ""))
	})
	return out, err
}

// Update replaces a task's editable fields. Setting the status to open releases any
// claim (open means "available"); setting in_progress on an unclaimed task claims it.
func (r *repo) Update(ctx context.Context, actor store.Actor, id int64, in Input) (*Task, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var out *Task
	err := r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, err := getTask(ctx, tx, id)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		claim := ""
		switch {
		case in.Status == StatusOpen && before.ClaimedBy != nil:
			claim = `, claimed_by_user_id = NULL, claimed_at = NULL`
		case in.Status == StatusInProgress && before.ClaimedBy == nil && actor.UserID > 0:
			claim = fmt.Sprintf(`, claimed_by_user_id = %d, claimed_at = %d`, actor.UserID, now)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plg_task_task SET title = ?, description_md = ?, status = ?, priority = ?,
			updated_by_user_id = ?, updated_at = ?`+claim+` WHERE id = ?`,
			in.Title, in.DescriptionMD, in.Status, in.Priority, nullID(actor.UserID), now, id); err != nil {
			return err
		}
		if err := writeLinks(ctx, tx, id, in); err != nil {
			return err
		}
		if out, err = getTask(ctx, tx, id); err != nil {
			return err
		}
		e := auditEntry(store.ActionUpdate, before, out, "")
		if len(e.Changes) == 0 {
			return nil
		}
		return audit(e)
	})
	return out, err
}

// Delete removes a task (links cascade).
func (r *repo) Delete(ctx context.Context, actor store.Actor, id int64) error {
	return r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, err := getTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM plg_task_task WHERE id = ?`, id); err != nil {
			return err
		}
		return audit(auditEntry(store.ActionDelete, before, nil, ""))
	})
}

func name(u *store.UserRef) string {
	if u == nil {
		return "a removed user"
	}
	return firstNonEmpty(u.DisplayName, u.Username)
}

// Claim assigns the task to the actor. An open task moves to in_progress. A task
// claimed by someone else is only taken over when takeOver is set.
func (r *repo) Claim(ctx context.Context, actor store.Actor, id int64, takeOver bool) (*Task, error) {
	var out *Task
	err := r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, err := getTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Status == StatusDone {
			return &apiutil.Error{Status: http.StatusConflict, Code: "task_done", Message: "This task is done. Reopen it before claiming it."}
		}
		if before.ClaimedBy != nil && before.ClaimedBy.ID == actor.UserID {
			out = before
			return nil
		}
		if before.ClaimedBy != nil && !takeOver {
			return &apiutil.Error{Status: http.StatusConflict, Code: "already_claimed",
				Message: fmt.Sprintf("%s is already working on this task.", name(before.ClaimedBy)),
				Fields:  map[string]string{"claimedBy": name(before.ClaimedBy)}}
		}
		status := before.Status
		if status == StatusOpen {
			status = StatusInProgress
		}
		now := time.Now().Unix()
		if _, err := tx.ExecContext(ctx, `UPDATE plg_task_task SET claimed_by_user_id = ?, claimed_at = ?, status = ?,
			updated_by_user_id = ?, updated_at = ? WHERE id = ?`, actor.UserID, now, status, nullID(actor.UserID), now, id); err != nil {
			return err
		}
		if out, err = getTask(ctx, tx, id); err != nil {
			return err
		}
		summary := "Claimed the task"
		if before.ClaimedBy != nil {
			summary = "Took over the task from " + name(before.ClaimedBy)
		}
		return audit(auditEntry(store.ActionUpdate, before, out, summary))
	})
	return out, err
}

// Unclaim releases the actor's claim (admins may release anyone's). A task in
// progress returns to open; blocked and done tasks keep their status.
func (r *repo) Unclaim(ctx context.Context, actor store.Actor, isAdmin bool, id int64) (*Task, error) {
	var out *Task
	err := r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, err := getTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.ClaimedBy == nil {
			out = before
			return nil
		}
		if before.ClaimedBy.ID != actor.UserID && !isAdmin {
			return &apiutil.Error{Status: http.StatusForbidden, Code: "not_claimer",
				Message: fmt.Sprintf("Only %s or an administrator can release this claim. You can take the task over instead.", name(before.ClaimedBy))}
		}
		status := before.Status
		if status == StatusInProgress {
			status = StatusOpen
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plg_task_task SET claimed_by_user_id = NULL, claimed_at = NULL, status = ?,
			updated_by_user_id = ?, updated_at = ? WHERE id = ?`, status, nullID(actor.UserID), time.Now().Unix(), id); err != nil {
			return err
		}
		if out, err = getTask(ctx, tx, id); err != nil {
			return err
		}
		return audit(auditEntry(store.ActionUpdate, before, out, "Released the claim"))
	})
	return out, err
}
