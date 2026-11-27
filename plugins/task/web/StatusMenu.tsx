import { useEffect, useId, useRef, useState, type KeyboardEvent, type SyntheticEvent } from "react";
import { useAuth } from "@/app/auth";
import { errorMessage } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { QUICK_STATUSES, useSetTaskStatus, type Task, type TaskStatus } from "./api";
import { STATUS } from "./model";

const stop = (e: SyntheticEvent) => e.stopPropagation();

/** Static status pill. */
export function StatusBadge({ status }: { status: TaskStatus }) {
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS[status].tone}`}>{STATUS[status].label}</span>;
}

/**
 * Status pill on a task card. For a claimed task (and an editor) it opens a small menu to
 * switch between In progress / Blocked / Done without opening the task. Unclaimed tasks
 * keep a static pill: someone has to claim a task before its status can change.
 */
export function StatusPill({ task }: { task: Task }) {
  const { user, canEdit } = useAuth();
  if (!canEdit || !task.claimedBy) return <StatusBadge status={task.status} />;
  return <StatusMenu task={task} me={{ id: user.id, username: user.username, displayName: user.displayName }} />;
}

function StatusMenu({ task, me }: { task: Task; me: { id: number; username: string; displayName: string } }) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const box = useRef<HTMLSpanElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const items = useRef<(HTMLButtonElement | null)[]>([]);
  const setStatus = useSetTaskStatus();
  const toast = useToast();
  const id = useId();

  useEffect(() => {
    if (!open) return;
    items.current[active]?.focus();
  }, [open, active]);
  useEffect(() => {
    if (!open) return;
    // Capture phase so canvases/handlers that stop propagation can't hide outside clicks.
    const onDown = (e: PointerEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("pointerdown", onDown, true);
    return () => document.removeEventListener("pointerdown", onDown, true);
  }, [open]);

  const openMenu = () => {
    setActive(Math.max(0, QUICK_STATUSES.indexOf(task.status)));
    setOpen(true);
  };
  const close = (focusButton = true) => {
    setOpen(false);
    if (focusButton) button.current?.focus();
  };
  const choose = (status: TaskStatus) => {
    close();
    if (status === task.status) return;
    setStatus.mutate(
      { id: task.id, status, me },
      { onError: (e) => toast(`Couldn't change the status: ${errorMessage(e)}`, "error") },
    );
  };
  const onMenuKey = (e: KeyboardEvent) => {
    e.stopPropagation();
    const n = QUICK_STATUSES.length;
    if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => (a + 1) % n); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => (a - 1 + n) % n); }
    else if (e.key === "Home") { e.preventDefault(); setActive(0); }
    else if (e.key === "End") { e.preventDefault(); setActive(n - 1); }
    else if (e.key === "Escape") { e.preventDefault(); close(); }
    else if (e.key === "Tab") close(false);
  };

  return (
    <span ref={box} className="relative inline-flex" onClick={stop} onPointerDown={stop} onMouseDown={stop}>
      <button
        ref={button}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        aria-label={`Status: ${STATUS[task.status].label}. Change status`}
        onClick={(e) => { e.stopPropagation(); open ? close() : openMenu(); }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); openMenu(); }
        }}
        className={`inline-flex items-center gap-1 rounded-full py-0.5 pl-2 pr-1.5 text-xs font-medium ring-1 ring-inset ring-black/5 transition hover:brightness-95 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:ring-white/10 dark:hover:brightness-125 ${STATUS[task.status].tone}`}
      >
        {STATUS[task.status].label}
        <span aria-hidden="true" className="text-[10px] opacity-70">▾</span>
      </button>
      {open && (
        <span id={id} role="menu" aria-label="Change status" onKeyDown={onMenuKey}
          className="absolute left-0 top-full z-30 mt-1 w-40 rounded-lg bg-white py-1 shadow-lg ring-1 ring-stone-200 dark:bg-stone-800 dark:ring-stone-700">
          {QUICK_STATUSES.map((s, i) => (
            <button key={s} ref={(el) => { items.current[i] = el; }} type="button" role="menuitemradio" aria-checked={task.status === s}
              tabIndex={i === active ? 0 : -1}
              onClick={(e) => { e.stopPropagation(); choose(s); }}
              onMouseEnter={() => setActive(i)}
              className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm focus:outline-none ${i === active ? "bg-stone-100 dark:bg-stone-700" : ""}`}>
              <span className={`h-2 w-2 rounded-full ${STATUS[s].dot}`} aria-hidden="true" />
              <span className="flex-1">{STATUS[s].label}</span>
              {task.status === s && <span aria-hidden="true" className="text-brand-600 dark:text-brand-300">✓</span>}
            </button>
          ))}
        </span>
      )}
    </span>
  );
}
