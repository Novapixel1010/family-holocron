import { useRef, useState, type FormEvent } from "react";
import { ApiError } from "@/api/client";
import { useAccessTokenMutations, useAccessTokens } from "@/api/hooks";
import type { AccessToken, CreatedAccessToken } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { Select, TextInput } from "@/components/ui/Field";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { Badge, Card } from "@/components/ui/Page";
import { InlineError, LoadingState, ErrorState } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { formatDateTime } from "@/lib/dates";

const EXPIRATIONS = [
  { value: "never", label: "Never" },
  { value: "7", label: "7 days" },
  { value: "30", label: "30 days" },
  { value: "90", label: "90 days" },
  { value: "365", label: "1 year" },
];

/** Settings → Your account → Access Tokens (personal access tokens, as in usememos/memos). */
export function AccessTokensCard() {
  const tokens = useAccessTokens();
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<AccessToken | null>(null);
  return (
    <Card>
      <h2 className="mb-1 font-semibold">Access Tokens</h2>
      <p className="mb-4 text-sm text-stone-500">A list of all access tokens for your account.</p>
      <div className="relative overflow-x-auto rounded-lg ring-1 ring-stone-200 dark:ring-stone-800">
        <table className="w-full min-w-[34rem] text-sm">
          <thead className="bg-stone-50 text-left text-xs uppercase tracking-wide text-stone-500 dark:bg-stone-800/50">
            <tr>
              <th scope="col" className="px-3 py-2 font-medium">Description</th>
              <th scope="col" className="px-3 py-2 font-medium">Created At</th>
              <th scope="col" className="px-3 py-2 font-medium">Expires At</th>
              <th scope="col" className="px-3 py-2"><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-stone-200 dark:divide-stone-800">
            {tokens.isPending ? (
              <tr><td colSpan={4}><LoadingState label="Loading access tokens…" className="py-6" /></td></tr>
            ) : tokens.isError ? (
              <tr><td colSpan={4} className="p-3"><ErrorState error={tokens.error} onRetry={() => tokens.refetch()} /></td></tr>
            ) : tokens.data.length === 0 ? (
              <tr><td colSpan={4} className="px-3 py-8 text-center text-stone-500">No access tokens found</td></tr>
            ) : (
              tokens.data.map((t) => <TokenRow key={t.id} t={t} onRevoke={() => setRevoking(t)} />)
            )}
          </tbody>
        </table>
      </div>
      <div className="mt-3 flex justify-end">
        <Button variant="primary" onClick={() => setCreating(true)}>+ Create</Button>
      </div>
      {creating && <CreateTokenModal onClose={() => setCreating(false)} />}
      {revoking && <RevokeTokenDialog token={revoking} onClose={() => setRevoking(null)} />}
    </Card>
  );
}

function TokenRow({ t, onRevoke }: { t: AccessToken; onRevoke: () => void }) {
  const expired = t.expiresAt !== null && new Date(t.expiresAt).getTime() <= Date.now();
  return (
    <tr>
      <td className="px-3 py-2.5 align-top">
        <div className="font-medium text-stone-900 dark:text-stone-100">{t.description}</div>
        <div className="text-xs text-stone-500">{t.lastUsedAt ? `Last used ${formatDateTime(t.lastUsedAt)}` : "Never used"}</div>
      </td>
      <td className="whitespace-nowrap px-3 py-2.5 align-top tabular-nums">{formatDateTime(t.createdAt)}</td>
      <td className="whitespace-nowrap px-3 py-2.5 align-top tabular-nums">
        {t.expiresAt ? formatDateTime(t.expiresAt) : "Never"} {expired && <Badge tone="red">Expired</Badge>}
      </td>
      <td className="px-3 py-2 text-right align-top">
        <Button size="sm" variant="ghost" className="!text-red-700 hover:!bg-red-50 dark:!text-red-400 dark:hover:!bg-red-950/40" onClick={onRevoke} aria-label={`Revoke ${t.description}`}>
          Revoke
        </Button>
      </td>
    </tr>
  );
}

