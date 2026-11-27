// Web plugin registry: UI extension slots.
//
// The core registers its own person tabs here (see ./core.tsx), so a plugin
// such as family-holocron-dna can add "DNA" / "Chromosome browser" tabs and pages
// as peers, without editing core views. A web plugin is only shown when the
// server reports a plugin with the same id as enabled (GET /api/v1/plugins).
import type { ComponentType } from "react";
import type { Person } from "@/api/types";

export const CORE_PLUGIN_ID = "core";

export interface PersonTabProps {
  person: Person;
  canEdit: boolean;
}

export interface PersonTabSlot {
  id: string;
  label: string;
  order: number;
  component: ComponentType<PersonTabProps>;
}

export interface NavSlot {
  id: string;
  label: string;
  icon: string;
  order: number;
  /** Route path below /x/<pluginId>/ ("" for the plugin's index page). */
  path: string;
  component: ComponentType;
}

export interface SettingsSectionSlot {
  id: string;
  label: string;
  order: number;
  adminOnly?: boolean;
  component: ComponentType;
}

export interface WebPlugin {
  id: string;
  personTabs?: PersonTabSlot[];
  nav?: NavSlot[];
  settingsSections?: SettingsSectionSlot[];
}

const plugins = new Map<string, WebPlugin>();

export function registerWebPlugin(p: WebPlugin) {
  if (plugins.has(p.id)) throw new Error(`web plugin ${p.id} registered twice`);
  plugins.set(p.id, p);
}

export function webPlugins(): WebPlugin[] {
  return [...plugins.values()];
}
