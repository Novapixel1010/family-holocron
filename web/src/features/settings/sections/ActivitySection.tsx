import { useState } from "react";
import { useUsers } from "@/api/hooks";
import { inputClass } from "@/components/ui/Field";
import { Card } from "@/components/ui/Page";
import { AuditList } from "@/features/audit/AuditList";

const types = [
  { value: "", label: "Everything" }, { value: "person", label: "People" }, { value: "relationship", label: "Relationships" },
  { value: "event", label: "Events" }, { value: "note", label: "Notes" }, { value: "media", label: "Media" },
  { value: "user", label: "User accounts" }, { value: "system", label: "Backups, restores & imports" }, { value: "setting", label: "Settings" },
];

export function ActivitySection() {
  const [entityType, setEntityType] = useState("");
  const [userId, setUserId] = useState<number | undefined>();
  const users = useUsers();
  return (
    <Card>
      <h2 className="mb-1 font-semibold">Activity log</h2>
      <p className="mb-4 text-sm text-stone-500">A permanent record of who changed what, and when.</p>
      <div className="mb-3 flex flex-wrap gap-2">
        <select aria-label="Filter by type" className={`${inputClass} w-auto`} value={entityType} onChange={(e) => setEntityType(e.target.value)}>
          {types.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
        </select>
        <select aria-label="Filter by user" className={`${inputClass} w-auto`} value={userId ?? ""} onChange={(e) => setUserId(Number(e.target.value) || undefined)} disabled={!users.data}>
          <option value="">{users.isError ? "Users unavailable" : "All users"}</option>
          {users.data?.map((u) => <option key={u.id} value={u.id}>{u.displayName || u.username}</option>)}
        </select>
      </div>
      <AuditList key={`${entityType}-${userId}`} entityType={entityType || undefined} userId={userId} emptyText="Nothing matches these filters yet." />
    </Card>
  );
}
