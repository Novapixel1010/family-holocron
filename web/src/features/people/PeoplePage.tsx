import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { usePersons } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/Page";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { formatVitalDate } from "@/lib/dates";
import { lifespan, personName } from "@/lib/format";

const PAGE = 50;

export function PeoplePage() {
  const { canEdit } = useAuth();
  const [params, setParams] = useSearchParams();
  const q = params.get("q") ?? "";
  const sort = params.get("sort") ?? "name";
  const page = Math.max(0, Number(params.get("page") ?? 0) || 0);
  const [text, setText] = useState(q);
  useEffect(() => {
    const t = setTimeout(() => {
      if (text !== q) setParams((p) => { const n = new URLSearchParams(p); text ? n.set("q", text) : n.delete("q"); n.delete("page"); return n; }, { replace: true });
    }, 250);
    return () => clearTimeout(t);
  }, [text, q, setParams]);
  const query = usePersons({ q, sort, limit: PAGE, offset: page * PAGE });
  const setParam = (k: string, v: string) => setParams((p) => { const n = new URLSearchParams(p); n.set(k, v); if (k !== "page") n.delete("page"); return n; });

  return (
    <PageContainer>
      <PageHeader
        title="People"
        subtitle={query.data ? `${query.data.total.toLocaleString()} ${query.data.total === 1 ? "person" : "people"}${q ? ` matching “${q}”` : ""}` : undefined}
        actions={canEdit && <Link to="/people/new" className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-brand-700">+ Add person</Link>}
      />
      <div className="mb-4 flex flex-wrap gap-2">
        <input type="search" aria-label="Search people" placeholder="Search by name or place…" className={`${inputClass} max-w-sm flex-1`} value={text} onChange={(e) => setText(e.target.value)} />
        <select aria-label="Sort" className={`${inputClass} w-auto`} value={sort} onChange={(e) => setParam("sort", e.target.value)}>
          <option value="name">Sort by surname</option>
          <option value="birth">Sort by birth date</option>
          <option value="updated">Recently updated</option>
        </select>
      </div>
      {query.isPending ? (
        <LoadingState label="Loading people…" />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => query.refetch()} />
      ) : query.data.items.length === 0 ? (
        q ? (
          <EmptyState icon="⌕" title="No matches" action={<Button onClick={() => setText("")}>Clear search</Button>}>Nobody matches “{q}”. Try a different spelling or a place name.</EmptyState>
        ) : (
          <EmptyState icon="☺" title="No people yet" action={canEdit && <Link to="/people/new" className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white">Add the first person</Link>}>
            People you add will be listed here.
          </EmptyState>
        )
      ) : (
        <>
          <ul className={`divide-y divide-stone-200 overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-stone-200 dark:divide-stone-800 dark:bg-stone-900 dark:ring-stone-800 ${query.isFetching ? "opacity-70" : ""}`}>
            {query.data.items.map((p) => (
              <li key={p.id}>
                <Link to={`/people/${p.id}`} className="flex items-center gap-3 px-4 py-3 hover:bg-stone-50 dark:hover:bg-stone-800/60">
                  <Avatar name={personName(p)} sex={p.sex} mediaId={p.portraitMediaId} />
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium">
                      {p.surname ? <><span>{p.givenName}</span> <span className="uppercase tracking-wide">{p.surname}</span></> : personName(p)}
                      {p.nickname && <span className="ml-1 text-stone-500">“{p.nickname}”</span>}
                    </div>
                    <div className="truncate text-sm text-stone-500">
                      {[p.birthDate && `b. ${formatVitalDate(p.birthDate)}${p.birthPlace ? `, ${p.birthPlace}` : ""}`, p.deathDate && `d. ${formatVitalDate(p.deathDate)}`].filter(Boolean).join(" · ") || "No dates recorded"}
                    </div>
                  </div>
                  <span className="hidden text-sm tabular-nums text-stone-500 sm:block">{lifespan(p)}</span>
                </Link>
              </li>
            ))}
          </ul>
          {query.data.total > PAGE && (
            <div className="mt-4 flex items-center justify-between text-sm">
              <Button size="sm" disabled={page === 0} onClick={() => setParam("page", String(page - 1))}>← Previous</Button>
              <span className="text-stone-500">Page {page + 1} of {Math.ceil(query.data.total / PAGE)}</span>
              <Button size="sm" disabled={(page + 1) * PAGE >= query.data.total} onClick={() => setParam("page", String(page + 1))}>Next →</Button>
            </div>
          )}
        </>
      )}
    </PageContainer>
  );
}
