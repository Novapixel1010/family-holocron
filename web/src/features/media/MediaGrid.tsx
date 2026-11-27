import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError, mediaFileUrl } from "@/api/client";
import { useDeleteMedia, usePersons, useUpdateMedia, useUploadMedia } from "@/api/hooks";
import type { Media, Person } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { Button } from "@/components/ui/Button";
import { TextArea, TextInput } from "@/components/ui/Field";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { InlineError, Spinner } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { dateHint } from "@/features/people/PersonForm";
import { fileSize, personName } from "@/lib/format";

export function MediaThumb({ m, className = "" }: { m: Media; className?: string }) {
  const [broken, setBroken] = useState(false);
  if (m.mimeType.startsWith("image/") && !broken) {
    return <img src={mediaFileUrl(m.id)} alt={m.title || m.originalName} loading="lazy" onError={() => setBroken(true)} className={`h-full w-full object-cover ${className}`} />;
  }
  const icon = broken ? "⚠" : m.mimeType === "application/pdf" ? "PDF" : m.mimeType.startsWith("audio/") ? "♪" : m.mimeType.startsWith("video/") ? "▶" : "TXT";
  return <div className={`flex h-full w-full items-center justify-center bg-stone-100 text-xl font-semibold text-stone-500 dark:bg-stone-800 ${className}`}>{icon}</div>;
}

