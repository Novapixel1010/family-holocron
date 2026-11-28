import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/api/client";
import type { User } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { InlineError } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { AuthShell } from "./AuthShell";

const MIN = 8;

/**
 * Shown instead of the app while the signed-in user has an admin-assigned temporary
 * password. The server refuses every other API call until the password is changed.
 */
export function ChangeTemporaryPasswordPage({ user }: { user: User }) {
  const [form, setForm] = useState({ current: "", next: "", confirm: "" });
  const [touched, setTouched] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  // Client-side checks mirror the server's rules; server field errors still win.
  const local: Record<string, string> = {};
  if (form.next && form.next.length < MIN) local.next = `Must be at least ${MIN} characters.`;
  else if (form.next && form.next === form.current) local.next = "Must be different from your temporary password.";
  if (form.confirm && form.confirm !== form.next) local.confirm = "Doesn’t match the new password.";
  const server = error instanceof ApiError ? error.fields : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (!form.current || !form.next || !form.confirm || Object.keys(local).length) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await api.post<User>("/api/v1/auth/change-password", { currentPassword: form.current, newPassword: form.next });
      qc.setQueryData(["me"], updated); // mustChangePassword is now false: the gate unlocks
      await qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "me" });
      toast("Password updated — welcome!");
      navigate("/tree", { replace: true });
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  const signOut = async () => {
    setSigningOut(true);
    try {
      await api.post("/api/v1/auth/signout");
    } finally {
      qc.clear();
      navigate("/auth", { replace: true });
    }
  };

  const show = (field: "current" | "next" | "confirm") =>
    (touched || form[field]) ? (local[field] ?? (field === "current" ? server.currentPassword : field === "next" ? server.newPassword : undefined)) : undefined;

  return (
    <AuthShell title="Change your temporary password"
      subtitle={<>Hi {user.displayName || user.username} — an administrator set a temporary password for you. Choose your own to continue.</>}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <TextInput label="Temporary (current) password" type="password" required autoFocus autoComplete="current-password"
          value={form.current} onChange={set("current")} error={touched && !form.current ? "Enter the temporary password you were given." : show("current")} />
        <TextInput label="New password" type="password" required autoComplete="new-password" minLength={MIN}
          value={form.next} onChange={set("next")} error={touched && !form.next ? "Choose a new password." : show("next")} hint={`At least ${MIN} characters.`} />
        <TextInput label="Confirm new password" type="password" required autoComplete="new-password"
          value={form.confirm} onChange={set("confirm")} error={touched && !form.confirm ? "Re-enter the new password." : show("confirm")} />
        <InlineError error={error instanceof ApiError && Object.keys(error.fields).length ? null : error} />
        <Button type="submit" variant="primary" className="w-full" loading={busy}>Update password</Button>
        <Button variant="ghost" className="w-full" onClick={signOut} loading={signingOut}>Sign out</Button>
      </form>
    </AuthShell>
  );
}
