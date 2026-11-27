import { AuditList } from "@/features/audit/AuditList";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";

export function HistoryTab({ person }: PersonTabProps) {
  return (
    <div className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
      <p className="mb-2 text-sm text-stone-500">Every change to {personName(person)} — their profile, relationships, events, notes and photos — and who made it.</p>
      <AuditList personId={person.id} />
    </div>
  );
}
