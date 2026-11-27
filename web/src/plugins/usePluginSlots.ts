import { useMemo } from "react";
import { usePlugins } from "@/api/hooks";
import { CORE_PLUGIN_ID, webPlugins, type NavSlot, type PersonTabSlot, type SettingsSectionSlot } from "./registry";

export interface ResolvedNav extends NavSlot {
  pluginId: string;
  href: string;
}

/** Slots from the core plus every web plugin the server reports as enabled. */
export function usePluginSlots() {
  const server = usePlugins();
  return useMemo(() => {
    const enabled = new Set((server.data ?? []).filter((p) => p.enabled).map((p) => p.id));
    enabled.add(CORE_PLUGIN_ID);
    const active = webPlugins().filter((p) => enabled.has(p.id));
    const personTabs: PersonTabSlot[] = active.flatMap((p) => p.personTabs ?? []).sort((a, b) => a.order - b.order);
    const nav: ResolvedNav[] = active
      .flatMap((p) => (p.nav ?? []).map((n) => ({ ...n, pluginId: p.id, href: `/x/${p.id}${n.path ? `/${n.path}` : ""}` })))
      .sort((a, b) => a.order - b.order);
    const settingsSections: SettingsSectionSlot[] = active.flatMap((p) => p.settingsSections ?? []).sort((a, b) => a.order - b.order);
    return { personTabs, nav, settingsSections, loading: server.isPending, error: server.error };
  }, [server.data, server.isPending, server.error]);
}
