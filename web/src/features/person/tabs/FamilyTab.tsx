import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "@/api/client";
import { useCreatePerson, useCreateRelationship, useDeleteRelationship, useFamily, useUpdateRelationship } from "@/api/hooks";
import type { Person, Relationship, RelationshipInput, Relative } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { DateField } from "@/components/ui/DateField";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { Select, TextArea, TextInput } from "@/components/ui/Field";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { EmptyState, InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { dateHint } from "@/features/people/PersonForm";
import { checkVitals } from "@/lib/dates";
import { lifespan, personName, titleCase } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";

type Kind = "parent" | "spouse" | "child";

const PARENT_SUBTYPES = ["biological", "adopted", "step", "foster", "guardian", "unknown"];
const SPOUSE_SUBTYPES = ["married", "partner", "engaged", "divorced", "separated", "widowed", "unknown"];

export function FamilyTab({ person, canEdit }: PersonTabProps) {
  const fam = useFamily(person.id);
  const [adding, setAdding] = useState<Kind | null>(null);
  const [editing, setEditing] = useState<Relationship | null>(null);
  return (
    <div>
      <QueryBoundary query={fam} errorTitle="Couldn't load family">
        {(f) => {
          const total = f.parents.length + f.spouses.length + f.children.length + f.siblings.length;
          return (
            <div className="space-y-6">
              {total === 0 && (
                <EmptyState icon="⚭" title="No relatives linked yet">
                  {canEdit ? "Link parents, partners and children to build the tree." : "No family members have been linked to this person."}
                </EmptyState>
              )}
              <Group title="Parents" items={f.parents} onAdd={canEdit ? () => setAdding("parent") : undefined} addLabel="Add parent" onEdit={canEdit ? setEditing : undefined} empty="No parents recorded." />
              <Group title="Partners & spouses" items={f.spouses} onAdd={canEdit ? () => setAdding("spouse") : undefined} addLabel="Add partner" onEdit={canEdit ? setEditing : undefined} empty="No partners recorded." />
              <Group title="Children" items={f.children} onAdd={canEdit ? () => setAdding("child") : undefined} addLabel="Add child" onEdit={canEdit ? setEditing : undefined} empty="No children recorded." />
              {f.siblings.length > 0 && <Group title="Siblings" items={f.siblings} empty="" note="Siblings share at least one parent." />}
            </div>
          );
        }}
      </QueryBoundary>
      {adding && <AddRelativeModal person={person} kind={adding} onClose={() => setAdding(null)} />}
      {editing && <EditRelationshipModal rel={editing} onClose={() => setEditing(null)} />}
    </div>
  );
}

function Group({ title, items, onAdd, addLabel, onEdit, empty, note }: {
  title: string; items: Relative[]; onAdd?: () => void; addLabel?: string; onEdit?: (r: Relationship) => void; empty: string; note?: string;
}) {
  if (!onAdd && items.length === 0) return null;
  return (
    <section>
      <div className="mb-2 flex items-center justify-between">
        <h3 className="font-semibold">{title}</h3>
        {onAdd && <Button size="sm" onClick={onAdd}>+ {addLabel}</Button>}
      </div>
      {note && <p className="mb-2 text-xs text-stone-500">{note}</p>}
      {items.length === 0 ? (
        <p className="text-sm text-stone-500">{empty}</p>
      ) : (
        <ul className="grid gap-2 sm:grid-cols-2">
          {items.map(({ person: p, relationship: r }) => (
            <li key={`${p.id}-${r?.id ?? "s"}`} className="flex items-center gap-3 rounded-xl bg-white p-3 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
              <Avatar name={personName(p)} sex={p.sex} mediaId={p.portraitMediaId} />
              <div className="min-w-0 flex-1">
                <Link to={`/people/${p.id}`} className="block truncate font-medium hover:underline">{personName(p)}</Link>
                <div className="text-xs text-stone-500">
                  {[lifespan(p), r && r.subtype !== "biological" && r.subtype !== "unknown" ? titleCase(r.subtype) : "", r?.startDate && `from ${r.startDate}`].filter(Boolean).join(" · ")}
                </div>
              </div>
              {onEdit && r && <Button size="sm" variant="ghost" onClick={() => onEdit(r)} aria-label={`Edit relationship with ${personName(p)}`}>Edit</Button>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function AddRelativeModal({ person, kind, onClose }: { person: Person; kind: Kind; onClose: () => void }) {
  const [mode, setMode] = useState<"existing" | "new">("existing");
  const [other, setOther] = useState<Person | null>(null);
  const [np, setNp] = useState({ givenName: "", surname: kind === "child" || kind === "parent" ? person.surname : "", sex: "U" as Person["sex"], birthDate: "" });
  const [subtype, setSubtype] = useState(kind === "spouse" ? "married" : "biological");
  const [startDate, setStartDate] = useState("");
  const [birthTouched, setBirthTouched] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const createPerson = useCreatePerson();
  const createRel = useCreateRelationship();
  const toast = useToast();
  const label = kind === "parent" ? "parent" : kind === "spouse" ? "partner" : "child";
  const fields = error instanceof ApiError ? error.fields : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    if (mode === "new" && checkVitals(np.birthDate, "").birthError) {
      setBirthTouched(true);
      setBusy(false);
      return;
    }
    try {
      let target = other;
      if (mode === "new") {
        target = await createPerson.mutateAsync({
          givenName: np.givenName, surname: np.surname, nickname: "", sex: np.sex, birthDate: checkVitals(np.birthDate, "").birth ?? "", birthPlace: "",
          deathDate: "", deathPlace: "", deceased: false, about: "", portraitMediaId: null,
        });
        // If linking fails below, a retry must not create the person twice.
        setOther(target);
        setMode("existing");
      }
      if (!target) {
        setError(new ApiError(422, "validation", `Choose the ${label} to link.`));
        setBusy(false);
        return;
      }
      const input: RelationshipInput =
        kind === "parent" ? { type: "parent", person1Id: target.id, person2Id: person.id, subtype, startDate: "", endDate: "", notes: "" }
        : kind === "child" ? { type: "parent", person1Id: person.id, person2Id: target.id, subtype, startDate: "", endDate: "", notes: "" }
        : { type: "spouse", person1Id: person.id, person2Id: target.id, subtype, startDate, endDate: "", notes: "" };
      await createRel.mutateAsync(input);
      toast(`${personName(target)} linked as ${label}`);
      onClose();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Add ${label} of ${personName(person)}`}>
      <form onSubmit={submit} className="space-y-4">
        <div className="flex gap-1 rounded-lg bg-stone-100 p-1 text-sm dark:bg-stone-800" role="radiogroup">
          {(["existing", "new"] as const).map((m) => (
            <button key={m} type="button" role="radio" aria-checked={mode === m} onClick={() => setMode(m)}
              className={`flex-1 rounded-md px-3 py-1.5 font-medium ${mode === m ? "bg-white shadow-sm dark:bg-stone-900" : "text-stone-500"}`}>
              {m === "existing" ? "Someone already in the tree" : "A new person"}
            </button>
          ))}
        </div>
        {mode === "existing" ? (
          <PersonPicker label={titleCase(label)} value={other} onChange={setOther} exclude={[person.id]} autoFocus />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            <TextInput label="Given names" value={np.givenName} onChange={(e) => setNp({ ...np, givenName: e.target.value })} error={fields.givenName} autoFocus />
            <TextInput label="Surname" value={np.surname} onChange={(e) => setNp({ ...np, surname: e.target.value })} />
            <Select label="Sex" value={np.sex} onChange={(e) => setNp({ ...np, sex: e.target.value as Person["sex"] })}
              options={[{ value: "U", label: "Unknown" }, { value: "F", label: "Female" }, { value: "M", label: "Male" }, { value: "X", label: "Other" }]} />
            <DateField label="Birth date" value={np.birthDate} onChange={(v) => setNp({ ...np, birthDate: v })} onBlur={() => setBirthTouched(true)}
              error={(birthTouched && checkVitals(np.birthDate, "").birthError) || fields.birthDate} />
          </div>
        )}
        <Select label="Relationship" value={subtype} onChange={(e) => setSubtype(e.target.value)}
          options={(kind === "spouse" ? SPOUSE_SUBTYPES : PARENT_SUBTYPES).map((s) => ({ value: s, label: titleCase(s) }))} />
        {kind === "spouse" && <TextInput label="Since (optional)" value={startDate} onChange={(e) => setStartDate(e.target.value)} hint={dateHint(startDate)} />}
        <InlineError error={error} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose} disabled={busy}>Cancel</Button>
          <Button type="submit" variant="primary" loading={busy}>Link {label}</Button>
        </div>
      </form>
    </Modal>
  );
}

function EditRelationshipModal({ rel, onClose }: { rel: Relationship; onClose: () => void }) {
  const [r, setR] = useState<RelationshipInput>({ type: rel.type, person1Id: rel.person1Id, person2Id: rel.person2Id, subtype: rel.subtype, startDate: rel.startDate, endDate: rel.endDate, notes: rel.notes });
  const [confirm, setConfirm] = useState(false);
  const update = useUpdateRelationship();
  const del = useDeleteRelationship();
  const toast = useToast();
  const fields = update.error instanceof ApiError ? update.error.fields : {};
  return (
    <Modal open onClose={onClose} title={rel.type === "spouse" ? "Edit partnership" : "Edit parent link"}>
      <form className="space-y-4" onSubmit={async (e) => {
        e.preventDefault();
        try {
          await update.mutateAsync({ id: rel.id, input: r });
          toast("Relationship updated");
          onClose();
        } catch { /* inline */ }
      }}>
        <Select label="Type" value={r.subtype} onChange={(e) => setR({ ...r, subtype: e.target.value })} error={fields.subtype}
          options={(rel.type === "spouse" ? SPOUSE_SUBTYPES : PARENT_SUBTYPES).map((s) => ({ value: s, label: titleCase(s) }))} />
        {rel.type === "spouse" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <TextInput label="Start date" value={r.startDate} onChange={(e) => setR({ ...r, startDate: e.target.value })} hint={dateHint(r.startDate)} />
            <TextInput label="End date" value={r.endDate} onChange={(e) => setR({ ...r, endDate: e.target.value })} hint={dateHint(r.endDate)} />
          </div>
        )}
        <TextArea label="Notes" rows={3} value={r.notes} onChange={(e) => setR({ ...r, notes: e.target.value })} />
        <Attribution value={rel} />
        <InlineError error={update.error} />
        <div className="flex flex-wrap justify-between gap-2">
          <Button variant="danger" onClick={() => setConfirm(true)}>Remove link</Button>
          <div className="flex gap-2">
            <Button onClick={onClose}>Cancel</Button>
            <Button type="submit" variant="primary" loading={update.isPending}>Save</Button>
          </div>
        </div>
      </form>
      <ConfirmDialog open={confirm} title="Remove this relationship?" confirmLabel="Remove" onClose={() => setConfirm(false)} loading={del.isPending}
        error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try {
            await del.mutateAsync(rel.id);
            toast("Relationship removed");
            onClose();
          } catch { /* inline */ }
        }}>
        Both people stay in the tree; only the link between them is removed.
      </ConfirmDialog>
    </Modal>
  );
}
