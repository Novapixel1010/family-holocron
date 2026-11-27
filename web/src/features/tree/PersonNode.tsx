import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Avatar } from "@/components/ui/Avatar";
import { lifespan } from "@/lib/format";
import { PERSON_H, PERSON_W, type PersonFlowNode } from "./graph";
import { useTreeActions } from "./TreeContext";

const sexBar: Record<string, string> = { M: "bg-sky-600", F: "bg-rose-600", X: "bg-violet-600", U: "bg-stone-400" };
const hiddenHandle = "!h-px !w-px !min-h-0 !min-w-0 !border-0 !bg-transparent !opacity-0";

/** Person card rendered inside React Flow. Positioned by Dagre; not draggable. */
export const PersonNode = memo(function PersonNode({ data }: NodeProps<PersonFlowNode>) {
  const { direction, selectedId, loadingId, collapsed, onToggle, onFocus } = useTreeActions();
  const p = data.person;
  const years = lifespan({ birthDate: p.birthDate, deathDate: p.deathDate, deceased: !p.living });
  const selected = selectedId === p.id;
  const isCollapsed = collapsed.has(p.id);
  const canToggle = data.outward > 0 || p.hasMore;
  const loading = loadingId === p.id;
  const toggleLabel = p.hasMore && !isCollapsed ? `Load more generations for ${p.name}` : isCollapsed ? `Expand ${p.name}` : `Collapse ${p.name}`;
  // The toggle sits on the edge that faces the next generation.
  const togglePos = direction === "ancestors" ? "-top-3" : "-bottom-3";

  return (
    <div style={{ width: PERSON_W, height: PERSON_H }}
      className={`group relative flex overflow-visible rounded-xl shadow-sm transition ${
        selected ? "bg-brand-50 ring-2 ring-brand-600 dark:bg-brand-900" : "bg-white ring-1 ring-stone-300 hover:ring-brand-400 dark:bg-stone-900 dark:ring-stone-700"
      }`}>
      <Handle type="target" position={Position.Top} id="top" isConnectable={false} className={hiddenHandle} />
      <Handle type="source" position={Position.Bottom} id="bottom" isConnectable={false} className={hiddenHandle} />
      <span aria-hidden="true" className={`my-3 ml-1.5 w-1 shrink-0 rounded ${sexBar[p.sex] ?? sexBar.U}`} />
      <button type="button" aria-label={`${p.name}${years ? `, ${years}` : ""}`} aria-pressed={selected}
        className="nodrag nopan flex min-w-0 flex-1 cursor-pointer items-center gap-2.5 rounded-xl py-2 pl-2 pr-8 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500">
        <Avatar name={p.name} sex={p.sex} mediaId={p.portraitMediaId} size={40} />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-semibold text-stone-900 dark:text-stone-100">{p.name}</span>
          <span className="block truncate text-xs text-stone-500 dark:text-stone-400">{years || (p.living ? "Living" : "Dates unknown")}</span>
          <span className="mt-1 flex gap-1">
            {data.isRoot && <span className="rounded-full bg-brand-100 px-1.5 py-px text-[10px] font-medium text-brand-800 dark:bg-brand-900/60 dark:text-brand-200">Root</span>}
            {p.role === "partner" && <span className="rounded-full bg-stone-100 px-1.5 py-px text-[10px] font-medium text-stone-600 dark:bg-stone-800 dark:text-stone-300">Partner</span>}
            {p.hasAbout && <span title="Has an About biography" className="rounded-full bg-amber-100 px-1.5 py-px text-[10px] font-medium text-amber-800 dark:bg-amber-900/50 dark:text-amber-200">✎ About</span>}
          </span>
        </span>
      </button>
      {!data.isRoot && (
        <button type="button" onClick={(e) => { e.stopPropagation(); onFocus(p); }} aria-label={`Center tree on ${p.name}`} title="Center tree here"
          className="nodrag nopan absolute right-1.5 top-1.5 rounded-md px-1.5 py-0.5 text-sm text-stone-400 opacity-70 hover:bg-stone-100 hover:text-brand-700 group-hover:opacity-100 focus-visible:opacity-100 dark:hover:bg-stone-800">
          ⌖
        </button>
      )}
      {canToggle && (
        <button type="button" onClick={(e) => { e.stopPropagation(); if (!loading) onToggle(p); }} aria-label={toggleLabel} title={toggleLabel} aria-busy={loading || undefined}
          className={`nodrag nopan absolute left-1/2 ${togglePos} flex h-6 w-6 -translate-x-1/2 items-center justify-center rounded-full bg-white text-sm font-bold text-stone-600 ring-1 ring-stone-400 hover:ring-brand-500 dark:bg-stone-800 dark:text-stone-200 dark:ring-stone-500`}>
          {loading ? (
            <span className="h-3 w-3 animate-spin rounded-full border-2 border-brand-600 border-t-transparent" />
          ) : (p.hasMore && !isCollapsed) || isCollapsed ? "+" : "−"}
        </button>
      )}
    </div>
  );
});
