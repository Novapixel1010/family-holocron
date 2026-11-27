package task

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// Comment is one entry in a task's research log.
type Comment struct {
	ID        int64          `json:"id"`
	TaskID    int64          `json:"taskId"`
	Author    *store.UserRef `json:"author"` // nil if the account was removed
	BodyMD    string         `json:"bodyMd"`
	Edited    bool           `json:"edited"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

const commentSelect = `SELECT c.id, c.task_id, u.id, u.username, u.display_name, c.body_md, c.edited, c.created_at, c.updated_at
	FROM plg_task_comments c LEFT JOIN user u ON u.id = c.user_id`

func scanComment(r scanner) (*Comment, error) {
	c := &Comment{}
	var uid sql.NullInt64
	var uname, udisp sql.NullString
	var created, updated int64
	err := r.Scan(&c.ID, &c.TaskID, &uid, &uname, &udisp, &c.BodyMD, &c.Edited, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Author = userRef(uid, uname, udisp)
	c.CreatedAt, c.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	return c, nil
}

func normalizeBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", store.Invalid("bodyMd", "write something first")
	}
	if len(body) > 50_000 {
		return "", store.Invalid("bodyMd", "is too long")
	}
	return body, nil
}

// Comments lists a task's comments, oldest first.
func (r *repo) Comments(ctx context.Context, taskID int64) ([]*Comment, error) {
	if _, err := scanTask(r.db.QueryRow(ctx, taskSelect+` WHERE t.id = ?`, taskID)); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, commentSelect+` WHERE c.task_id = ? ORDER BY c.created_at, c.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}

// commentAudit records a comment change against its task (and the task's people).
func commentAudit(t *Task, action, summary string, from, to string) store.AuditEntry {
	changes := map[string]store.FieldChange{}
	if from != to {
		changes["comment"] = store.FieldChange{From: excerpt(from), To: excerpt(to)}
	}
	return store.AuditEntry{Action: action, EntityType: EntityTask, EntityID: t.ID, EntityLabel: t.Title,
		PersonIDs: t.personIDs(), Changes: changes, Summary: summary}
}

// AddComment appends a comment by the actor.
func (r *repo) AddComment(ctx context.Context, actor store.Actor, taskID int64, body string) (*Comment, error) {
	body, err := normalizeBody(body)
	if err != nil {
		return nil, err
	}
	var out *Comment
	err = r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		t, err := getTask(ctx, tx, taskID)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		res, err := tx.ExecContext(ctx, `INSERT INTO plg_task_comments (task_id, user_id, body_md, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			taskID, nullID(actor.UserID), body, now, now)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if out, err = scanComment(tx.QueryRowContext(ctx, commentSelect+` WHERE c.id = ?`, id)); err != nil {
			return err
		}
		return audit(commentAudit(t, store.ActionUpdate, "Added a research note", "", body))
	})
	return out, err
}

// commentForWrite loads a comment of the task and checks the actor may change it.
func commentForWrite(ctx context.Context, tx *sql.Tx, actor store.Actor, isAdmin bool, taskID, commentID int64) (*Comment, *Task, error) {
	t, err := getTask(ctx, tx, taskID)
	if err != nil {
		return nil, nil, err
	}
	c, err := scanComment(tx.QueryRowContext(ctx, commentSelect+` WHERE c.id = ? AND c.task_id = ?`, commentID, taskID))
	if err != nil {
		return nil, nil, err
	}
	if !isAdmin && (c.Author == nil || c.Author.ID != actor.UserID) {
		return nil, nil, &apiutil.Error{Status: http.StatusForbidden, Code: "not_author",
			Message: "Only the comment's author or an administrator can change it."}
	}
	return c, t, nil
}

// EditComment replaces a comment's text (author or admin).
func (r *repo) EditComment(ctx context.Context, actor store.Actor, isAdmin bool, taskID, commentID int64, body string) (*Comment, error) {
	body, err := normalizeBody(body)
	if err != nil {
		return nil, err
	}
	var out *Comment
	err = r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, t, err := commentForWrite(ctx, tx, actor, isAdmin, taskID, commentID)
		if err != nil {
			return err
		}
		if before.BodyMD == body {
			out = before
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plg_task_comments SET body_md = ?, edited = 1, updated_at = ? WHERE id = ?`, body, time.Now().Unix(), commentID); err != nil {
			return err
		}
		if out, err = scanComment(tx.QueryRowContext(ctx, commentSelect+` WHERE c.id = ?`, commentID)); err != nil {
			return err
		}
		return audit(commentAudit(t, store.ActionUpdate, "Edited a research note", before.BodyMD, body))
	})
	return out, err
}

// DeleteComment removes a comment (author or admin).
func (r *repo) DeleteComment(ctx context.Context, actor store.Actor, isAdmin bool, taskID, commentID int64) error {
	return r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, t, err := commentForWrite(ctx, tx, actor, isAdmin, taskID, commentID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM plg_task_comments WHERE id = ?`, commentID); err != nil {
			return err
		}
		summary := "Deleted a research note"
		if before.Author != nil && before.Author.ID != actor.UserID {
			summary = "Deleted a research note by " + name(before.Author)
		}
		return audit(commentAudit(t, store.ActionUpdate, summary, before.BodyMD, ""))
	})
}

// ---- handlers ------------------------------------------------------------------

func commentNotFound(err error) error {
	if err == store.ErrNotFound {
		return apiutil.NotFound("Task or comment")
	}
	return err
}

func (p *Plugin) listComments(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	cs, err := p.repo.Comments(r.Context(), id)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, cs)
}

type commentBody struct {
	BodyMD string `json:"bodyMd"`
}

func (p *Plugin) addComment(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in commentBody
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	c, err := p.repo.AddComment(r.Context(), apiutil.ActorFrom(r.Context()), id, in.BodyMD)
	if err != nil {
		return notFound(err)
	}
	return apiutil.Created(w, c)
}

func (p *Plugin) editComment(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	cid, err := apiutil.PathID(r, "commentId")
	if err != nil {
		return err
	}
	var in commentBody
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	c, err := p.repo.EditComment(r.Context(), u.Actor(), u.Role == store.RoleAdmin, id, cid, in.BodyMD)
	if err != nil {
		return commentNotFound(err)
	}
	return apiutil.OK(w, c)
}

func (p *Plugin) deleteComment(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	cid, err := apiutil.PathID(r, "commentId")
	if err != nil {
		return err
	}
	u := apiutil.UserFrom(r.Context())
	if err := p.repo.DeleteComment(r.Context(), u.Actor(), u.Role == store.RoleAdmin, id, cid); err != nil {
		return commentNotFound(err)
	}
	return apiutil.NoContent(w)
}

func (p *Plugin) setStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := apiutil.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := apiutil.Decode(r, &in); err != nil {
		return err
	}
	t, err := p.repo.SetStatus(r.Context(), apiutil.ActorFrom(r.Context()), id, in.Status)
	if err != nil {
		return notFound(err)
	}
	return apiutil.OK(w, t)
}
