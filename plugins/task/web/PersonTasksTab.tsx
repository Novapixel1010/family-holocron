import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { EmptyState, QueryBoundary } from "@/components/ui/States";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";
import { useTasks } from "./api";
import { TaskModal } from "./TaskModal";
import { TaskRow } from "./TaskRow";

/** "Tasks" tab on the person profile. */
export function PersonTasksTab({ person, canEdit }: PersonTabProps) {
  const tasks = useTasks({ personId: person.id });
  const [modal, setModal] = useState<{ open: boolean; id?: number }>({ open: false });
  const name = personName(person);
  return (
    <div className="space-y-3">
      {canEdit && <div className="flex justify-end"><Button variant="primary" size="sm" onClick={() => setModal({ open: true })}>+ New task about {person.givenName || name}</Button></div>}
      <QueryBoundary query={tasks} errorTitle="Couldn't load tasks">
        {(list) => list.length === 0 ? (
          <EmptyState icon="☑" title="No research tasks" action={canEdit && <Button onClick={() => setModal({ open: true })}>Create a task</Button>}>
            Record what you still want to find out about {name}, and who is looking into it.
          </EmptyState>
        ) : (
          <ul className="space-y-3">{list.map((t) => <TaskRow key={t.id} task={t} hidePerson={person.id} onOpen={(x) => setModal({ open: true, id: x.id })} />)}</ul>
        )}
      </QueryBoundary>
      <TaskModal open={modal.open} taskId={modal.id} onClose={() => setModal({ open: false })} preset={{ people: [{ id: person.id, name }] }} />
    </div>
  );
}
