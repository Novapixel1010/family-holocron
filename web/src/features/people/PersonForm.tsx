import { useState, type FormEvent } from "react";
import { ApiError } from "@/api/client";
import type { PersonInput } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { DateField } from "@/components/ui/DateField";
import { Select, TextInput } from "@/components/ui/Field";
import { MarkdownEditor } from "@/components/ui/Markdown";
import { InlineError } from "@/components/ui/States";
import { checkVitals, parseISODate } from "@/lib/dates";
import { formatGenDate, isSortableDate } from "@/lib/format";

export const emptyPerson: PersonInput = {
  givenName: "", surname: "", nickname: "", sex: "U", birthDate: "", birthPlace: "", deathDate: "", deathPlace: "",
  deceased: false, about: "", portraitMediaId: null,
};

export const DATE_HINT = "e.g. 1850-03-12, 12 MAR 1850, 1850, ABT 1850, BEF 1900";

export function dateHint(v: string) {
  if (!v.trim()) return DATE_HINT;
  return isSortableDate(v) ? `Reads as: ${formatGenDate(v)}` : "Kept as written, but can’t be placed on the timeline. " + DATE_HINT;
}

export function PersonForm({
  initial, onSubmit, submitLabel, onCancel, compact = false,
}: { initial: PersonInput; onSubmit: (p: PersonInput) => Promise<unknown>; submitLabel: string; onCancel?: () => void; compact?: boolean }) {
  const [p, setP] = useState<PersonInput>(initial);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const fields = error instanceof ApiError ? error.fields : {};
  const set = <K extends keyof PersonInput>(k: K, v: PersonInput[K]) => setP((x) => ({ ...x, [k]: v }));

  // Birth/death dates: YYYY-MM-DD (or YYYY-MM / YYYY). Errors show once a field is left or on submit.
  const [touched, setTouched] = useState({ birth: false, death: false });
  const [attempted, setAttempted] = useState(false);
  const vitals = checkVitals(p.birthDate, p.deathDate, initial.birthDate, initial.deathDate);
  const legacyHint = (v: string, stored: string) =>
    v && v === stored && !parseISODate(v).ok ? `Recorded as “${v}” before dates were standardized. Pick a date, or enter YYYY if only the year is known.` : undefined;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setTouched({ birth: true, death: true });
    setAttempted(true);
    if (vitals.birthError || vitals.deathError) return;
    setBusy(true);
    setError(null);
    try {
      await onSubmit({ ...p, birthDate: vitals.birth ?? "", deathDate: vitals.death ?? "" });
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextInput label="Given names" value={p.givenName} onChange={(e) => set("givenName", e.target.value)} error={fields.givenName} autoFocus />
        <TextInput label="Surname" value={p.surname} onChange={(e) => set("surname", e.target.value)} error={fields.surname} hint="Birth surname is usually best." />
        <TextInput label="Nickname" value={p.nickname} onChange={(e) => set("nickname", e.target.value)} error={fields.nickname} />
        <Select label="Sex" value={p.sex} onChange={(e) => set("sex", e.target.value as PersonInput["sex"])} error={fields.sex}
          options={[{ value: "U", label: "Unknown" }, { value: "F", label: "Female" }, { value: "M", label: "Male" }, { value: "X", label: "Other" }]} />
      </div>
      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-stone-800 dark:text-stone-200">Birth</legend>
        <DateField label="Birth date" value={p.birthDate} onChange={(v) => set("birthDate", v)} onBlur={() => setTouched((t) => ({ ...t, birth: true }))}
          error={(touched.birth && vitals.birthError) || fields.birthDate} hint={legacyHint(p.birthDate, initial.birthDate)} />
        <TextInput label="Place" value={p.birthPlace} onChange={(e) => set("birthPlace", e.target.value)} error={fields.birthPlace} />
      </fieldset>
      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-stone-800 dark:text-stone-200">Death</legend>
        <DateField label="Death date" value={p.deathDate} onChange={(v) => set("deathDate", v)} onBlur={() => setTouched((t) => ({ ...t, death: true }))}
          error={(touched.death && vitals.deathError) || fields.deathDate} hint={legacyHint(p.deathDate, initial.deathDate)}
          warning={touched.birth || touched.death ? vitals.warnings[0] : undefined} />
        <TextInput label="Place" value={p.deathPlace} onChange={(e) => set("deathPlace", e.target.value)} error={fields.deathPlace} />
        <label className="flex items-center gap-2 text-sm sm:col-span-2">
          <input type="checkbox" className="rounded border-stone-300" checked={p.deceased || !!p.deathDate || !!p.deathPlace}
            disabled={!!p.deathDate || !!p.deathPlace} onChange={(e) => set("deceased", e.target.checked)} />
          Deceased (even if the date is unknown)
        </label>
      </fieldset>
      {!compact && (
        <MarkdownEditor label="About" value={p.about} onChange={(v) => set("about", v)} rows={8} error={fields.about}
          placeholder="Family stories, personality, what they did, how people remember them…" />
      )}
      {attempted && (vitals.birthError || vitals.deathError) && (
        <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/50 dark:text-red-300">Please fix the highlighted dates.</p>
      )}
      <InlineError error={error} />
      <div className="flex justify-end gap-2">
        {onCancel && <Button onClick={onCancel} disabled={busy}>Cancel</Button>}
        <Button type="submit" variant="primary" loading={busy}>{submitLabel}</Button>
      </div>
    </form>
  );
}
