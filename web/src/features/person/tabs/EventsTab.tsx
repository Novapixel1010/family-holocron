import { useState } from "react";
import { useTimeline } from "@/api/hooks";
import { Button } from "@/components/ui/Button";
import { EmptyState, QueryBoundary } from "@/components/ui/States";
import { EventFormModal } from "@/features/timeline/EventFormModal";
import { TimelineList } from "@/features/timeline/TimelineList";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";

export function EventsTab({ person, canEdit }: PersonTabProps) {
  const tl = useTimeline({ personId: person.id });
  const [modal, setModal] = useState<{ open: boolean; id?: number }>({ open: false });
  return (
    <div>
      {canEdit && (
        <div className="mb-3 flex justify-end">
          <Button variant="primary" size="sm" onClick={() => setModal({ open: true })}>+ Add event</Button>
        </div>
      )}
      <QueryBoundary query={tl}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState icon="⌛" title="No facts or events yet" action={canEdit && <Button onClick={() => setModal({ open: true })}>Add the first event</Button>}>
              Add residences, occupations, marriages and other milestones in {personName(person)}’s life. Birth and death come from the profile.
            </EmptyState>
          ) : (
            <TimelineList items={data.items} onOpenEvent={canEdit ? (id) => setModal({ open: true, id }) : undefined} hidePerson={person.id} />
          )
        }
      </QueryBoundary>
      <EventFormModal open={modal.open} eventId={modal.id} onClose={() => setModal({ open: false })}
        defaultParticipants={[{ personId: person.id, role: "principal", name: personName(person) }]} />
    </div>
  );
}
