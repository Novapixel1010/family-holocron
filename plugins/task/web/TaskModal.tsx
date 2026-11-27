import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/api/client";
import { useMediaList } from "@/api/hooks";
import type { Media, Person } from "@/api/types";
import { useAuth } from "@/app/auth";
import { Attribution } from "@/components/ui/Attribution";
import { Button } from "@/components/ui/Button";
import { inputClass, Select, TextInput } from "@/components/ui/Field";
import { Markdown, MarkdownEditor } from "@/components/ui/Markdown";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { EmptyState, ErrorState, InlineError, LoadingState } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { MediaThumb, UploadButton } from "@/features/media/MediaGrid";
import { lifespan, relativeTime, userLabel } from "@/lib/format";
import { toInput, useCreateTask, useDeleteTask, useTask, useTaskTags, useUpdateTask, type Task, type TaskInput } from "./api";
import { ClaimControls } from "./ClaimControls";
import { CommentsThread } from "./CommentsThread";
import { addTags, PRIORITY, STATUS } from "./model";
import { PriorityLabel, StatusBadge } from "./TaskRow";

type Ref<T> = T & { label: string };

/** Create (taskId undefined) or view/edit a task. */
export function TaskModal({ open, onClose, taskId, preset }: {
  open: boolean; onClose: () => void; taskId?: number; preset?: { people?: { id: number; name: string }[] };
}) {
  const existing = useTask(open ? taskId : undefined);
  if (!open) return null;
  if (taskId && existing.isPending) return <Modal open onClose={onClose} title="Task"><LoadingState /></Modal>;
  if (taskId && existing.isError) return <Modal open onClose={onClose} title="Task"><ErrorState error={existing.error} onRetry={() => existing.refetch()} /></Modal>;
  return <TaskDialog key={taskId ?? "new"} task={existing.data} preset={preset} onClose={onClose} />;
}

function TaskDialog({ task, preset, onClose }: { task?: Task; preset?: { people?: { id: number; name: string }[] }; onClose: () => void }) {
  const { canEdit } = useAuth();
  const [editing, setEditing] = useState(!task);
  if (task && !editing) return <TaskView task={task} canEdit={canEdit} onEdit={() => setEditing(true)} onClose={onClose} />;
  return <TaskForm task={task} preset={preset} onClose={onClose} onCancel={task ? () => setEditing(false) : onClose} />;
}

