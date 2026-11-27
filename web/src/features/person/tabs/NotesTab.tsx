import { useState } from "react";
import { ApiError } from "@/api/client";
import { useCreateNote, useDeleteNote, useNotes, useUpdateNote } from "@/api/hooks";
import type { Note } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { Markdown, MarkdownEditor } from "@/components/ui/Markdown";
import { ConfirmDialog } from "@/components/ui/Modal";
import { EmptyState, InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";

export function NotesTab({ person, canEdit }: PersonTabProps) {
  const notes = useNotes(person.id);
  const [creating, setCreating] = useState(false);
  return (
    <div className="space-y-4">
      {canEdit && !creating && <div className="flex justify-end"><Button variant="primary" size="sm" onClick={() => setCreating(true)}>+ Add note</Button></div>}
      {creating && <NoteEditor personId={person.id} onDone={() => setCreating(false)} />}
      <QueryBoundary query={notes} errorTitle="Couldn't load notes">
        {(list) =>
          list.length === 0 && !creating ? (
            <EmptyState icon="✐" title="No notes yet" action={canEdit && <Button onClick={() => setCreating(true)}>Write a note</Button>}>
              Use notes for research findings, sources, open questions and anything else about {personName(person)}.
            </EmptyState>
          ) : (
            <ul className="space-y-3">{list.map((n) => <NoteCard key={n.id} note={n} canEdit={canEdit} />)}</ul>
          )
        }
      </QueryBoundary>
    </div>
  );
}

function NoteCard({ note, canEdit }: { note: Note; canEdit: boolean }) {
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const del = useDeleteNote();
  const toast = useToast();
  if (editing) return <li><NoteEditor note={note} personId={note.personId} onDone={() => setEditing(false)} /></li>;
  return (
    <li className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
      <div className="flex items-start justify-between gap-2">
        {note.title ? <h3 className="font-semibold">{note.title}</h3> : <span />}
        {canEdit && (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => setEditing(true)}>Edit</Button>
            <Button size="sm" variant="ghost" className="text-red-700 dark:text-red-400" onClick={() => setConfirm(true)}>Delete</Button>
          </div>
        )}
      </div>
      {note.body && <div className="mt-1"><Markdown>{note.body}</Markdown></div>}
      <div className="mt-3"><Attribution value={note} /></div>
      <ConfirmDialog open={confirm} title="Delete this note?" onClose={() => setConfirm(false)} loading={del.isPending} error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try { await del.mutateAsync(note.id); toast("Note deleted"); setConfirm(false); } catch { /* inline */ }
        }}>
        The note will be removed. The deletion is kept in the change history.
      </ConfirmDialog>
    </li>
  );
}

function NoteEditor({ note, personId, onDone }: { note?: Note; personId: number; onDone: () => void }) {
  const [title, setTitle] = useState(note?.title ?? "");
  const [body, setBody] = useState(note?.body ?? "");
  const create = useCreateNote();
  const update = useUpdateNote();
  const toast = useToast();
  const m = note ? update : create;
  const fields = m.error instanceof ApiError ? m.error.fields : {};
  return (
    <form className="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-brand-200 dark:bg-stone-900 dark:ring-brand-800"
      onSubmit={async (e) => {
        e.preventDefault();
        try {
          if (note) await update.mutateAsync({ id: note.id, title, body });
          else await create.mutateAsync({ personId, title, body });
          toast(note ? "Note saved" : "Note added");
          onDone();
        } catch { /* inline */ }
      }}>
      <TextInput label="Title" value={title} onChange={(e) => setTitle(e.target.value)} error={fields.title} autoFocus />
      <MarkdownEditor label="Note" value={body} onChange={setBody} rows={6} error={fields.body} />
      <InlineError error={m.error} />
      <div className="flex justify-end gap-2">
        <Button onClick={onDone} disabled={m.isPending}>Cancel</Button>
        <Button type="submit" variant="primary" loading={m.isPending}>{note ? "Save note" : "Add note"}</Button>
      </div>
    </form>
  );
}
