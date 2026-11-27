import { usePlugins, useSetPlugin } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { Badge, Card } from "@/components/ui/Page";
import { EmptyState, errorMessage, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

export function PluginsSection() {
  const { isAdmin } = useAuth();
  const plugins = usePlugins();
  const set = useSetPlugin();
  const toast = useToast();
  return (
    <Card>
      <h2 className="font-semibold">Plugins</h2>
      <p className="mb-4 mt-1 text-sm text-stone-500">Optional modules that add pages and profile tabs without changing your core records. Their data is included in backups.</p>
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
                  <input type="checkbox" className="h-4 w-4 rounded" checked={p.enabled} disabled={!isAdmin || p.locked || set.isPending}
                    onChange={async (e) => {
                      try { await set.mutateAsync({ id: p.id, enabled: e.target.checked }); toast(`${p.name} ${e.target.checked ? "enabled" : "disabled"}`); }
                      catch (err) { toast(errorMessage(err), "error"); }
                    }} />
                  {p.enabled ? "On" : "Off"}
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
