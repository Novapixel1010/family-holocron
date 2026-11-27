import { useState } from "react";
import { usePlugins, useSetPlugin } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { Badge, Card } from "@/components/ui/Page";
import { EmptyState, errorMessage, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

export function PluginsSection() {
  const { isAdmin } = useAuth();
  const plugins = usePlugins();
  const set = useSetPlugin();
  // The switch reflects the click synchronously; the cache (nav items, tabs) follows optimistically.
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const toast = useToast();
  return (
    <Card>
      <h2 className="font-semibold">Plugins</h2>
      <p className="mb-4 mt-1 text-sm text-stone-500">Optional modules that add pages and profile tabs without changing your core records. Their data is included in backups.</p>
      <div role="note" className="mb-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-900 ring-1 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-200 dark:ring-amber-900">
        <p className="font-semibold">Only use plugins from sources you trust.</p>
        <p className="mt-1">
          Plugins are compiled into this server and run with the same access as Family Holocron itself — they can read and change your family’s records.
          The plugins below shipped with this build. Turning one <strong>off</strong> hides its pages and tabs and blocks its API immediately; its data is kept
          (and still included in backups), so turning it back on restores everything. Your choice is saved and survives restarts.
        </p>
      </div>
      <QueryBoundary query={plugins}>
        {(list) => list.length === 0 ? <EmptyState icon="⧉" title="No plugins installed">This build of Family Holocron has no optional plugins.</EmptyState> : (
          <ul className="divide-y divide-stone-200 dark:divide-stone-800">
            {list.map((p) => (
              <li key={p.id} className="flex items-start gap-4 py-3">
                <div className="min-w-0 flex-1">
                  <div className="font-medium">{p.name} <span className="text-xs text-stone-500">v{p.version}</span> {p.locked && <Badge tone="amber">disabled by server config</Badge>}</div>
                  <p className="text-sm text-stone-500">{p.description}</p>
                  {p.slots.length > 0 && <p className="mt-1 text-xs text-stone-400">Adds: {p.slots.map((s) => `${s.label} (${s.kind})`).join(", ")}</p>}
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" className="h-4 w-4 rounded" checked={pending[p.id] ?? p.enabled} disabled={!isAdmin || p.locked || p.id in pending}
                    onChange={async (e) => {
                      const enabled = e.target.checked;
                      setPending((m) => ({ ...m, [p.id]: enabled }));
                      try { await set.mutateAsync({ id: p.id, enabled }); toast(`${p.name} turned ${enabled ? "on" : "off"}`); }
                      catch (err) { toast(errorMessage(err), "error"); }
                      finally { setPending(({ [p.id]: _done, ...rest }) => rest); }
                    }} />
                  <span className="sr-only">{p.name}: </span>{(pending[p.id] ?? p.enabled) ? "On" : "Off"}
                </label>
              </li>
            ))}
          </ul>
        )}
      </QueryBoundary>
      {!isAdmin && <p className="mt-3 text-xs text-stone-500">Only admins can turn plugins on or off.</p>}
    </Card>
  );
}
