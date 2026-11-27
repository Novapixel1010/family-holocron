import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useDeletePerson, usePerson } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { NotFoundPage } from "@/app/NotFoundPage";
import { Attribution } from "@/components/ui/Attribution";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/Modal";
import { Badge, Tabs } from "@/components/ui/Page";
import { InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { usePluginSlots } from "@/plugins/usePluginSlots";
import { formatGenDate, lifespan, personName, SEX_LABEL } from "@/lib/format";

export function PersonPage() {
  const params = useParams();
  const id = Number(params.id);
  const person = usePerson(Number.isFinite(id) && id > 0 ? id : undefined);
  if (!Number.isFinite(id) || id <= 0) return <NotFoundPage />;
  return (
    <PageContainer>
      <QueryBoundary query={person} errorTitle="Couldn't load this person">
        {(p) => <Profile person={p} tab={params.tab} />}
      </QueryBoundary>
    </PageContainer>
  );
}

function Profile({ person: p, tab }: { person: NonNullable<ReturnType<typeof usePerson>["data"]>; tab?: string }) {
  const { canEdit } = useAuth();
  const slots = usePluginSlots();
  const navigate = useNavigate();
  const toast = useToast();
  const del = useDeletePerson();
  const [confirm, setConfirm] = useState(false);
  const tabs = slots.personTabs;
  const active = tabs.find((t) => t.id === tab) ?? tabs[0];
  const name = personName(p);

  const vitals = [
    (p.birthDate || p.birthPlace) && `Born ${[formatGenDate(p.birthDate), p.birthPlace && `in ${p.birthPlace}`].filter(Boolean).join(" ")}`,
    (p.deathDate || p.deathPlace) && `Died ${[formatGenDate(p.deathDate), p.deathPlace && `in ${p.deathPlace}`].filter(Boolean).join(" ")}`,
  ].filter(Boolean);

  return (
    <>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start">
        <Avatar name={name} sex={p.sex} mediaId={p.portraitMediaId} size={96} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-serif text-3xl font-semibold tracking-tight">{name}</h1>
            {p.nickname && <span className="text-lg text-stone-500">“{p.nickname}”</span>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-stone-600 dark:text-stone-400">
            {lifespan(p) && <span className="tabular-nums">{lifespan(p)}</span>}
            <Badge>{SEX_LABEL[p.sex]}</Badge>
            {p.deceased || p.deathDate ? <Badge>Deceased</Badge> : <Badge tone="brand">Living</Badge>}
          </div>
          {vitals.length > 0 && <p className="mt-2 text-sm text-stone-700 dark:text-stone-300">{vitals.join(" · ")}</p>}
          <div className="mt-2"><Attribution value={p} /></div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link to={`/tree/${p.id}`} className="rounded-lg px-3.5 py-2 text-sm font-medium ring-1 ring-stone-300 hover:bg-stone-50 dark:ring-stone-700 dark:hover:bg-stone-800">View in tree</Link>
          {canEdit && <Link to={`/people/${p.id}/edit`} className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-brand-700">Edit</Link>}
          {canEdit && <Button variant="ghost" onClick={() => setConfirm(true)} className="text-red-700 dark:text-red-400">Delete</Button>}
        </div>
      </div>
      <Tabs tabs={tabs.map((t) => ({ id: t.id, label: t.label }))} value={active?.id ?? ""} onChange={(t) => navigate(`/people/${p.id}/${t}`, { replace: true })} />
      <div className="pt-5">{active && <active.component person={p} canEdit={canEdit} />}</div>
      <ConfirmDialog
        open={confirm}
        title={`Delete ${name}?`}
        onClose={() => setConfirm(false)}
        loading={del.isPending}
        error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try {
            await del.mutateAsync(p.id);
            toast(`${name} deleted`);
            navigate("/people", { replace: true });
          } catch {
            /* shown inline */
          }
        }}
      >
        <p>This removes {name}, their relationships and notes, and any events only they took part in. Photos stay in the media library.</p>
        <p>The deletion is recorded in the change history, and you can recover it from a backup.</p>
      </ConfirmDialog>
    </>
  );
}
