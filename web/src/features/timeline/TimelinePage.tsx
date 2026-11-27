import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useEventTypes, usePerson, useTimeline } from "@/api/hooks";
import type { Person } from "@/api/types";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/Page";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { personName, titleCase } from "@/lib/format";
import { EventFormModal } from "./EventFormModal";
import { TimelineList } from "./TimelineList";

export function TimelinePage() {
  const { canEdit } = useAuth();
  const [params, setParams] = useSearchParams();
  const personId = Number(params.get("person")) || undefined;
  const types = (params.get("types") ?? "").split(",").filter(Boolean);
  const q = params.get("q") ?? "";
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const [text, setText] = useState(q);
  const [range, setRange] = useState({ from, to });
  const [modal, setModal] = useState<{ open: boolean; id?: number }>({ open: false });
  const selected = usePerson(personId);
  const eventTypes = useEventTypes();
  const tl = useTimeline({ personId, types, q, from: /^\d{3,4}$/.test(from) ? from : undefined, to: /^\d{3,4}$/.test(to) ? to : undefined });

  const update = (patch: Record<string, string | undefined>) =>
    setParams((p) => {
      const n = new URLSearchParams(p);
      for (const [k, v] of Object.entries(patch)) v ? n.set(k, v) : n.delete(k);
      return n;
    }, { replace: true });

  useEffect(() => {
    const t = setTimeout(() => text !== q && update({ q: text }), 300);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  const allTypes = ["birth", "death", ...(eventTypes.data?.eventTypes ?? [])];
  const toggleType = (t: string) => update({ types: (types.includes(t) ? types.filter((x) => x !== t) : [...types, t]).join(",") });
  const filtered = !!(personId || types.length || q || from || to);
  const yearInvalid = (from && !/^\d{3,4}$/.test(from)) || (to && !/^\d{3,4}$/.test(to));

  return (
    <PageContainer>
      <PageHeader
        title="Timeline"
        subtitle="Births, marriages, moves and milestones in the order they happened."
        actions={canEdit && <Button variant="primary" onClick={() => setModal({ open: true })}>+ Add event</Button>}
      />
      <div className="mb-6 space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
        <div className="grid gap-3 md:grid-cols-3">
          {personId && selected.isPending ? (
            <LoadingState className="py-2" label="Loading person…" />
          ) : (
            <PersonPicker label="Person" value={selected.data ?? null} onChange={(p: Person | null) => update({ person: p ? String(p.id) : undefined })} placeholder="Everyone" />
          )}
          <div className="space-y-1">
            <label htmlFor="tl-q" className="block text-sm font-medium text-stone-700 dark:text-stone-300">Search</label>
            <input id="tl-q" type="search" className={inputClass} placeholder="Title, place, description…" value={text} onChange={(e) => setText(e.target.value)} />
          </div>
          <div className="space-y-1">
            <span className="block text-sm font-medium text-stone-700 dark:text-stone-300">Years</span>
            <div className="flex items-center gap-2">
              <input aria-label="From year" inputMode="numeric" className={inputClass} placeholder="From" value={range.from}
                onChange={(e) => setRange({ ...range, from: e.target.value })} onBlur={() => update({ from: range.from })} onKeyDown={(e) => e.key === "Enter" && update({ from: range.from })} />
              <span className="text-stone-400">–</span>
              <input aria-label="To year" inputMode="numeric" className={inputClass} placeholder="To" value={range.to}
                onChange={(e) => setRange({ ...range, to: e.target.value })} onBlur={() => update({ to: range.to })} onKeyDown={(e) => e.key === "Enter" && update({ to: range.to })} />
            </div>
            {yearInvalid && <p className="text-xs text-red-600">Enter a 3–4 digit year.</p>}
          </div>
        </div>
        <div>
          <div className="mb-1 text-sm font-medium text-stone-700 dark:text-stone-300">Event types</div>
          {eventTypes.isError ? (
            <p className="text-sm text-red-600">Couldn’t load event types. <button className="underline" onClick={() => eventTypes.refetch()}>Retry</button></p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {allTypes.map((t) => (
                <button key={t} onClick={() => toggleType(t)} aria-pressed={types.includes(t)}
                  className={`rounded-full px-2.5 py-1 text-xs font-medium ring-1 transition ${types.includes(t) ? "bg-brand-600 text-white ring-brand-600" : "bg-white text-stone-700 ring-stone-300 hover:bg-stone-50 dark:bg-stone-800 dark:text-stone-200 dark:ring-stone-700"}`}>
                  {t === "family" ? "Family milestone" : t === "historical" ? "Historical" : titleCase(t)}
                </button>
              ))}
            </div>
          )}
        </div>
        {filtered && (
          <Button size="sm" variant="ghost" onClick={() => { setText(""); setRange({ from: "", to: "" }); setParams({}, { replace: true }); }}>Clear all filters</Button>
        )}
      </div>
      {tl.isPending ? (
        <LoadingState label="Building timeline…" />
      ) : tl.isError ? (
        <ErrorState error={tl.error} onRetry={() => tl.refetch()} />
      ) : tl.data.items.length === 0 ? (
        filtered ? (
          <EmptyState icon="⌕" title="Nothing matches these filters">
            {selected.data ? `No matching events for ${personName(selected.data)}.` : "Try widening the year range or removing a type filter."}
          </EmptyState>
        ) : (
          <EmptyState icon="⌛" title="The timeline is empty" action={canEdit && <Button variant="primary" onClick={() => setModal({ open: true })}>Add an event</Button>}>
            Add people with birth or death dates, or record events like marriages, moves and family milestones.
          </EmptyState>
        )
      ) : (
        <div className={tl.isFetching ? "opacity-70" : ""}>
          {tl.data.truncated && <p className="mb-3 text-sm text-amber-700">Showing the first {tl.data.items.length} of {tl.data.total} entries — narrow the filters to see the rest.</p>}
          <TimelineList items={tl.data.items} onOpenEvent={canEdit ? (id) => setModal({ open: true, id }) : undefined} />
        </div>
      )}
      <EventFormModal open={modal.open} eventId={modal.id} onClose={() => setModal({ open: false })}
        defaultType={personId ? "residence" : "family"}
        defaultParticipants={selected.data ? [{ personId: selected.data.id, role: "principal", name: personName(selected.data) }] : []} />
    </PageContainer>
  );
}
