import { useState, type FormEvent } from "react";
import { ApiError } from "@/api/client";
import { useUserMutations, useUsers } from "@/api/hooks";
import type { User } from "@/api/types";
import { useAuth } from "@/app/auth";
import { Button } from "@/components/ui/Button";
import { inputClass, Select, TextInput } from "@/components/ui/Field";
import { Modal } from "@/components/ui/Modal";
import { Badge, Card } from "@/components/ui/Page";
import { EmptyState, InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { relativeTime } from "@/lib/format";

const ROLES = [
  { value: "viewer", label: "Viewer — can browse" },
  { value: "editor", label: "Editor — can add & edit" },
  { value: "admin", label: "Admin — full control" },
];

export function UsersSection() {
  const users = useUsers();
  const [creating, setCreating] = useState(false);
  const [resetting, setResetting] = useState<User | null>(null);
  return (
    <Card>
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2 className="font-semibold">Users</h2>
          <p className="text-sm text-stone-500">Invite relatives and choose what they can do.</p>
        </div>
        <Button variant="primary" onClick={() => setCreating(true)}>+ Add user</Button>
      </div>
      <QueryBoundary query={users}>
        {(list) => list.length === 0 ? <EmptyState title="No users" /> : (
          <ul className="divide-y divide-stone-200 dark:divide-stone-800">{list.map((u) => <UserRow key={u.id} u={u} onReset={() => setResetting(u)} />)}</ul>
        )}
      </QueryBoundary>
      {creating && <CreateUserModal onClose={() => setCreating(false)} />}
      {resetting && <ResetPasswordModal user={resetting} onClose={() => setResetting(null)} />}
    </Card>
  );
}

function UserRow({ u, onReset }: { u: User; onReset: () => void }) {
  const { user: me } = useAuth();
  const { update } = useUserMutations();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const patch = async (p: { role?: string; status?: string }, msg: string) => {
    setBusy(true);
    try { await update.mutateAsync({ id: u.id, ...p }); toast(msg); } catch (e) { toast(e instanceof ApiError ? e.message : "Update failed", "error"); } finally { setBusy(false); }
  };
  return (
    <li className="flex flex-wrap items-center gap-3 py-3">
      <div className="min-w-0 flex-1">
        <div className="font-medium">{u.displayName || u.username} {u.id === me.id && <Badge>you</Badge>} {u.status === "disabled" && <Badge tone="red">disabled</Badge>}</div>
        <div className="text-xs text-stone-500">@{u.username}{u.email && ` · ${u.email}`} · joined {relativeTime(u.createdAt)}</div>
      </div>
      <select aria-label={`Role for ${u.username}`} className={`${inputClass} w-auto`} value={u.role} disabled={busy}
        onChange={(e) => patch({ role: e.target.value }, `${u.username} is now ${e.target.value === "admin" ? "an" : "a"} ${e.target.value}`)}>
        {ROLES.map((r) => <option key={r.value} value={r.value}>{r.value[0].toUpperCase() + r.value.slice(1)}</option>)}
      </select>
      <Button size="sm" onClick={onReset} disabled={busy}>Reset password</Button>
      {u.id !== me.id && (
        <Button size="sm" variant={u.status === "active" ? "ghost" : "secondary"} disabled={busy}
          onClick={() => patch({ status: u.status === "active" ? "disabled" : "active" }, u.status === "active" ? `${u.username} disabled and signed out` : `${u.username} re-enabled`)}>
          {u.status === "active" ? "Disable" : "Enable"}
        </Button>
      )}
    </li>
  );
}

function CreateUserModal({ onClose }: { onClose: () => void }) {
  const [f, setF] = useState({ username: "", displayName: "", email: "", password: "", role: "editor" });
  const { create } = useUserMutations();
  const toast = useToast();
  const fields = create.error instanceof ApiError ? create.error.fields : {};
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try { await create.mutateAsync(f); toast(`User ${f.username} created`); onClose(); } catch { /* inline */ }
  };
  return (
    <Modal open onClose={onClose} title="Add a user">
      <form onSubmit={submit} className="space-y-4">
        <TextInput label="Username" required value={f.username} onChange={(e) => setF({ ...f, username: e.target.value })} error={fields.username} autoComplete="off" />
        <TextInput label="Display name" value={f.displayName} onChange={(e) => setF({ ...f, displayName: e.target.value })} />
        <TextInput label="Email (optional)" type="email" value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} />
        <TextInput label="Temporary password" type="password" required minLength={8} autoComplete="new-password" value={f.password} onChange={(e) => setF({ ...f, password: e.target.value })} error={fields.password} hint="Share it privately; they can change it under Your account." />
        <Select label="Role" value={f.role} onChange={(e) => setF({ ...f, role: e.target.value })} options={ROLES} error={fields.role} />
        <InlineError error={create.error} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" loading={create.isPending}>Create user</Button>
        </div>
      </form>
    </Modal>
  );
}

function ResetPasswordModal({ user, onClose }: { user: User; onClose: () => void }) {
  const [password, setPassword] = useState("");
  const { update } = useUserMutations();
  const toast = useToast();
  const fields = update.error instanceof ApiError ? update.error.fields : {};
  return (
    <Modal open onClose={onClose} title={`Reset password for ${user.username}`} size="sm">
      <form className="space-y-4" onSubmit={async (e) => {
        e.preventDefault();
        try { await update.mutateAsync({ id: user.id, password }); toast("Password reset; their other sessions were signed out"); onClose(); } catch { /* inline */ }
      }}>
        <TextInput label="New password" type="password" required minLength={8} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} error={fields.password} />
        <InlineError error={update.error} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" loading={update.isPending}>Reset password</Button>
        </div>
      </form>
    </Modal>
  );
}
