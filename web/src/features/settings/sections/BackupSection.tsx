import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { download } from "@/api/client";
import { useBackupMutations, useBackups, useStats } from "@/api/hooks";
import type { BackupInfo, Stats } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { Badge, Card } from "@/components/ui/Page";
import { EmptyState, errorMessage, InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { dateTime, fileSize, relativeTime } from "@/lib/format";

const kindBadge: Record<BackupInfo["kind"], { label: string; tone: "brand" | "amber" | "stone" }> = {
  manual: { label: "Backup", tone: "brand" },
  safety: { label: "Pre-restore snapshot", tone: "amber" },
  upload: { label: "Uploaded", tone: "stone" },
};

export function BackupSection() {
  const backups = useBackups();
  const m = useBackupMutations();
  const toast = useToast();
  const fileInput = useRef<HTMLInputElement>(null);
  const [downloading, setDownloading] = useState(false);
  const [restoring, setRestoring] = useState<BackupInfo | null>(null);
  const [deleting, setDeleting] = useState<BackupInfo | null>(null);
  const qc = useQueryClient();

  const downloadNew = async () => {
    setDownloading(true);
    try {
      const name = await download("/api/v1/backups/download", "holocron-backup.tar.gz");
      toast(`Downloaded ${name}`);
      qc.invalidateQueries({ queryKey: ["backups"] });
    } catch (e) {
      toast(`Backup failed: ${errorMessage(e)}`, "error");
    } finally {
      setDownloading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="font-semibold">Create a backup</h2>
        <p className="mb-4 mt-1 text-sm text-stone-500">
          A backup is one <code>.tar.gz</code> archive with the complete database, every photo and document, and a portable <code>export.json</code> you can read without Family Holocron.
          Store a copy somewhere safe, away from this server.
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="primary" onClick={downloadNew} loading={downloading}>Download a new backup</Button>
          <Button loading={m.create.isPending} onClick={async () => {
            try { const b = await m.create.mutateAsync(); toast(`Saved ${b.name} on the server`); } catch (e) { toast(errorMessage(e), "error"); }
          }}>Save a backup on the server</Button>
        </div>
      </Card>

      <Card>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="font-semibold">Restore</h2>
            <p className="text-sm text-stone-500">Upload an archive, or pick one stored on the server. Archives are fully checked before anything changes.</p>
          </div>
          <input ref={fileInput} type="file" accept=".gz,.tgz,application/gzip" className="hidden" onChange={async (e) => {
            const f = e.target.files?.[0];
            e.target.value = "";
            if (!f) return;
            try {
              const b = await m.upload.mutateAsync(f);
              toast(`${f.name} checked and ready to restore`);
              setRestoring(b);
            } catch { /* inline below */ }
          }} />
          <Button onClick={() => fileInput.current?.click()} loading={m.upload.isPending}>{m.upload.isPending ? "Checking archive…" : "Upload backup archive"}</Button>
        </div>
        <InlineError error={m.upload.error} />
        <QueryBoundary query={backups} errorTitle="Couldn't list backups">
          {(list) => list.length === 0 ? (
            <EmptyState icon="⛁" title="No backups stored on the server">Backups you save here, uploaded archives, and automatic pre-restore snapshots will be listed.</EmptyState>
          ) : (
            <ul className="divide-y divide-stone-200 dark:divide-stone-800">
              {list.map((b) => (
                <li key={b.name} className="flex flex-wrap items-center gap-3 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate font-mono text-sm">{b.name}</span>
                      <Badge tone={kindBadge[b.kind].tone}>{kindBadge[b.kind].label}</Badge>
                    </div>
                    <div className="text-xs text-stone-500" title={dateTime(b.createdAt)}>
                      {relativeTime(b.createdAt)} · {fileSize(b.sizeBytes)}
                      {b.manifest && ` · ${b.manifest.counts.persons} people, ${b.manifest.counts.media} files · by ${b.manifest.createdBy || "system"}`}
                    </div>
                  </div>
                  <BackupDownload b={b} />
                  <Button size="sm" onClick={() => setRestoring(b)}>Restore…</Button>
                  <Button size="sm" variant="ghost" className="text-red-700 dark:text-red-400" onClick={() => setDeleting(b)}>Delete</Button>
                </li>
              ))}
            </ul>
          )}
        </QueryBoundary>
      </Card>
      {restoring && <RestoreModal backup={restoring} onClose={() => setRestoring(null)} />}
      <ConfirmDialog open={!!deleting} title="Delete this backup?" onClose={() => { setDeleting(null); m.remove.reset(); }} loading={m.remove.isPending}
        error={<InlineError error={m.remove.error} />}
        onConfirm={async () => {
          try { await m.remove.mutateAsync(deleting!.name); toast("Backup deleted"); setDeleting(null); } catch { /* inline */ }
        }}>
        <p><span className="font-mono">{deleting?.name}</span> will be permanently removed from the server. Copies you downloaded are not affected.</p>
      </ConfirmDialog>
    </div>
  );
}

function BackupDownload({ b }: { b: BackupInfo }) {
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  return (
    <Button size="sm" variant="ghost" loading={busy} onClick={async () => {
      setBusy(true);
      try { await download(`/api/v1/backups/${encodeURIComponent(b.name)}/download`, b.name); } catch (e) { toast(errorMessage(e), "error"); } finally { setBusy(false); }
    }}>Download</Button>
  );
}

function CountsTable({ current, incoming }: { current?: Stats; incoming?: Stats }) {
  const rows: [keyof Stats, string][] = [["persons", "People"], ["relationships", "Relationships"], ["events", "Events"], ["notes", "Notes"], ["media", "Media files"], ["users", "Users"]];
  return (
    <table className="w-full text-sm">
      <thead><tr className="text-left text-xs uppercase text-stone-500"><th className="py-1">Records</th><th>Now</th><th>After restore</th></tr></thead>
      <tbody>
        {rows.map(([k, label]) => (
          <tr key={k} className="border-t border-stone-100 dark:border-stone-800">
            <td className="py-1">{label}</td>
            <td className="tabular-nums">{current?.[k] ?? "—"}</td>
            <td className={`tabular-nums font-medium ${current && incoming && incoming[k] < current[k] ? "text-amber-700 dark:text-amber-400" : ""}`}>{incoming?.[k] ?? "unknown"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RestoreModal({ backup, onClose }: { backup: BackupInfo; onClose: () => void }) {
  const [confirm, setConfirm] = useState("");
  const stats = useStats();
  const { restore } = useBackupMutations();
  const toast = useToast();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const ready = confirm === "RESTORE";
  return (
    <Modal open onClose={restore.isPending ? () => {} : onClose} title="Restore from backup" size="lg"
      footer={
        <>
          <Button onClick={onClose} disabled={restore.isPending}>Cancel</Button>
          <Button variant="danger" disabled={!ready} loading={restore.isPending} onClick={async () => {
            try {
              const res = await restore.mutateAsync(backup.name);
              toast(`Restored ${backup.name}. A safety snapshot (${res.result.safetySnapshot}) was saved first.`);
              onClose();
              if (!res.result.sessionKept) {
                qc.clear();
                navigate("/auth", { replace: true });
              } else {
                navigate("/settings/backup", { replace: true });
              }
            } catch { /* inline */ }
          }}>Replace all data</Button>
        </>
      }>
      <div className="space-y-4 text-sm">
        <div className="rounded-lg bg-amber-50 p-3 text-amber-900 ring-1 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-200 dark:ring-amber-900">
          <p className="font-semibold">This replaces everything currently in Family Holocron.</p>
          <p className="mt-1">All people, relationships, events, notes, media, users and history will be swapped for the contents of the backup. Before anything changes, a
            <strong> pre-restore safety snapshot</strong> of the current data is saved on the server so you can undo this. Other signed-in users will be signed out.</p>
        </div>
        <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-1">
          <dt className="text-stone-500">Archive</dt><dd className="break-all font-mono">{backup.name}</dd>
          {backup.manifest && (
            <>
              <dt className="text-stone-500">Created</dt><dd>{dateTime(backup.manifest.createdAt)} by {backup.manifest.createdBy || "system"}</dd>
              <dt className="text-stone-500">From version</dt><dd>{backup.manifest.appVersion} (schema v{backup.manifest.schemaVersion})</dd>
            </>
          )}
        </dl>
        <CountsTable current={stats.data} incoming={backup.manifest?.counts} />
        <div className="space-y-1">
          <label htmlFor="restore-confirm" className="block font-medium">Type <span className="font-mono">RESTORE</span> to confirm</label>
          <input id="restore-confirm" className={`${inputClass} font-mono`} autoComplete="off" value={confirm} onChange={(e) => setConfirm(e.target.value)} disabled={restore.isPending} />
        </div>
        {restore.isPending && <p className="text-stone-500">Restoring… this can take a while for large media libraries. Don’t close this page.</p>}
        <InlineError error={restore.error} />
      </div>
    </Modal>
  );
}
