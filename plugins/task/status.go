package task

import (
	"context"
	"database/sql"
	"net/http"
	"slices"
	"time"

	"github.com/novapixel1010/family-holocron/server/apiutil"
	"github.com/novapixel1010/family-holocron/store"
)

// QuickStatuses are the statuses a claimed task can be switched to from the list.
// "open" is not one of them: a task becomes open again by releasing its claim.
var QuickStatuses = []string{StatusInProgress, StatusBlocked, StatusDone}

// SetStatus changes only the status of a claimed task (the quick switch on task
// cards). Unclaimed tasks are rejected: someone has to claim a task first.
func (r *repo) SetStatus(ctx context.Context, actor store.Actor, id int64, status string) (*Task, error) {
	if !slices.Contains(QuickStatuses, status) {
		return nil, &store.ValidationError{Message: "Status must be in_progress, blocked or done. To reopen a task, release its claim.",
			Fields: map[string]string{"status": "must be in_progress, blocked or done"}}
	}
	var out *Task
	err := r.db.WriteTx(ctx, actor, func(tx *sql.Tx, audit func(store.AuditEntry) error) error {
		before, err := getTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.ClaimedBy == nil {
			return &apiutil.Error{Status: http.StatusConflict, Code: "not_claimed",
				Message: "Claim this task before changing its status."}
		}
		if before.Status == status {
			out = before
			return nil
		}
		// The claimed_by_user_id IS NOT NULL guard repeats the check inside the write itself.
		res, err := tx.ExecContext(ctx, `UPDATE plg_task_task SET status = ?, updated_by_user_id = ?, updated_at = ?
			WHERE id = ? AND claimed_by_user_id IS NOT NULL`, status, nullID(actor.UserID), time.Now().Unix(), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return &apiutil.Error{Status: http.StatusConflict, Code: "not_claimed", Message: "Claim this task before changing its status."}
		}
		if out, err = getTask(ctx, tx, id); err != nil {
			return err
		}
		return audit(auditEntry(store.ActionUpdate, before, out, ""))
	})
	return out, err
}