export function MediaGrid({ items, onOpen }: { items: Media[]; onOpen: (m: Media) => void }) {
  return (
    <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {items.map((m) => (
        <li key={m.id}>
          <button onClick={() => onOpen(m)} className="group block w-full overflow-hidden rounded-xl bg-white text-left shadow-sm ring-1 ring-stone-200 hover:ring-brand-400 dark:bg-stone-900 dark:ring-stone-800">
            <div className="aspect-square overflow-hidden"><MediaThumb m={m} className="transition group-hover:scale-105" /></div>
            <div className="p-2">
              <div className="truncate text-sm font-medium">{m.title || m.originalName}</div>
              <div className="text-xs text-stone-500">{m.takenDate || fileSize(m.sizeBytes)}</div>
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}

/** File picker button that uploads one or more files. */
export function UploadButton({ personIds, label = "Upload files", onUploaded }: { personIds?: number[]; label?: string; onUploaded?: (m: Media[]) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadMedia();
  const toast = useToast();
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const onFiles = async (files: FileList | null) => {
    if (!files?.length) return;
    const list = [...files];
    const ok: Media[] = [];
    const failed: string[] = [];
    setProgress({ done: 0, total: list.length });
    for (const [i, f] of list.entries()) {
      try {
        ok.push(await upload.mutateAsync({ file: f, personIds }));
      } catch (e) {
        failed.push(`${f.name}: ${e instanceof ApiError ? e.message : "upload failed"}`);
      }
      setProgress({ done: i + 1, total: list.length });
    }
    setProgress(null);
    if (input.current) input.current.value = "";
    if (ok.length) toast(`Uploaded ${ok.length} file${ok.length === 1 ? "" : "s"}`);
    if (failed.length) toast(failed.join(" · "), "error");
    onUploaded?.(ok);
  };
  return (
    <>
      <input ref={input} type="file" multiple className="hidden" accept="image/*,application/pdf,text/plain,audio/*,video/mp4,video/webm" onChange={(e) => onFiles(e.target.files)} />
      <Button variant="primary" onClick={() => input.current?.click()} disabled={!!progress}>
        {progress ? <><Spinner className="h-4 w-4" /> Uploading {progress.done}/{progress.total}…</> : label}
      </Button>
    </>
  );
}

export function MediaModal({ media, onClose, canEdit, portraitFor }: {
  media: Media; onClose: () => void; canEdit: boolean;
  portraitFor?: { person: Person; onSet: () => Promise<unknown> };
}) {
  const [form, setForm] = useState({ title: media.title, description: media.description, takenDate: media.takenDate, personIds: media.personIds });
  const [confirm, setConfirm] = useState(false);
  const update = useUpdateMedia();
  const del = useDeleteMedia();
  const toast = useToast();
  const tagged = usePersons({ ids: form.personIds.join(","), limit: 1000 }, form.personIds.length > 0);
  const people = (tagged.data?.items ?? []).filter((p) => form.personIds.includes(p.id));
  const fields = update.error instanceof ApiError ? update.error.fields : {};
  const [settingPortrait, setSettingPortrait] = useState(false);
  return (
    <Modal open onClose={onClose} title={media.title || media.originalName} size="lg">
      <div className="grid gap-5 md:grid-cols-2">
        <div className="space-y-2">
          <div className="overflow-hidden rounded-xl bg-stone-100 dark:bg-stone-800">
            {media.mimeType.startsWith("image/") ? (
              <img src={mediaFileUrl(media.id)} alt={media.title} className="max-h-[50vh] w-full object-contain" />
            ) : media.mimeType.startsWith("audio/") ? (
              <audio controls src={mediaFileUrl(media.id)} className="w-full p-4" />
            ) : media.mimeType.startsWith("video/") ? (
              <video controls src={mediaFileUrl(media.id)} className="w-full" />
            ) : (
              <div className="aspect-video"><MediaThumb m={media} /></div>
            )}
          </div>
          <div className="flex flex-wrap gap-2 text-sm">
            <a href={mediaFileUrl(media.id)} target="_blank" rel="noopener" className="text-brand-700 hover:underline dark:text-brand-300">Open original</a>
            <a href={mediaFileUrl(media.id, true)} className="text-brand-700 hover:underline dark:text-brand-300">Download</a>
            <span className="text-stone-500">{media.originalName} · {fileSize(media.sizeBytes)}</span>
          </div>
          <Attribution value={media} />
        </div>
        {canEdit ? (
          <form className="space-y-3" onSubmit={async (e) => {
            e.preventDefault();
            try { await update.mutateAsync({ id: media.id, input: form }); toast("Media details saved"); onClose(); } catch { /* inline */ }
          }}>
            <TextInput label="Title" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} error={fields.title} />
            <TextInput label="Date taken" value={form.takenDate} onChange={(e) => setForm({ ...form, takenDate: e.target.value })} hint={dateHint(form.takenDate)} />
            <TextArea label="Description" rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
            <div className="space-y-1">
              <div className="text-sm font-medium text-stone-700 dark:text-stone-300">People in this item</div>
              <div className="flex flex-wrap gap-1.5">
                {form.personIds.length === 0 && <span className="text-sm text-stone-500">Nobody tagged yet.</span>}
                {form.personIds.map((id) => {
                  const p = people.find((x) => x.id === id);
                  return (
                    <span key={id} className="inline-flex items-center gap-1 rounded-full bg-brand-100 px-2 py-0.5 text-xs text-brand-800 dark:bg-brand-900/50 dark:text-brand-200">
                      {p ? personName(p) : `#${id}`}
                      <button type="button" aria-label="Remove tag" onClick={() => setForm({ ...form, personIds: form.personIds.filter((x) => x !== id) })}>✕</button>
                    </span>
                  );
                })}
              </div>
              <PersonPicker label="Tag a person" value={null} exclude={form.personIds} onChange={(p) => p && setForm({ ...form, personIds: [...form.personIds, p.id] })} />
            </div>
            <InlineError error={update.error} />
            <div className="flex flex-wrap justify-between gap-2 pt-2">
              <Button variant="danger" onClick={() => setConfirm(true)}>Delete</Button>
              <div className="flex flex-wrap gap-2">
                {portraitFor && media.mimeType.startsWith("image/") && (
                  <Button loading={settingPortrait} onClick={async () => {
                    setSettingPortrait(true);
                    try { await portraitFor.onSet(); toast("Profile photo updated"); onClose(); } catch (e) { toast(e instanceof Error ? e.message : "Failed", "error"); } finally { setSettingPortrait(false); }
                  }}>Use as profile photo</Button>
                )}
                <Button type="submit" variant="primary" loading={update.isPending}>Save</Button>
              </div>
            </div>
          </form>
        ) : (
          <div className="space-y-2 text-sm">
            {media.takenDate && <p><span className="font-medium">Date:</span> {media.takenDate}</p>}
            {media.description && <p className="whitespace-pre-wrap">{media.description}</p>}
            <div className="flex flex-wrap gap-2">
              {people.map((p) => <Link key={p.id} to={`/people/${p.id}`} className="text-brand-700 hover:underline dark:text-brand-300" onClick={onClose}>{personName(p)}</Link>)}
            </div>
          </div>
        )}
      </div>
      <ConfirmDialog open={confirm} title="Delete this file?" onClose={() => setConfirm(false)} loading={del.isPending} error={<InlineError error={del.error} />}
        onConfirm={async () => {
          try { await del.mutateAsync(media.id); toast("File deleted"); onClose(); } catch { /* inline */ }
        }}>
        The file is removed from storage and untagged from everyone. Backups taken earlier still contain it.
      </ConfirmDialog>
    </Modal>
  );
}
