import { useState, type FormEvent } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { User } from "@/api/types";
import { useMe, useStatus } from "@/api/hooks";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { InlineError, LoadingState } from "@/components/ui/States";
import { AuthShell } from "./AuthShell";

function safeNext(next: string | null): string {
  return next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/login") && !next.startsWith("/auth") ? next : "/";
}

export function LoginPage() {
  const [params] = useSearchParams();
  const next = safeNext(params.get("next"));
  const status = useStatus();
  const me = useMe();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();

  if (me.isPending) return <LoadingState className="h-full" />;
  if (me.data) return <Navigate to={next} replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const user = await api.post<User>("/api/v1/auth/signin", { username, password });
      qc.setQueryData(["me"], user);
      await qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "me" });
      // With a temporary password the app gate shows the change-password screen first.
      navigate(user.mustChangePassword ? "/" : next, { replace: true });
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <AuthShell title={status.data?.instanceName ?? "Family Holocron"} subtitle="Sign in to explore your family’s history.">
      <form onSubmit={submit} className="space-y-4">
        <TextInput label="Username or email" required autoFocus autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} />
        <TextInput label="Password" type="password" required autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
        <InlineError error={error} />
        <Button type="submit" variant="primary" className="w-full" loading={busy}>Sign in</Button>
        <p className="text-center text-xs text-stone-500">Forgot your password? Ask your family archive’s administrator to reset it.</p>
      </form>
    </AuthShell>
  );
}
