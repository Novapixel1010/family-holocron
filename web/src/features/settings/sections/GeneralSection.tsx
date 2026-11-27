import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/api/client";
import { useSettings, useStatus } from "@/api/hooks";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { Card } from "@/components/ui/Page";
import { InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

export function GeneralSection() {
  const settings = useSettings();
  const status = useStatus();
  return (
    <div className="space-y-6">
      <QueryBoundary query={settings}>{(s) => <NameForm initial={s.instanceName} />}</QueryBoundary>
      <Card>
        <h2 className="mb-2 font-semibold">About this instance</h2>
        {status.data ? (
          <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-stone-500">Version</dt><dd>{status.data.version}</dd>
            <dt className="text-stone-500">Database schema</dt><dd>v{status.data.schemaVersion}</dd>
          </dl>
        ) : <p className="text-sm text-stone-500">Loading…</p>}
      </Card>
    </div>
  );
}

function NameForm({ initial }: { initial: string }) {
  const [name, setName] = useState(initial);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();
  const toast = useToast();
  useEffect(() => setName(initial), [initial]);
  return (
    <Card>
      <h2 className="mb-4 font-semibold">Archive name</h2>
      <form className="grid max-w-lg gap-4" onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setErr(null);
        try {
          await api.put("/api/v1/settings", { instanceName: name });
          qc.invalidateQueries({ queryKey: ["settings"] });
          qc.invalidateQueries({ queryKey: ["status"] });
          toast("Archive name saved");
        } catch (x) { setErr(x); } finally { setBusy(false); }
      }}>
        <TextInput label="Name shown in the sidebar and sign-in page" value={name} onChange={(e) => setName(e.target.value)} maxLength={80}
          error={err instanceof ApiError ? err.fields.instanceName : undefined} />
        <InlineError error={err} />
        <div><Button type="submit" variant="primary" loading={busy}>Save</Button></div>
      </form>
    </Card>
  );
}
