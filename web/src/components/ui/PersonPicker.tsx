import { useEffect, useId, useRef, useState } from "react";
import { usePersons } from "@/api/hooks";
import type { Person } from "@/api/types";
import { lifespan, personName } from "@/lib/format";
import { inputClass } from "./Field";
import { Avatar } from "./Avatar";
import { Spinner } from "./States";

/** Searchable person combobox. */
export function PersonPicker({
  label, value, onChange, exclude = [], placeholder = "Search people…", autoFocus,
}: {
  label: string; value: Person | null; onChange: (p: Person | null) => void; exclude?: number[]; placeholder?: string; autoFocus?: boolean;
}) {
  const id = useId();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [debounced, setDebounced] = useState("");
  const [active, setActive] = useState(0);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 200);
    return () => clearTimeout(t);
  }, [q]);
  useEffect(() => {
    const onDoc = (e: MouseEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);
  const query = usePersons({ q: debounced, limit: 12 });
  const items = (query.data?.items ?? []).filter((p) => !exclude.includes(p.id));

  if (value) {
    return (
      <div className="space-y-1">
        <div className="text-sm font-medium text-stone-700 dark:text-stone-300">{label}</div>
        <div className="flex items-center gap-2 rounded-lg bg-white px-3 py-2 ring-1 ring-stone-300 dark:bg-stone-900 dark:ring-stone-700">
          <Avatar name={personName(value)} sex={value.sex} mediaId={value.portraitMediaId} size={28} />
          <span className="flex-1 truncate text-sm">
            {personName(value)} <span className="text-stone-500">{lifespan(value)}</span>
          </span>
          <button type="button" className="text-sm text-stone-500 hover:text-stone-800 dark:hover:text-stone-200" onClick={() => onChange(null)}>
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
  };
  return (
    <div className="relative space-y-1" ref={box}>
      <label htmlFor={id} className="block text-sm font-medium text-stone-700 dark:text-stone-300">{label}</label>
      <input
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        autoFocus={autoFocus}
        autoComplete="off"
        className={inputClass}
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
          } else if (e.key === "Escape") setOpen(false);
        }}
      />
      {open && (
        <ul id={`${id}-list`} role="listbox" className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-lg bg-white py-1 shadow-lg ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700">
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
                <span className="truncate">{personName(p)}</span>
                <span className="ml-auto text-xs text-stone-500">{lifespan(p)}</span>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
