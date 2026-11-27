import { useEffect, useId, useRef, useState } from "react";
import { usePersons } from "@/api/hooks";
import type { Person } from "@/api/types";
import { lifespan, personName } from "@/lib/format";
import { inputClass } from "./Field";
import { Avatar } from "./Avatar";
import { Spinner } from "./States";

/** "1890–1960", "b. 1948" or "Dates unknown" — shown next to names so namesakes are distinguishable. */
function years(p: Person): string {
  return lifespan(p) || "Dates unknown";
}

/**
 * Searchable person combobox.
 *
 * `clearable` (default) — "Change" clears the value via onChange(null), e.g. an optional filter.
 * `clearable={false}` — for required fields (like the tree's root person) "Change" opens the
 * search while keeping the current value; Escape or clicking away cancels.
 */
export function PersonPicker({
  label, value, onChange, exclude = [], placeholder = "Search people…", autoFocus, clearable = true,
}: {
  label: string; value: Person | null; onChange: (p: Person | null) => void; exclude?: number[]; placeholder?: string; autoFocus?: boolean;
  clearable?: boolean;
}) {
  const id = useId();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [debounced, setDebounced] = useState("");
  const [active, setActive] = useState(0);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 200);
    return () => clearTimeout(t);
  }, [q]);
  useEffect(() => {
    const onDoc = (e: PointerEvent) => {
      if (box.current?.contains(e.target as Node)) return;
      setOpen(false);
      setEditing(false);
    };
    // Capture phase: canvases such as React Flow stop propagation of pointer events,
    // which would otherwise hide "clicked outside" from a bubbling listener.
    document.addEventListener("pointerdown", onDoc, true);
    return () => document.removeEventListener("pointerdown", onDoc, true);
  }, []);
  const query = usePersons({ q: debounced, limit: 12 });
  const items = (query.data?.items ?? []).filter((p) => !exclude.includes(p.id));
  // Namesakes in the results also get their birthplace.
  const nameCounts = new Map<string, number>();
  for (const p of items) nameCounts.set(personName(p), (nameCounts.get(personName(p)) ?? 0) + 1);

  if (value && !editing) {
    return (
      <div className="space-y-1">
        <div className="text-sm font-medium text-stone-700 dark:text-stone-300">{label}</div>
        <div className="flex items-center gap-2 rounded-lg bg-white px-3 py-2 ring-1 ring-stone-300 dark:bg-stone-900 dark:ring-stone-700">
          <Avatar name={personName(value)} sex={value.sex} mediaId={value.portraitMediaId} size={28} />
          <span className="flex-1 truncate text-sm">
            {personName(value)} <span className="text-stone-500">{years(value)}</span>
          </span>
          <button type="button" aria-label={`Change ${label.toLowerCase()}`}
            className="rounded px-1 text-sm text-stone-500 hover:text-stone-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:hover:text-stone-200"
            onClick={() => {
              if (clearable) {
                onChange(null);
              } else {
                setEditing(true);
                setOpen(true);
              }
            }}>
            Change
          </button>
        </div>
      </div>
    );
  }
  const pick = (p: Person) => {
    onChange(p);
    setQ("");
    setOpen(false);
    setEditing(false);
  };
  const cancel = () => {
    setQ("");
    setOpen(false);
    setEditing(false);
  };
  return (
    <div className="relative space-y-1" ref={box}>
      <div className="flex items-center justify-between">
        <label htmlFor={id} className="block text-sm font-medium text-stone-700 dark:text-stone-300">{label}</label>
        {editing && (
          <button type="button" onClick={cancel} className="text-xs text-stone-500 hover:text-stone-800 dark:hover:text-stone-200">
            Cancel
          </button>
        )}
      </div>
      <input
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        autoFocus={autoFocus || editing}
        autoComplete="off"
        className={`${inputClass} ${editing ? "h-11" : ""}`} // match the selected-person chip so the toolbar doesn't jump
        placeholder={placeholder}
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
          setActive(0);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") setActive((a) => Math.min(a + 1, items.length - 1));
          else if (e.key === "ArrowUp") setActive((a) => Math.max(a - 1, 0));
          else if (e.key === "Enter" && open && items[active]) {
            e.preventDefault();
            pick(items[active]);
          } else if (e.key === "Escape") {
            if (editing) cancel();
            else setOpen(false);
          }
        }}
      />
      {open && (
        <ul id={`${id}-list`} role="listbox" className="absolute z-40 mt-1 max-h-64 w-full overflow-auto rounded-lg bg-white py-1 shadow-lg ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700">
          {query.isPending ? (
            <li className="flex items-center gap-2 px-3 py-2 text-sm text-stone-500"><Spinner className="h-4 w-4" /> Searching…</li>
          ) : query.isError ? (
            <li className="px-3 py-2 text-sm text-red-600">Search failed. Try again.</li>
          ) : items.length === 0 ? (
            <li className="px-3 py-2 text-sm text-stone-500">{debounced ? `No one matches “${debounced}”.` : "No people yet."}</li>
          ) : (
            items.map((p, i) => (
              <li
                key={p.id}
                role="option"
                aria-selected={i === active}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => pick(p)}
                onMouseEnter={() => setActive(i)}
                className={`flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm ${i === active ? "bg-brand-50 dark:bg-brand-900/40" : ""}`}
              >
                <Avatar name={personName(p)} sex={p.sex} mediaId={p.portraitMediaId} size={24} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{personName(p)}</span>
                  {(nameCounts.get(personName(p)) ?? 0) > 1 && p.birthPlace && (
                    <span className="block truncate text-xs text-stone-500">b. in {p.birthPlace}</span>
                  )}
                </span>
                <span className="shrink-0 text-xs tabular-nums text-stone-500">{years(p)}</span>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