function TaskView({ task, canEdit, onEdit, onClose }: { task: Task; canEdit: boolean; onEdit: () => void; onClose: () => void }) {
  const actions = canEdit || task.status !== "done";
  return (
    <Modal open onClose={onClose} title={task.title} size="lg"
      footer={actions && <>{canEdit && <Button onClick={onEdit}>Edit</Button>}<ClaimControls task={task} size="md" /></>}>
      <div className="space-y-4 text-sm">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge status={task.status} />
          <PriorityLabel priority={task.priority} />
          {task.tags.map((t) => <span key={t} className="rounded bg-stone-100 px-1.5 py-0.5 text-xs dark:bg-stone-800">#{t}</span>)}
        </div>
        <p className="text-stone-600 dark:text-stone-400">
          {task.claimedBy ? <>Claimed by <span className="font-medium">{userLabel(task.claimedBy)}</span>{task.claimedAt && ` ${relativeTime(task.claimedAt)}`}.</> : task.status === "done" ? "Completed." : "Nobody has claimed this task yet."}
        </p>
        {task.descriptionMd.trim() ? <Markdown>{task.descriptionMd}</Markdown> : <p className="text-stone-500">No description.</p>}
        <LinkedLists task={task} onNavigate={onClose} />
        <Attribution value={task} />
        <CommentsThread taskId={task.id} />
      </div>
    </Modal>
  );
}

function LinkedLists({ task, onNavigate }: { task: Task; onNavigate: () => void }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div>
        <h3 className="mb-1 font-semibold">People</h3>
        {task.people.length === 0 ? <p className="text-stone-500">None linked.</p> : (
          <ul className="space-y-0.5">{task.people.map((p) => (
            <li key={p.id}><Link to={`/people/${p.id}`} onClick={onNavigate} className="text-brand-700 hover:underline dark:text-brand-300">{p.name}</Link> <span className="text-xs text-stone-500">{lifespan(p)}</span></li>
          ))}</ul>
        )}
      </div>
      <div>
        <h3 className="mb-1 font-semibold">Sources & media</h3>
        {task.media.length === 0 ? <p className="text-stone-500">None attached.</p> : (
          <ul className="space-y-0.5">{task.media.map((m) => (
            <li key={m.id}><a href={`/api/v1/media/${m.id}/file`} target="_blank" rel="noopener" className="text-brand-700 hover:underline dark:text-brand-300">{m.title || m.originalName}</a></li>
          ))}</ul>
        )}
      </div>
    </div>
  );
}

function TaskForm({ task, preset, onClose, onCancel }: { task?: Task; preset?: { people?: { id: number; name: string }[] }; onClose: () => void; onCancel: () => void }) {
  const [form, setForm] = useState<TaskInput>(task ? toInput(task) : {
    title: "", descriptionMd: "", status: "open", priority: "medium", tags: [], personIds: preset?.people?.map((p) => p.id) ?? [], mediaIds: [],
  });
  const [people, setPeople] = useState<Ref<{ id: number }>[]>((task?.people ?? preset?.people ?? []).map((p) => ({ id: p.id, label: p.name })));
  const [media, setMedia] = useState<Ref<{ id: number; mimeType: string }>[]>((task?.media ?? []).map((m) => ({ id: m.id, label: m.title || m.originalName, mimeType: m.mimeType })));
  const [tagText, setTagText] = useState("");
  const [picking, setPicking] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const tags = useTaskTags();
  const create = useCreateTask();
  const update = useUpdateTask();
  const del = useDeleteTask();
  const toast = useToast();
  const m = task ? update : create;
  const fields = m.error instanceof ApiError ? m.error.fields : {};

  const addPerson = (p: Person | null) => {
    if (!p || form.personIds.includes(p.id)) return;
    setForm((f) => ({ ...f, personIds: [...f.personIds, p.id] }));
    setPeople((ps) => [...ps, { id: p.id, label: [p.givenName, p.surname].filter(Boolean).join(" ") || "Unnamed person" }]);
  };
  const addMedia = (items: Media[]) => {
    const fresh = items.filter((x) => !form.mediaIds.includes(x.id));
    setForm((f) => ({ ...f, mediaIds: [...f.mediaIds, ...fresh.map((x) => x.id)] }));
    setMedia((ms) => [...ms, ...fresh.map((x) => ({ id: x.id, label: x.title || x.originalName, mimeType: x.mimeType }))]);
  };
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const input = { ...form, tags: tagText.trim() ? addTags(form.tags, tagText) : form.tags };
    try {
      if (task) await update.mutateAsync({ id: task.id, input });
      else await create.mutateAsync(input);
      toast(task ? "Task saved" : "Task created");
      onClose();
    } catch { /* shown inline */ }
  };

  return (
    <Modal open onClose={onClose} title={task ? "Edit task" : "New research task"} size="lg">
      <form onSubmit={submit} className="space-y-4">
        <TextInput label="Title" value={form.title} autoFocus required onChange={(e) => setForm({ ...form, title: e.target.value })} error={fields.title}
          placeholder="e.g. Find Walter Hale in the 1911 census" />
        <div className="grid gap-4 sm:grid-cols-2">
          <Select label="Status" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as TaskInput["status"] })} error={fields.status}
            options={(Object.keys(STATUS) as TaskInput["status"][]).map((s) => ({ value: s, label: STATUS[s].label }))}
            hint={form.status === "open" && task?.claimedBy ? "Setting a task to Open releases its claim." : form.status === "in_progress" && !task?.claimedBy ? "Starting a task claims it for you." : undefined} />
          <Select label="Priority" value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value as TaskInput["priority"] })} error={fields.priority}
            options={(Object.keys(PRIORITY) as TaskInput["priority"][]).reverse().map((p) => ({ value: p, label: PRIORITY[p].label }))} />
        </div>
        <MarkdownEditor label="Description" value={form.descriptionMd} onChange={(v) => setForm({ ...form, descriptionMd: v })} rows={6} error={fields.descriptionMd}
          placeholder="What needs finding, where you've already looked, leads to follow…" />
        <div className="space-y-1">
          <label htmlFor="task-tags" className="block text-sm font-medium text-stone-700 dark:text-stone-300">Tags</label>
          <div className="flex flex-wrap gap-1.5">
            {form.tags.map((t) => (
              <span key={t} className="inline-flex items-center gap-1 rounded-full bg-stone-100 px-2 py-0.5 text-xs dark:bg-stone-800">
                #{t}<button type="button" aria-label={`Remove tag ${t}`} onClick={() => setForm({ ...form, tags: form.tags.filter((x) => x !== t) })}>✕</button>
              </span>
            ))}
          </div>
          <input id="task-tags" list="task-tag-options" className={inputClass} value={tagText} placeholder="Type a tag and press Enter or comma"
            onChange={(e) => {
              if (e.target.value.includes(",")) { setForm({ ...form, tags: addTags(form.tags, e.target.value) }); setTagText(""); }
              else setTagText(e.target.value);
            }}
            onKeyDown={(e) => { if (e.key === "Enter" && tagText.trim()) { e.preventDefault(); setForm({ ...form, tags: addTags(form.tags, tagText) }); setTagText(""); } }} />
          <datalist id="task-tag-options">{tags.data?.map((t) => <option key={t.tag} value={t.tag} />)}</datalist>
          {fields.tags && <p className="text-xs text-red-600">{fields.tags}</p>}
        </div>
        <div className="space-y-2">
          <div className="text-sm font-medium text-stone-700 dark:text-stone-300">People</div>
          <div className="flex flex-wrap gap-1.5">
            {people.length === 0 && <span className="text-sm text-stone-500">No one linked yet.</span>}
            {people.map((p) => (
              <span key={p.id} className="inline-flex items-center gap-1 rounded-full bg-brand-100 px-2 py-0.5 text-xs text-brand-800 dark:bg-brand-900/50 dark:text-brand-200">
                {p.label}<button type="button" aria-label={`Unlink ${p.label}`} onClick={() => { setForm({ ...form, personIds: form.personIds.filter((x) => x !== p.id) }); setPeople(people.filter((x) => x.id !== p.id)); }}>✕</button>
              </span>
            ))}
          </div>
          <PersonPicker label="Link a person" value={null} exclude={form.personIds} onChange={addPerson} />
          {fields.personIds && <p className="text-xs text-red-600">{fields.personIds}</p>}
        </div>
        <div className="space-y-2">
          <div className="text-sm font-medium text-stone-700 dark:text-stone-300">Sources & media</div>
          {media.length > 0 && (
            <ul className="grid grid-cols-3 gap-2 sm:grid-cols-5">
              {media.map((x) => (
                <li key={x.id} className="relative overflow-hidden rounded-lg ring-1 ring-stone-200 dark:ring-stone-700">
                  <div className="aspect-square"><MediaThumb m={{ id: x.id, mimeType: x.mimeType, title: x.label, originalName: x.label } as Media} /></div>
                  <div className="truncate px-1 py-0.5 text-[11px]">{x.label}</div>
                  <button type="button" aria-label={`Detach ${x.label}`} onClick={() => { setForm({ ...form, mediaIds: form.mediaIds.filter((y) => y !== x.id) }); setMedia(media.filter((y) => y.id !== x.id)); }}
                    className="absolute right-1 top-1 rounded-full bg-white/90 px-1.5 text-xs shadow dark:bg-stone-900/90">✕</button>
                </li>
              ))}
            </ul>
          )}
          <div className="flex flex-wrap gap-2">
            <Button size="sm" onClick={() => setPicking(true)}>Attach from library…</Button>
            <UploadButton label="Upload new source" personIds={form.personIds} onUploaded={addMedia} />
          </div>
          {fields.mediaIds && <p className="text-xs text-red-600">{fields.mediaIds}</p>}
        </div>
        <InlineError error={m.error} />
        <div className="flex flex-wrap justify-between gap-2">
          {task ? <Button variant="danger" onClick={() => setConfirmDelete(true)}>Delete task</Button> : <span />}
          <div className="flex gap-2">
            <Button onClick={onCancel} disabled={m.isPending}>Cancel</Button>
            <Button type="submit" variant="primary" loading={m.isPending}>{task ? "Save task" : "Create task"}</Button>
          </div>
        </div>
      </form>
      {picking && <MediaPicker selected={form.mediaIds} onClose={() => setPicking(false)} onPick={(items) => { addMedia(items); setPicking(false); }} />}
      <ConfirmDialog open={confirmDelete} title="Delete this task?" onClose={() => setConfirmDelete(false)} loading={del.isPending} error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try { await del.mutateAsync(task!.id); toast("Task deleted"); onClose(); } catch { /* inline */ }
        }}>
        The task is removed; linked people and attached files are not affected. The deletion is kept in the change history.
      </ConfirmDialog>
    </Modal>
  );
}

