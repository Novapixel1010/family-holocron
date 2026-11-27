import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/api/client";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { InlineError } from "@/components/ui/States";
import { AuthShell } from "./AuthShell";

export function SetupPage() {
  const [form, setForm] = useState({ username: "", displayName: "", email: "", password: "", confirm: "" });
  const [error, setError] = useState<unknown>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (form.password !== form.confirm) {
      setFields({ confirm: "Passwords do not match" });
      return;
    }
    setBusy(true);
    try {
      await api.post("/api/v1/auth/setup", { username: form.username, displayName: form.displayName, email: form.email, password: form.password });
      await qc.invalidateQueries();
      navigate("/", { replace: true });
    } catch (err) {
      setError(err);
      setFields(err instanceof ApiError ? err.fields : {});
      if (err instanceof ApiError && err.status === 409) await qc.invalidateQueries({ queryKey: ["status"] });
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title="Welcome to Family Holocron" subtitle="Create the administrator account for this family archive. You can invite relatives afterwards.">
      <form onSubmit={submit} className="space-y-4">
        <TextInput label="Username" required autoComplete="username" value={form.username} onChange={set("username")} error={fields.username} />
        <TextInput label="Your name" autoComplete="name" value={form.displayName} onChange={set("displayName")} placeholder="Shown next to your changes" />
        <TextInput label="Email (optional)" type="email" autoComplete="email" value={form.email} onChange={set("email")} />
        <TextInput label="Password" type="password" required autoComplete="new-password" minLength={8} value={form.password} onChange={set("password")} error={fields.password} hint="At least 8 characters." />
        <TextInput label="Confirm password" type="password" required autoComplete="new-password" value={form.confirm} onChange={set("confirm")} error={fields.confirm} />
        <InlineError error={error} />
        <Button type="submit" variant="primary" className="w-full" loading={busy}>Create administrator</Button>
      </form>
    </AuthShell>
  );
}
