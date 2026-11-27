import { useRef, useState, type KeyboardEvent } from "react";
import { ApiError } from "@/api/client";
import { useAuth } from "@/app/auth";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { Markdown, MarkdownEditor } from "@/components/ui/Markdown";
import { ConfirmDialog } from "@/components/ui/Modal";
import { EmptyState, ErrorState, InlineError, LoadingState } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { dateTime, relativeTime, userLabel } from "@/lib/format";
import { useAddComment, useDeleteComment, useEditComment, useTaskComments, type TaskComment } from "./api";

/** Submit on Ctrl/Cmd+Enter from the Markdown textarea. */
function submitOnModEnter(submit: () => void) {
  return (e: KeyboardEvent) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      submit();
    }
  };
}

/** Research log for a task: chronological notes with a reply box at the bottom. */
export function CommentsThread({ taskId }: { taskId: number }) {
  const { canEdit } = useAuth();
  const comments = useTaskComments(taskId);
  const end = useRef<HTMLDivElement>(null);
  return (
    <section aria-labelledby={`notes-${taskId}`} className="border-t border-stone-200 pt-4 dark:border-stone-800">
      <h3 id={`notes-${taskId}`} className="mb-3 font-semibold">
        Research notes{comments.data && comments.data.length > 0 && <span className="ml-1 font-normal text-stone-500">({comments.data.length})</span>}
      </h3>
      {comments.isPending ? (
        <LoadingState label="Loading notes…" className="py-6" />
      ) : comments.isError ? (
        <ErrorState error={comments.error} onRetry={() => comments.refetch()} title="Couldn't load notes" />
      ) : comments.data.length === 0 ? (
        <EmptyState icon="💬" title="No notes yet">
          {canEdit ? "Log what you searched, what you found and where — without changing the task description." : "Nobody has logged any findings yet."}
        </EmptyState>
      ) : (
        <ol className="space-y-3">
          {comments.data.map((c) => <CommentItem key={c.id} comment={c} />)}
        </ol>
      )}
      <div ref={end} />
      {canEdit && <ReplyBox taskId={taskId} onPosted={() => end.current?.scrollIntoView({ block: "nearest", behavior: "smooth" })} />}
    </section>
  );
}

function CommentItem({ comment: c }: { comment: TaskComment }) {
  const { user, isAdmin } = useAuth();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(c.bodyMd);
  const [confirm, setConfirm] = useState(false);
  const edit = useEditComment();
  const del = useDeleteComment();
  const toast = useToast();
  const mine = c.author?.id === user.id;
  const canChange = mine || isAdmin;
  const author = userLabel(c.author);
  const save = async () => {
    try {
      await edit.mutateAsync({ taskId: c.taskId, id: c.id, bodyMd: draft });
      setEditing(false);
    } catch { /* inline */ }
  };
  return (
    <li className="flex gap-3">
      <Avatar name={author} size={32} />
      <div className="min-w-0 flex-1 rounded-xl bg-stone-50 px-3 py-2 ring-1 ring-stone-200 dark:bg-stone-800/60 dark:ring-stone-700">
        <div className="flex flex-wrap items-baseline gap-x-2 text-xs">
          <span className="font-semibold text-stone-800 dark:text-stone-100">{author}</span>
          <time dateTime={c.createdAt} title={dateTime(c.createdAt)} className="text-stone-500">{relativeTime(c.createdAt)}</time>
          {c.edited && <span className="text-stone-400" title={`Edited ${dateTime(c.updatedAt)}`}>(edited)</span>}
          {canChange && !editing && (
            <span className="ml-auto flex gap-1">
              <button type="button" className="rounded px-1 text-stone-500 hover:text-stone-800 dark:hover:text-stone-200" onClick={() => { setDraft(c.bodyMd); setEditing(true); }}
                aria-label={`Edit note by ${author}`}>Edit</button>
              <button type="button" className="rounded px-1 text-red-700 hover:underline dark:text-red-400" onClick={() => setConfirm(true)}
                aria-label={`Delete note by ${author}`}>Delete</button>
            </span>
          )}
        </div>
        {editing ? (
          <div className="mt-2 space-y-2" onKeyDown={submitOnModEnter(save)}>
            <MarkdownEditor label="Edit note" value={draft} onChange={setDraft} rows={4}
              error={edit.error instanceof ApiError ? edit.error.fields.bodyMd : undefined} />
            <InlineError error={edit.error} />
            <div className="flex justify-end gap-2">
              <Button size="sm" onClick={() => { setEditing(false); edit.reset(); }} disabled={edit.isPending}>Cancel</Button>
              <Button size="sm" variant="primary" loading={edit.isPending} onClick={save}>Save</Button>
            </div>
          </div>
        ) : (
          <div className="mt-1 text-sm"><Markdown>{c.bodyMd}</Markdown></div>
        )}
      </div>
      <ConfirmDialog open={confirm} title="Delete this note?" onClose={() => setConfirm(false)} loading={del.isPending} error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try { await del.mutateAsync({ taskId: c.taskId, id: c.id }); toast("Note deleted"); setConfirm(false); } catch { /* inline */ }
        }}>
        {mine ? "Your note will be removed from the research log." : `This removes ${author}’s note from the research log.`} The deletion is kept in the change history.
      </ConfirmDialog>
    </li>
  );
}

function ReplyBox({ taskId, onPosted }: { taskId: number; onPosted: () => void }) {
  const [body, setBody] = useState("");
  const add = useAddComment();
  const submit = async () => {
    if (!body.trim() || add.isPending) return;
    try {
      await add.mutateAsync({ taskId, bodyMd: body });
      setBody("");
      add.reset();
      onPosted();
    } catch { /* inline */ }
  };
  return (
    <div className="mt-4 space-y-2" onKeyDown={submitOnModEnter(submit)}>
      <MarkdownEditor label="Add a research note" value={body} onChange={setBody} rows={3}
        placeholder="e.g. Searched the 1911 census for Leeds — no match. Trying Bradford next."
        error={add.error instanceof ApiError ? add.error.fields.bodyMd : undefined} />
      <InlineError error={add.error} />
      <div className="flex items-center justify-end gap-3">
        <span className="hidden text-xs text-stone-400 sm:inline">Ctrl/⌘ + Enter to post</span>
        <Button variant="primary" size="sm" loading={add.isPending} disabled={!body.trim()} onClick={submit}>Add note</Button>
      </div>
    </div>
  );
}
