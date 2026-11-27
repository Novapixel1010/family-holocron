import type { Task, TaskPriority, TaskStatus } from "./api";

export const STATUS: Record<TaskStatus, { label: string; tone: string }> = {
  open: { label: "Open", tone: "bg-sky-100 text-sky-800 dark:bg-sky-900/50 dark:text-sky-200" },
  in_progress: { label: "In progress", tone: "bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200" },
  blocked: { label: "Blocked", tone: "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-200" },
  done: { label: "Done", tone: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-200" },
};

export const PRIORITY: Record<TaskPriority, { label: string; tone: string; icon: string }> = {
  high: { label: "High", tone: "text-red-700 dark:text-red-400", icon: "▲" },
  medium: { label: "Medium", tone: "text-amber-700 dark:text-amber-400", icon: "■" },
  low: { label: "Low", tone: "text-stone-500", icon: "▼" },
};

export const ACTIVE: TaskStatus[] = ["open", "in_progress", "blocked"];

export type ClaimAction = "claim" | "release" | "takeOver";

/** Which claim buttons a user sees for a task (mirrors the server's rules). */
export function claimActions(task: Pick<Task, "status" | "claimedBy">, me: { id: number; role: string }): ClaimAction[] {
  if (me.role === "viewer" || task.status === "done") return [];
  if (!task.claimedBy) return ["claim"];
  if (task.claimedBy.id === me.id) return ["release"];
  return me.role === "admin" ? ["takeOver", "release"] : ["takeOver"];
}

/** Splits free-typed tag text ("census, uk") into clean, de-duplicated tags. */
export function addTags(existing: string[], text: string): string[] {
  const out = [...existing];
  for (const raw of text.split(",")) {
    const t = raw.trim().replace(/\s+/g, " ");
    if (t && !out.some((x) => x.toLowerCase() === t.toLowerCase())) out.push(t.slice(0, 40));
  }
  return out.slice(0, 20);
}
