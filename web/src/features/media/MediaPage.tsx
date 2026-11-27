import { useEffect, useState } from "react";
import { useMediaList } from "@/api/hooks";
import type { Media } from "@/api/types";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/Page";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/States";
import { MediaGrid, MediaModal, UploadButton } from "./MediaGrid";

const PAGE = 48;

export function MediaPage() {
  const { canEdit } = useAuth();
  const [text, setText] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);
  const [open, setOpen] = useState<Media | null>(null);
  useEffect(() => {
    const t = setTimeout(() => { setQ(text); setPage(0); }, 250);
    return () => clearTimeout(t);
  }, [text]);
  const list = useMediaList({ q, limit: PAGE, offset: page * PAGE });
  return (
    <PageContainer wide>
      <PageHeader title="Media" subtitle={list.data ? `${list.data.total} item${list.data.total === 1 ? "" : "s"}` : undefined} actions={canEdit && <UploadButton />} />
      <input type="search" aria-label="Search media" className={`${inputClass} mb-4 max-w-sm`} placeholder="Search titles, file names, descriptions…" value={text} onChange={(e) => setText(e.target.value)} />
      {list.isPending ? (
        <LoadingState label="Loading media…" />
      ) : list.isError ? (
        <ErrorState error={list.error} onRetry={() => list.refetch()} />
      ) : list.data.items.length === 0 ? (
        q ? <EmptyState icon="⌕" title="No matches" action={<Button onClick={() => setText("")}>Clear search</Button>}>Nothing matches “{q}”.</EmptyState>
          : <EmptyState icon="▣" title="No media yet" action={canEdit && <UploadButton label="Upload the first file" />}>Photos, letters, certificates and recordings you upload appear here.</EmptyState>
      ) : (
        <>
          <div className={list.isFetching ? "opacity-70" : ""}><MediaGrid items={list.data.items} onOpen={setOpen} /></div>
          {list.data.total > PAGE && (
            <div className="mt-4 flex items-center justify-between text-sm">
              <Button size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>← Previous</Button>
              <span className="text-stone-500">Page {page + 1} of {Math.ceil(list.data.total / PAGE)}</span>
              <Button size="sm" disabled={(page + 1) * PAGE >= list.data.total} onClick={() => setPage(page + 1)}>Next →</Button>
            </div>
          )}
        </>
      )}
      {open && <MediaModal media={open} canEdit={canEdit} onClose={() => setOpen(null)} />}
    </PageContainer>
  );
}
