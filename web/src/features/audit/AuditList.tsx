import { useState } from "react";
import { Link } from "react-router-dom";
import { useAudit } from "@/api/hooks";
import type { AuditEntry } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { dateTime, relativeTime, titleCase, userLabel } from "@/lib/format";

const verbs: Record<string, string> = {
  create: "added", update: "edited", delete: "deleted", import: "imported", restore: "restored", backup: "backed up", login: "signed in",
};

const entityNames: Record<string, string> = {
  person: "person", relationship: "relationship", event: "event", note: "note", media: "media item", user: "user account",
  system: "", setting: "setting",
};

function fmt(v: unknown): string {
  if (v === null || v === undefined || v === "") return "—";
  if (typeof v === "boolean") return v ? "yes" : "no";
  if (Array.isArray(v)) {
    if (v.length === 0) return "—";
    if (typeof v[0] === "object") return v.map((x) => (x as { name?: string }).name ?? JSON.stringify(x)).join(", ");
    return v.join(", ");
  }
  if (typeof v === "object") return JSON.stringify(v);
  const s = String(v);
  return s.length > 160 ? s.slice(0, 160) + "…" : s;
}

function EntryLink({ e }: { e: AuditEntry }) {
  const label = e.entityLabel || `#${e.entityId}`;
  if (e.entityType === "person" && e.action !== "delete") {
    return <Link to={`/people/${e.entityId}`} className="font-medium text-brand-700 hover:underline dark:text-brand-300">{label}</Link>;
  }
  return <span className="font-medium">{label}</span>;
}

function Entry({ e }: { e: AuditEntry }) {
  const [open, setOpen] = useState(false);
  const changes = Object.entries(e.changes ?? {});
  const who = e.user ? userLabel(e.user) : e.username || "System";
  return (
    <li className="py-3">
      <div className="flex flex-wrap items-baseline gap-x-1.5 text-sm">
        <span className="font-semibold">{who}</span>
        <span className="text-stone-600 dark:text-stone-400">{verbs[e.action] ?? e.action}</span>
        {entityNames[e.entityType] && <span className="text-stone-600 dark:text-stone-400">{entityNames[e.entityType]}</span>}
        <EntryLink e={e} />
        <span className="ml-auto text-xs text-stone-500" title={dateTime(e.createdAt)}>{relativeTime(e.createdAt)}</span>
      </div>
      {e.summary && <p className="mt-0.5 text-xs text-stone-500">{e.summary}</p>}
      {changes.length > 0 && (
        <button onClick={() => setOpen(!open)} className="mt-1 text-xs text-stone-500 underline-offset-2 hover:underline" aria-expanded={open}>
          {open ? "Hide" : "Show"} {changes.length} field{changes.length === 1 ? "" : "s"}
        </button>
      )}
      {open && (
        <dl className="mt-2 grid grid-cols-[auto,1fr] gap-x-3 gap-y-1 rounded-lg bg-stone-50 p-3 text-xs dark:bg-stone-800/60">
          {changes.map(([k, c]) => (
            <div key={k} className="contents">
              <dt className="font-medium text-stone-600 dark:text-stone-300">{titleCase(k.replace(/([A-Z])/g, " $1").toLowerCase())}</dt>
              <dd className="break-words">
                {e.action !== "create" && <span className="text-red-700 line-through decoration-red-300 dark:text-red-300">{fmt(c.from)}</span>}
                {e.action === "update" && " → "}
                {e.action !== "delete" && <span className="text-brand-800 dark:text-brand-200">{fmt(c.to)}</span>}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </li>
  );
}

/** Paginated change log. Filter by person, entity type or user. */
export function AuditList({ personId, entityType, userId, pageSize = 25, emptyText }: {
  personId?: number; entityType?: string; userId?: number; pageSize?: number; emptyText?: string;
}) {
  const [cursors, setCursors] = useState<number[]>([]);
  const before = cursors[cursors.length - 1];
  const q = useAudit({ personId, entityType, userId, before, limit: pageSize });
  if (q.isPending) return <LoadingState label="Loading history…" />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} title="Couldn't load the change history" />;
  const { items, nextBefore } = q.data;
  if (items.length === 0 && cursors.length === 0) {
    return <EmptyState icon="↺" title="No changes recorded yet">{emptyText ?? "Every edit made by anyone in the family will appear here."}</EmptyState>;
  }
  return (
    <div>
      <ul className={`divide-y divide-stone-200 dark:divide-stone-800 ${q.isFetching ? "opacity-60" : ""}`}>
        {items.map((e) => <Entry key={e.id} e={e} />)}
      </ul>
      {(nextBefore > 0 || cursors.length > 0) && (
        <div className="mt-3 flex justify-between">
          <Button size="sm" disabled={cursors.length === 0} onClick={() => setCursors(cursors.slice(0, -1))}>← Newer</Button>
          <Button size="sm" disabled={!nextBefore} onClick={() => setCursors([...cursors, nextBefore])}>Older →</Button>
        </div>
      )}
    </div>
  );
}