/** Pick existing media items to attach. */
function MediaPicker({ selected, onPick, onClose }: { selected: number[]; onPick: (m: Media[]) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [chosen, setChosen] = useState<Media[]>([]);
  const list = useMediaList({ q, limit: 60 });
  const toggle = (m: Media) => setChosen((c) => (c.some((x) => x.id === m.id) ? c.filter((x) => x.id !== m.id) : [...c, m]));
  return (
    <Modal open onClose={onClose} title="Attach from media library" size="lg"
      footer={<><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={chosen.length === 0} onClick={() => onPick(chosen)}>Attach {chosen.length || ""}</Button></>}>
      <input type="search" aria-label="Search media" className={`${inputClass} mb-3`} placeholder="Search titles and file names…" value={q} onChange={(e) => setQ(e.target.value)} />
      {list.isPending ? <LoadingState /> : list.isError ? <ErrorState error={list.error} onRetry={() => list.refetch()} /> : list.data.items.length === 0 ? (
        <EmptyState icon="▣" title={q ? "No matches" : "The media library is empty"}>{q ? `Nothing matches “${q}”.` : "Use “Upload new source” to add a file."}</EmptyState>
      ) : (
        <ul className="grid grid-cols-3 gap-2 sm:grid-cols-4">
          {list.data.items.map((m) => {
            const already = selected.includes(m.id);
            const on = chosen.some((x) => x.id === m.id);
            return (
              <li key={m.id}>
                <button type="button" disabled={already} aria-pressed={on} onClick={() => toggle(m)}
                  className={`block w-full overflow-hidden rounded-lg text-left ring-2 disabled:opacity-40 ${on ? "ring-brand-600" : "ring-transparent hover:ring-stone-300"}`}>
                  <div className="aspect-square"><MediaThumb m={m} /></div>
                  <div className="truncate p-1 text-xs">{already ? "Attached · " : ""}{m.title || m.originalName}</div>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </Modal>
  );
}
