import type { ComponentType } from "react";
import { NavLink, useParams } from "react-router-dom";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { NotFoundPage } from "@/app/NotFoundPage";
import { PageHeader } from "@/components/ui/Page";
import { usePluginSlots } from "@/plugins/usePluginSlots";
import { AccountSection } from "./sections/AccountSection";
import { ActivitySection } from "./sections/ActivitySection";
import { BackupSection } from "./sections/BackupSection";
import { BioTemplatesSection } from "./sections/BioTemplatesSection";
import { GeneralSection } from "./sections/GeneralSection";
import { ImportExportSection } from "./sections/ImportExportSection";
import { PluginsSection } from "./sections/PluginsSection";
import { UsersSection } from "./sections/UsersSection";

interface Section {
  id: string;
  label: string;
  adminOnly?: boolean;
  component: ComponentType;
}

const core: Section[] = [
  { id: "account", label: "Your account", component: AccountSection },
  { id: "general", label: "General", adminOnly: true, component: GeneralSection },
  { id: "users", label: "Users", adminOnly: true, component: UsersSection },
  { id: "bio-templates", label: "Bio templates", adminOnly: true, component: BioTemplatesSection },
  { id: "backup", label: "Backup & restore", adminOnly: true, component: BackupSection },
  { id: "import-export", label: "Import & export", component: ImportExportSection },
  { id: "activity", label: "Activity log", component: ActivitySection },
  { id: "plugins", label: "Plugins", component: PluginsSection },
];

export function SettingsPage() {
  const { isAdmin } = useAuth();
  const slots = usePluginSlots();
  const sectionId = useParams().section ?? "account";
  const sections = [...core, ...slots.settingsSections].filter((s) => !s.adminOnly || isAdmin);
  const active = sections.find((s) => s.id === sectionId);
  if (!active) return <NotFoundPage />;
  const C = active.component;
  return (
    <PageContainer wide>
      <PageHeader title="Settings" />
      <div className="flex flex-col gap-6 md:flex-row">
        <nav aria-label="Settings sections" className="flex shrink-0 gap-1 overflow-x-auto md:w-52 md:flex-col">
          {sections.map((s) => (
            <NavLink key={s.id} to={`/settings/${s.id}`}
              className={() => `whitespace-nowrap rounded-lg px-3 py-2 text-sm font-medium ${s.id === active.id ? "bg-brand-100 text-brand-800 dark:bg-brand-900/50 dark:text-brand-100" : "text-stone-600 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-800"}`}>
              {s.label}
            </NavLink>
          ))}
        </nav>
        <div className="min-w-0 flex-1"><C /></div>
      </div>
    </PageContainer>
  );
}
