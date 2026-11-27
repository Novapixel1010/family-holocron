import { Link } from "react-router-dom";
import type { TimelineItem } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { Markdown } from "@/components/ui/Markdown";
import { formatDate } from "@/lib/dates";
import { EVENT_ICON, titleCase } from "@/lib/format";

const kindTone: Record<string, string> = {
  birth: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-200",
  death: "bg-stone-200 text-stone-700 dark:bg-stone-700 dark:text-stone-200",
  marriage: "bg-rose-100 text-rose-800 dark:bg-rose-900/50 dark:text-rose-200",
  historical: "bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200",
  family: "bg-violet-100 text-violet-800 dark:bg-violet-900/50 dark:text-violet-200",
};

function itemTitle(it: TimelineItem): string {
  if (it.title) return it.title;
  const names = it.people.map((p) => p.name).join(" & ");
  return names ? `${titleCase(it.type)} — ${names}` : titleCase(it.type);
}

function decadeOf(it: TimelineItem): string {
  if (!it.year) return "Undated";
  return `${Math.floor(it.year / 10) * 10}s`;
}

/** Vertical chronological list, grouped by decade. */
export function TimelineList({ items, onOpenEvent, hidePerson }: { items: TimelineItem[]; onOpenEvent?: (id: number) => void; hidePerson?: number }) {
  const groups: { label: string; items: TimelineItem[] }[] = [];
  for (const it of items) {
    const label = decadeOf(it);
    const last = groups[groups.length - 1];
    if (last && last.label === label) last.items.push(it);
    else groups.push({ label, items: [it] });
  }
  return (
    <div className="space-y-6">
      {groups.map((g) => (
        <section key={g.label + g.items[0].key} aria-label={g.label}>
          <h3 className="sticky top-0 z-[1] mb-2 bg-stone-50/90 py-1 font-serif text-lg font-semibold text-stone-700 backdrop-blur dark:bg-stone-950/90 dark:text-stone-300">{g.label}</h3>
          <ol className="relative ml-3 space-y-3 border-l-2 border-stone-200 pl-6 dark:border-stone-800">
            {g.items.map((it) => {
              const clickable = it.kind === "event" && it.eventId && onOpenEvent;
              const others = it.people.filter((p) => p.personId !== hidePerson);
              return (
                <li key={it.key} className="relative">
                  <span aria-hidden="true" className={`absolute -left-[37px] flex h-6 w-6 items-center justify-center rounded-full text-xs ring-4 ring-stone-50 dark:ring-stone-950 ${kindTone[it.type] ?? "bg-brand-100 text-brand-800 dark:bg-brand-900/60 dark:text-brand-200"}`}>
                    {EVENT_ICON[it.type] ?? "•"}
                  </span>
                  <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
                    <div className="flex flex-wrap items-baseline justify-between gap-2">
                      <div className="min-w-0">
                        <div className="text-xs font-medium uppercase tracking-wide text-stone-500">
                          <span className="normal-case tabular-nums">{formatDate(it.date) || "Date unknown"}</span> · {it.type === "family" ? "Family milestone" : it.type === "historical" ? "Historical" : titleCase(it.type)}
                        </div>
                        {clickable ? (
                          <button onClick={() => onOpenEvent!(it.eventId!)} className="text-left font-semibold hover:text-brand-700 hover:underline dark:hover:text-brand-300">{itemTitle(it)}</button>
                        ) : (
                          <div className="font-semibold">{itemTitle(it)}</div>
                        )}
                      </div>
                      {it.place && <div className="text-sm text-stone-500">⌖ {it.place}</div>}
                    </div>
                    {others.length > 0 && (
                      <div className="mt-1 flex flex-wrap gap-x-2 text-sm">
                        {others.map((p) => (
                          <Link key={p.personId} to={`/people/${p.personId}`} className="text-brand-700 hover:underline dark:text-brand-300">
                            {p.name}{p.role && p.role !== "principal" ? ` (${p.role})` : ""}
                          </Link>
                        ))}
                      </div>
                    )}
                    {it.description && <div className="mt-2 text-sm"><Markdown>{it.description}</Markdown></div>}
                    <div className="mt-2"><Attribution value={it} compact={false} /></div>
                  </div>
                </li>
              );
            })}
          </ol>
        </section>
      ))}
    </div>
  );
}