function RevokeTokenDialog({ token, onClose }: { token: AccessToken; onClose: () => void }) {
  const { revoke } = useAccessTokenMutations();
  const toast = useToast();
  const [err, setErr] = useState<unknown>(null);
  const confirm = async () => {
    setErr(null);
    try {
      await revoke.mutateAsync(token.id);
      toast(`Revoked “${token.description}”`);
      onClose();
    } catch (e) {
      setErr(e);
    }
  };
  return (
    <ConfirmDialog open title="Revoke access token?" confirmLabel="Revoke" onConfirm={confirm} onClose={onClose} loading={revoke.isPending} error={<InlineError error={err} />}>
      <p>Anything using <span className="font-medium">“{token.description}”</span> will stop working immediately. This can’t be undone.</p>
    </ConfirmDialog>
  );
}

function CreateTokenModal({ onClose }: { onClose: () => void }) {
  const { create } = useAccessTokenMutations();
  const [description, setDescription] = useState("");
  const [expiration, setExpiration] = useState("30");
  const [err, setErr] = useState<unknown>(null);
  const [created, setCreated] = useState<CreatedAccessToken | null>(null);
  const fields = err instanceof ApiError ? err.fields : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr(null);
    if (!description.trim()) {
      setErr(new ApiError(422, "validation", "Add a description so you can recognize this token later.", { description: "is required" }));
      return;
    }
    try {
      setCreated(await create.mutateAsync({ description: description.trim(), expiresInDays: expiration === "never" ? null : Number(expiration) }));
    } catch (e2) {
      setErr(e2);
    }
  };

  if (created) return <RevealTokenModal token={created} onClose={onClose} />;
  return (
    <Modal
      open
      onClose={onClose}
      title="Create access token"
      size="sm"
      footer={
        <>
          <Button onClick={onClose} disabled={create.isPending}>Cancel</Button>
          <Button type="submit" form="create-access-token" variant="primary" loading={create.isPending}>Create</Button>
        </>
      }
    >
      <form id="create-access-token" onSubmit={submit} className="grid gap-4" noValidate>
        <TextInput label="Description" required maxLength={100} placeholder="e.g. GEDCOM Sync" value={description}
          onChange={(e) => setDescription(e.target.value)} error={fields.description} hint="What is this token for?" />
        <Select label="Expiration" options={EXPIRATIONS} value={expiration} onChange={(e) => setExpiration(e.target.value)} error={fields.expiresInDays} />
        <p className="text-xs text-stone-500">The token can do everything your account can, and its changes are recorded under your name.</p>
        <InlineError error={err} />
      </form>
    </Modal>
  );
}

/** Copies text, falling back to a hidden selection when the Clipboard API is unavailable (plain-HTTP LAN installs). */
async function copyText(text: string, input: HTMLInputElement | null): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* fall through */
  }
  if (!input) return false;
  input.focus();
  input.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  }
}

function RevealTokenModal({ token, onClose }: { token: CreatedAccessToken; onClose: () => void }) {
  const toast = useToast();
  const ref = useRef<HTMLInputElement>(null);
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    if (await copyText(token.token, ref.current)) {
      setCopied(true);
      toast("Access token copied");
    } else {
      ref.current?.select();
      toast("Couldn’t copy automatically — the token is selected, press Ctrl+C / ⌘C", "error");
    }
  };
  return (
    <Modal open onClose={onClose} title="Access token created" size="sm" footer={<Button variant="primary" onClick={onClose}>Done</Button>}>
      <div className="grid gap-4 text-sm">
        <p>
          Your new token <span className="font-medium">“{token.description}”</span> {token.expiresAt ? <>expires {formatDateTime(token.expiresAt)}</> : <>never expires</>}.
          Send it as <code className="rounded bg-stone-100 px-1 dark:bg-stone-800">Authorization: Bearer &lt;token&gt;</code>.
        </p>
        <div className="flex gap-2">
          <input ref={ref} readOnly aria-label="Access token" value={token.token} onFocus={(e) => e.currentTarget.select()}
            className="min-w-0 flex-1 rounded-lg border border-stone-300 bg-stone-50 px-3 py-2 font-mono text-xs text-stone-900 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-100" />
          <Button onClick={copy} variant={copied ? "secondary" : "primary"}>{copied ? "Copied" : "Copy"}</Button>
        </div>
        <div role="alert" className="rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
          Copy this token now. It won’t be shown again after you close this dialog.
        </div>
      </div>
    </Modal>
  );
}
