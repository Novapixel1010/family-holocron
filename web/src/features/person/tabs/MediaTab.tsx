import { useState } from "react";
import { toPersonInput, useMediaList, useUpdatePerson } from "@/api/hooks";
import type { Media } from "@/api/types";
import { EmptyState, QueryBoundary } from "@/components/ui/States";
import { MediaGrid, MediaModal, UploadButton } from "@/features/media/MediaGrid";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";

export function MediaTab({ person, canEdit }: PersonTabProps) {
  const list = useMediaList({ personId: person.id, limit: 200 });
  const [open, setOpen] = useState<Media | null>(null);
  const update = useUpdatePerson();
  return (
    <div className="space-y-4">
      {canEdit && <div className="flex justify-end"><UploadButton personIds={[person.id]} label="Upload photos & documents" /></div>}
      <QueryBoundary query={list} errorTitle="Couldn't load media">
        {(page) =>
          page.items.length === 0 ? (
            <EmptyState icon="▣" title="No photos or documents">
              {canEdit ? `Upload photos, letters or certificates of ${personName(person)}. Images can become their profile photo.` : "Nothing has been tagged with this person yet."}
            </EmptyState>
          ) : (
            <MediaGrid items={page.items} onOpen={setOpen} />
          )
        }
      </QueryBoundary>
      {open && (
        <MediaModal media={open} canEdit={canEdit} onClose={() => setOpen(null)}
          portraitFor={{ person, onSet: () => update.mutateAsync({ id: person.id, input: { ...toPersonInput(person), portraitMediaId: open.id } }) }} />
      )}
    </div>
  );
}
