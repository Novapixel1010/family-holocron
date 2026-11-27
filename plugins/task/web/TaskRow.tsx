import { Link } from "react-router-dom";
import { relativeTime, userLabel } from "@/lib/format";
import type { Task } from "./api";
import { ClaimControls } from "./ClaimControls";
import { PRIORITY, STATUS } from "./model";

export function StatusBadge({ status }: { status: Task["status"] }) {
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS[status].tone}`}>{STATUS[status].label}</span>;
}

export function PriorityLabel({ priority }: { priority: Task["priority"] }) {
  const p = PRIORITY[priority];
  return <span className={`text-xs font-medium ${p.tone}`} title={`${p.label} priority`}><span aria-hidden="true">{p.icon}</span> {p.label}</span>;
}

/** One task in a list. */
export function TaskRow({ task, onOpen, hidePerson }: { task: Task; onOpen: (t: Task) => void; hidePerson?: number }) {
  const people = task.people.filter((p) => p.id !== hidePerson);
  return (
    <li className="flex flex-col gap-2 rounded-xl bg-white p-4 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800 sm:flex-row sm:items-start">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge status={task.status} />
          <PriorityLabel priority={task.priority} />
          {task.tags.map((t) => <span key={t} className="rounded bg-stone-100 px-1.5 py-0.5 text-xs text-stone-600 dark:bg-stone-800 dark:text-stone-300">#{t}</span>)}
        </div>
        <button onClick={() => onOpen(task)} className={`mt-1 block text-left font-semibold hover:text-brand-700 hover:underline dark:hover:text-brand-300 ${task.status === "done" ? "text-stone-500 line-through decoration-stone-300" : ""}`}>
          {task.title}
        </button>
        <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-stone-500">
          {people.length > 0 && (
            <span>
              About{" "}
              {people.map((p, i) => (
                <span key={p.id}>{i > 0 && ", "}<Link to={`/people/${p.id}`} className="text-brand-700 hover:underline dark:text-brand-300">{p.name}</Link></span>
              ))}
            </span>
          )}
          {task.media.length > 0 && <span>▣ {task.media.length} source{task.media.length === 1 ? "" : "s"}</span>}
          {task.claimedBy ? (
            <span>Claimed by <span className="font-medium text-stone-700 dark:text-stone-300">{userLabel(task.claimedBy)}</span>{task.claimedAt && ` · ${relativeTime(task.claimedAt)}`}</span>
          ) : task.status !== "done" ? <span>Unclaimed</span> : null}
          <span>Updated {relativeTime(task.updatedAt)} by {userLabel(task.updatedBy)}</span>
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap gap-2"><ClaimControls task={task} /></div>
    </li>
  );
}
