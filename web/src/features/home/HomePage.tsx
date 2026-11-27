import { Link } from "react-router-dom";
import { usePersons, useStats } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Avatar } from "@/components/ui/Avatar";
import { Card, PageHeader } from "@/components/ui/Page";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { AuditList } from "@/features/audit/AuditList";
import { lifespan, personName, relativeTime } from "@/lib/format";

function StatTile({ label, value, to }: { label: string; value: number; to: string }) {
  return (
    <Link to={to} className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-stone-200 transition hover:ring-brand-300 dark:bg-stone-900 dark:ring-stone-800">
      <div className="text-2xl font-semibold tabular-nums">{value.toLocaleString()}</div>
      <div className="text-sm text-stone-500">{label}</div>
    </Link>
  );
}

export function HomePage() {
  const { user, canEdit } = useAuth();
  const stats = useStats();
  const recent = usePersons({ sort: "updated", limit: 6 });

  return (
    <PageContainer>
      <PageHeader title={`Welcome, ${user.displayName || user.username}`} subtitle="Your family archive at a glance." />
      {stats.isPending ? (
        <LoadingState />
      ) : stats.isError ? (
        <ErrorState error={stats.error} onRetry={() => stats.refetch()} />
      ) : stats.data.persons === 0 ? (
        <EmptyState
          icon="🌱"
          title="Your family tree is empty"
          action={
            canEdit ? (
              <div className="flex flex-wrap justify-center gap-2">
                <Link to="/people/new" className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-brand-700">Add the first person</Link>
                <Link to="/settings/import-export" className="rounded-lg px-3.5 py-2 text-sm font-medium ring-1 ring-stone-300 hover:bg-stone-50 dark:ring-stone-700 dark:hover:bg-stone-800">Import a GEDCOM file</Link>
              </div>
            ) : undefined
          }
        >
          {canEdit ? "Start with yourself or the oldest ancestor you know, or import an existing tree." : "An editor hasn’t added anyone yet. Check back soon!"}
        </EmptyState>
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatTile label="People" value={stats.data.persons} to="/people" />
            <StatTile label="Surnames" value={stats.data.surnames} to="/people" />
            <StatTile label="Events" value={stats.data.events} to="/timeline" />
            <StatTile label="Photos & files" value={stats.data.media} to="/media" />
          </div>
          <div className="grid gap-6 lg:grid-cols-5">
            <Card className="lg:col-span-2">
              <h2 className="mb-3 font-semibold">Recently updated people</h2>
              {recent.isPending ? (
                <LoadingState />
              ) : recent.isError ? (
                <ErrorState error={recent.error} onRetry={() => recent.refetch()} />
              ) : (
                <ul className="space-y-1">
                  {recent.data.items.map((p) => (
                    <li key={p.id}>
                      <Link to={`/people/${p.id}`} className="flex items-center gap-3 rounded-lg p-2 hover:bg-stone-50 dark:hover:bg-stone-800">
                        <Avatar name={personName(p)} sex={p.sex} mediaId={p.portraitMediaId} size={36} />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium">{personName(p)}</div>
                          <div className="text-xs text-stone-500">{lifespan(p) || "Dates unknown"} · updated {relativeTime(p.updatedAt)}</div>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
            <Card className="lg:col-span-3">
              <h2 className="mb-1 font-semibold">Recent activity</h2>
              <AuditList pageSize={8} />
            </Card>
          </div>
        </div>
      )}
    </PageContainer>
  );
}
