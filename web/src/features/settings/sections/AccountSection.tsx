import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/api/client";
import type { User } from "@/api/types";
import { useAuth } from "@/app/auth";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { Badge, Card } from "@/components/ui/Page";
import { InlineError } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

export function AccountSection() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const toast = useToast();
  const [profile, setProfile] = useState({ displayName: user.displayName, email: user.email });
  const [pErr, setPErr] = useState<unknown>(null);
  const [pBusy, setPBusy] = useState(false);
  const [pw, setPw] = useState({ currentPassword: "", newPassword: "", confirm: "" });
  const [pwErr, setPwErr] = useState<unknown>(null);
  const [pwBusy, setPwBusy] = useState(false);
  const pwFields = pwErr instanceof ApiError ? pwErr.fields : {};

  const saveProfile = async (e: FormEvent) => {
    e.preventDefault();
    setPBusy(true);
    setPErr(null);
    try {
      const u = await api.patch<User>("/api/v1/auth/me", profile);
      qc.setQueryData(["me"], u);
      qc.invalidateQueries({ queryKey: ["users"] });
      toast("Profile saved");
    } catch (err) {
      setPErr(err);
    } finally {
      setPBusy(false);
    }
  };
  const changePassword = async (e: FormEvent) => {
    e.preventDefault();
    setPwErr(null);
    if (pw.newPassword !== pw.confirm) {
      setPwErr(new ApiError(422, "validation", "The new passwords don’t match.", { confirm: "does not match" }));
      return;
    }
    setPwBusy(true);
    try {
      await api.post("/api/v1/auth/me/password", { currentPassword: pw.currentPassword, newPassword: pw.newPassword });
      setPw({ currentPassword: "", newPassword: "", confirm: "" });
      toast("Password changed. Other devices have been signed out.");
    } catch (err) {
      setPwErr(err);
    } finally {
      setPwBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="mb-1 font-semibold">Profile</h2>
        <p className="mb-4 text-sm text-stone-500">Signed in as <span className="font-medium">{user.username}</span> <Badge tone="brand">{user.role}</Badge>. Your display name is shown next to every change you make.</p>
        <form onSubmit={saveProfile} className="grid max-w-lg gap-4">
          <TextInput label="Display name" value={profile.displayName} onChange={(e) => setProfile({ ...profile, displayName: e.target.value })} />
          <TextInput label="Email" type="email" value={profile.email} onChange={(e) => setProfile({ ...profile, email: e.target.value })} />
          <InlineError error={pErr} />
          <div><Button type="submit" variant="primary" loading={pBusy}>Save profile</Button></div>
        </form>
      </Card>
      <Card>
        <h2 className="mb-4 font-semibold">Change password</h2>
        <form onSubmit={changePassword} className="grid max-w-lg gap-4">
          <TextInput label="Current password" type="password" autoComplete="current-password" required value={pw.currentPassword} onChange={(e) => setPw({ ...pw, currentPassword: e.target.value })} error={pwFields.currentPassword} />
          <TextInput label="New password" type="password" autoComplete="new-password" required minLength={8} value={pw.newPassword} onChange={(e) => setPw({ ...pw, newPassword: e.target.value })} error={pwFields.newPassword} hint="At least 8 characters." />
          <TextInput label="Confirm new password" type="password" autoComplete="new-password" required value={pw.confirm} onChange={(e) => setPw({ ...pw, confirm: e.target.value })} error={pwFields.confirm} />
          <InlineError error={pwErr} />
          <div><Button type="submit" variant="primary" loading={pwBusy}>Change password</Button></div>
        </form>
      </Card>
    </div>
  );
}
