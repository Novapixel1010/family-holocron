import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/Page";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { useTaskTags, useTasks, type TaskStatus } from "./api";
import { ACTIVE, PRIORITY, STATUS } from "./model";
import { TaskModal } from "./TaskModal";
import { TaskRow } from "./TaskRow";

const VIEWS: { id: string; label: string; status?: TaskStatus[] }[] = [
  { id: "active", label: "Active", status: ACTIVE },
  { id: "open", label: STATUS.open.label, status: ["open"] },
  { id: "in_progress", label: STATUS.in_progress.label, status: ["in_progress"] },
  { id: "blocked", label: STATUS.blocked.label, status: ["blocked"] },
  { id: "done", label: STATUS.done.label, status: ["done"] },
  { id: "all", label: "All" },
];

export function TasksPage() {
  const { canEdit } = useAuth();
  const [params, setParams] = useSearchParams();
  const view = VIEWS.find((v) => v.id === params.get("view")) ?? VIEWS[0];
  const priority = params.get("priority") ?? "";
  const tag = params.get("tag") ?? "";
  const mine = params.get("mine") === "1";
  const [text, setText] = useState(params.get("q") ?? "");
  const [q, setQ] = useState(text);
  const [modal, setModal] = useState<{ open: boolean; id?: number }>({ open: false });
  useEffect(() => {
    const t = setTimeout(() => setQ(text), 250);
    return () => clearTimeout(t);
  }, [text]);
  const set = (k: string, v: string) => setParams((p) => { const n = new URLSearchParams(p); v ? n.set(k, v) : n.delete(k); return n; }, { replace: true });
  const tasks = useTasks({ status: view.status, priority: priority || undefined, tag: tag || undefined, claimedBy: mine ? "me" : undefined, q: q || undefined });
  const tags = useTaskTags();
  const filtered = view.id !== "active" || !!priority || !!tag || mine || !!q;

  return (
    <PageContainer>
      <PageHeader title="Research tasks" subtitle="A shared to-do list for the family’s research. Claim a task so others know you’re on it."
        actions={canEdit && <Button variant="primary" onClick={() => setModal({ open: true })}>+ New task</Button>} />
      <div className="mb-4 space-y-3">
        <div role="tablist" aria-label="Task status" className="flex flex-wrap gap-1">
          {VIEWS.map((v) => (
            <button key={v.id} role="tab" aria-selected={view.id === v.id} onClick={() => set("view", v.id === "active" ? "" : v.id)}
              className={`rounded-full px-3 py-1 text-sm font-medium ring-1 ${view.id === v.id ? "bg-brand-600 text-white ring-brand-600" : "bg-white text-stone-700 ring-stone-300 hover:bg-stone-50 dark:bg-stone-800 dark:text-stone-200 dark:ring-stone-700"}`}>
              {v.label}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <input type="search" aria-label="Search tasks" placeholder="Search tasks…" className={`${inputClass} max-w-xs`} value={text} onChange={(e) => setText(e.target.value)} />
          <select aria-label="Priority" className={`${inputClass} !w-auto`} value={priority} onChange={(e) => set("priority", e.target.value)}>
            <option value="">Any priority</option>
            {(["high", "medium", "low"] as const).map((p) => <option key={p} value={p}>{PRIORITY[p].label}</option>)}
          </select>
          <select aria-label="Tag" className={`${inputClass} !w-auto`} value={tag} onChange={(e) => set("tag", e.target.value)} disabled={!tags.data?.length}>
            <option value="">{tags.isError ? "Tags unavailable" : "Any tag"}</option>
            {tags.data?.map((t) => <option key={t.tag} value={t.tag}>#{t.tag} ({t.count})</option>)}
          </select>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" className="rounded" checked={mine} onChange={(e) => set("mine", e.target.checked ? "1" : "")} /> Claimed by me
          </label>
        </div>
      </div>
      {tasks.isPending ? (
        <LoadingState label="Loading tasks…" />
      ) : tasks.isError ? (
        <ErrorState error={tasks.error} onRetry={() => tasks.refetch()} title="Couldn't load tasks" />
      ) : tasks.data.length === 0 ? (
        filtered ? (
          <EmptyState icon="⌕" title="No tasks match" action={<Button onClick={() => { setText(""); setParams({}, { replace: true }); }}>Clear filters</Button>}>
            Try another status or remove a filter.
          </EmptyState>
        ) : (
          <EmptyState icon="☑" title="No research tasks yet" action={canEdit && <Button variant="primary" onClick={() => setModal({ open: true })}>Create the first task</Button>}>
            Track what still needs finding — a missing record, a photo to identify, a relative to ask — and who is working on it.
          </EmptyState>
        )
      ) : (
        <ul className={`space-y-3 ${tasks.isFetching ? "opacity-70" : ""}`}>
          {tasks.data.map((t) => <TaskRow key={t.id} task={t} onOpen={(x) => setModal({ open: true, id: x.id })} />)}
        </ul>
      )}
      <TaskModal open={modal.open} taskId={modal.id} onClose={() => setModal({ open: false })} />
    </PageContainer>
  );
}
