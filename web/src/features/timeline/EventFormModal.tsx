import { useState, type FormEvent } from "react";
import { ApiError } from "@/api/client";
import { useCreateEvent, useDeleteEvent, useEvent, useEventTypes, useUpdateEvent } from "@/api/hooks";
import type { EventInput, EventParticipant, Person } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { Button } from "@/components/ui/Button";
import { inputClass, Select, TextInput } from "@/components/ui/Field";
import { MarkdownEditor } from "@/components/ui/Markdown";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { ErrorState, InlineError, LoadingState } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { dateHint } from "@/features/people/PersonForm";
import { titleCase } from "@/lib/format";

const NO_PEOPLE_TYPES = ["family", "historical"];

export function EventFormModal({
  open, onClose, eventId, defaultParticipants = [], defaultType = "residence",
}: { open: boolean; onClose: () => void; eventId?: number; defaultParticipants?: EventParticipant[]; defaultType?: string }) {
  const existing = useEvent(open ? eventId : undefined);
  const types = useEventTypes();
  const title = eventId ? "Edit event" : "Add event";
  return (
    <Modal open={open} onClose={onClose} title={title} size="lg">
      {eventId && existing.isPending ? (
        <LoadingState />
      ) : eventId && existing.isError ? (
        <ErrorState error={existing.error} onRetry={() => existing.refetch()} />
      ) : types.isPending ? (
        <LoadingState />
      ) : types.isError ? (
        <ErrorState error={types.error} onRetry={() => types.refetch()} />
      ) : (
        <EventForm
          key={eventId ?? "new"}
          eventId={eventId}
          types={types.data.eventTypes}
          initial={
            existing.data
              ? { type: existing.data.type, title: existing.data.title, date: existing.data.date, place: existing.data.place, description: existing.data.description,
                  relationshipId: existing.data.relationshipId, participants: existing.data.participants }
              : { type: defaultType, title: "", date: "", place: "", description: "", relationshipId: null, participants: defaultParticipants }
          }
          attribution={existing.data}
          onDone={onClose}
        />
      )}
    </Modal>
  );
}

function EventForm({
  eventId, types, initial, attribution, onDone,
}: { eventId?: number; types: string[]; initial: EventInput; attribution?: Parameters<typeof Attribution>[0]["value"]; onDone: () => void }) {
  const [e, setE] = useState<EventInput>(initial);
  const [error, setError] = useState<unknown>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const create = useCreateEvent();
  const update = useUpdateEvent();
  const del = useDeleteEvent();
  const toast = useToast();
  const fields = error instanceof ApiError ? error.fields : {};
  const busy = create.isPending || update.isPending;

  const addPerson = (person: Person | null) => {
    if (!person) return;
    setE((x) => (x.participants.some((p) => p.personId === person.id) ? x : {
      ...x,
      participants: [...x.participants, {
        personId: person.id, role: x.participants.length ? (x.type === "marriage" ? "spouse" : "participant") : "principal",
        name: `${person.givenName} ${person.surname}`.trim() || "Unnamed person",
      }],
    }));
  };

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setError(null);
    try {
      if (eventId) await update.mutateAsync({ id: eventId, input: e });
      else await create.mutateAsync(e);
      toast(eventId ? "Event updated" : "Event added");
      onDone();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <Select label="Type" value={e.type} onChange={(x) => setE({ ...e, type: x.target.value })} error={fields.type}
          options={types.map((t) => ({ value: t, label: t === "family" ? "Family milestone" : t === "historical" ? "Historical event" : titleCase(t) }))} />
        <TextInput label={e.type === "custom" ? "Title (required)" : "Title"} value={e.title} onChange={(x) => setE({ ...e, title: x.target.value })} error={fields.title}
          placeholder={e.type === "occupation" ? "e.g. Blacksmith" : e.type === "historical" ? "e.g. Great Flood of 1953" : "Optional"} />
        <TextInput label="Date" value={e.date} onChange={(x) => setE({ ...e, date: x.target.value })} error={fields.date} hint={dateHint(e.date)} />
        <TextInput label="Place" value={e.place} onChange={(x) => setE({ ...e, place: x.target.value })} error={fields.place} />
      </div>
      <div className="space-y-2">
        <div className="text-sm font-medium text-stone-700 dark:text-stone-300">
          People {NO_PEOPLE_TYPES.includes(e.type) && <span className="font-normal text-stone-500">(optional for {e.type === "family" ? "family milestones" : "historical events"})</span>}
        </div>
        {e.participants.length > 0 && (
          <ul className="space-y-2">
            {e.participants.map((p, i) => (
              <li key={p.personId} className="flex items-center gap-2">
                <span className="flex-1 truncate text-sm">{p.name || `Person #${p.personId}`}</span>
                <input aria-label={`Role of ${p.name}`} className={`${inputClass} w-36`} value={p.role}
                  onChange={(x) => setE({ ...e, participants: e.participants.map((q, j) => (j === i ? { ...q, role: x.target.value } : q)) })} />
                <Button size="sm" variant="ghost" onClick={() => setE({ ...e, participants: e.participants.filter((_, j) => j !== i) })} aria-label={`Remove ${p.name}`}>✕</Button>
              </li>
            ))}
          </ul>
        )}
        <PersonPicker label="Add a person" value={null} onChange={addPerson} exclude={e.participants.map((p) => p.personId)} />
        {fields.participants && <p className="text-xs text-red-600">{fields.participants}</p>}
      </div>
      <MarkdownEditor label="Description" value={e.description} onChange={(v) => setE({ ...e, description: v })} rows={4} error={fields.description} />
      {attribution && <Attribution value={attribution} />}
      <InlineError error={error} />
      <div className="flex flex-wrap justify-between gap-2">
        {eventId ? <Button variant="danger" onClick={() => setConfirmDelete(true)}>Delete event</Button> : <span />}
        <div className="flex gap-2">
          <Button onClick={onDone} disabled={busy}>Cancel</Button>
          <Button type="submit" variant="primary" loading={busy}>{eventId ? "Save event" : "Add event"}</Button>
        </div>
      </div>
      <ConfirmDialog
        open={confirmDelete}
        title="Delete this event?"
        onClose={() => setConfirmDelete(false)}
        loading={del.isPending}
        error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try {
            await del.mutateAsync(eventId!);
            toast("Event deleted");
            setConfirmDelete(false);
            onDone();
          } catch {
            /* shown inline */
          }
        }}
      >
        The event will be removed from everyone’s timeline. This is recorded in the change history.
      </ConfirmDialog>
    </form>
  );
}
