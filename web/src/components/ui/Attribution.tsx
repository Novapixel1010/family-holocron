import type { Attribution as Attr } from "@/api/types";
import { dateTime, relativeTime, userLabel } from "@/lib/format";

/** "Added by Ann · 3 days ago — edited by Bob · just now" */
export function Attribution({ value, compact = false }: { value: Attr; compact?: boolean }) {
  const edited = value.updatedAt !== value.createdAt || value.updatedBy?.id !== value.createdBy?.id;
  return (
    <p className="text-xs text-stone-500 dark:text-stone-400">
      <span title={dateTime(value.createdAt)}>
        Added by <UserName u={value.createdBy} /> {compact ? "" : "· "}
        {!compact && relativeTime(value.createdAt)}
      </span>
      {edited && (
        <span title={dateTime(value.updatedAt)}>
          {" "}— last edited by <UserName u={value.updatedBy} /> · {relativeTime(value.updatedAt)}
        </span>
      )}
    </p>
  );
}

export function UserName({ u }: { u: Attr["createdBy"] }) {
  return <span className="font-medium text-stone-700 dark:text-stone-300">{userLabel(u)}</span>;
}
