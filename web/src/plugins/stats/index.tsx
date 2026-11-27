// Web half of the built-in "stats" plugin (see plugin/builtin/stats).
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { PageContainer } from "@/app/Layout";
import { Card, PageHeader } from "@/components/ui/Page";
import { EmptyState, QueryBoundary } from "@/components/ui/States";
import { registerWebPlugin } from "../registry";

interface Count { label: string; count: number }
interface Summary {
  people: number; living: number; deceased: number; bySex: Count[]; topSurnames: Count[]; birthsByCentury: Count[];
  averageLifespan: number; lifespanSample: number; earliestBirth: string; changesRecorded: number;
}

function Bars({ data, label }: { data: Count[]; label: string }) {
  const max = Math.max(1, ...data.map((d) => d.count));
  if (data.length === 0) return <p className="text-sm text-stone-500">Not enough data yet.</p>;
  return (
    <ul className="space-y-1.5" aria-label={label}>
      {data.map((d) => (
        <li key={d.label} className="grid grid-cols-[8rem,1fr,2.5rem] items-center gap-2 text-sm">
          <span className="truncate">{d.label}</span>
          <span className="h-3 rounded-full bg-stone-100 dark:bg-stone-800">
            <span className="block h-3 rounded-full bg-brand-500" style={{ width: `${(d.count / max) * 100}%` }} />
          </span>
          <span className="text-right tabular-nums text-stone-500">{d.count}</span>
        </li>
      ))}
    </ul>
  );
}

function StatsPage() {
  const q = useQuery({ queryKey: ["plugin-stats"], queryFn: () => api.get<Summary>("/api/v1/plugins/stats/summary") });
  return (
    <PageContainer>
      <PageHeader title="Family statistics" subtitle="Provided by the built-in Statistics plugin." />
      <QueryBoundary query={q} errorTitle="Couldn't compute statistics">
        {(s) => s.people === 0 ? (
          <EmptyState icon="∑" title="No statistics yet">Add people with birth and death dates to see patterns across generations.</EmptyState>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <dl className="grid grid-cols-2 gap-4">
                <div><dt className="text-sm text-stone-500">People</dt><dd className="text-2xl font-semibold">{s.people}</dd></div>
                <div><dt className="text-sm text-stone-500">Living / deceased</dt><dd className="text-2xl font-semibold">{s.living} / {s.deceased}</dd></div>
                <div><dt className="text-sm text-stone-500">Average lifespan</dt><dd className="text-2xl font-semibold">{s.lifespanSample ? `${s.averageLifespan.toFixed(1)} yrs` : "—"}</dd>
                  {s.lifespanSample > 0 && <dd className="text-xs text-stone-500">from {s.lifespanSample} people with both dates</dd>}</div>
                <div><dt className="text-sm text-stone-500">Earliest birth</dt><dd className="text-2xl font-semibold">{s.earliestBirth || "—"}</dd></div>
              </dl>
              <p className="mt-4 text-xs text-stone-400">{s.changesRecorded} changes counted by this plugin since it was enabled.</p>
            </Card>
            <Card><h2 className="mb-3 font-semibold">Most common surnames</h2><Bars data={s.topSurnames} label="Surnames" /></Card>
            <Card><h2 className="mb-3 font-semibold">Births by century</h2><Bars data={s.birthsByCentury} label="Births by century" /></Card>
            <Card><h2 className="mb-3 font-semibold">By sex</h2><Bars data={s.bySex} label="By sex" /></Card>
          </div>
        )}
      </QueryBoundary>
    </PageContainer>
  );
}

registerWebPlugin({
  id: "stats",
  nav: [{ id: "stats", label: "Statistics", icon: "∑", order: 50, path: "", component: StatsPage }],
});
