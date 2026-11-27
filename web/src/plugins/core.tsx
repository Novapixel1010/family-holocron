// Core person-profile tabs, registered through the same slot API plugins use.
import { AboutTab } from "@/features/person/tabs/AboutTab";
import { EventsTab } from "@/features/person/tabs/EventsTab";
import { FamilyTab } from "@/features/person/tabs/FamilyTab";
import { HistoryTab } from "@/features/person/tabs/HistoryTab";
import { MediaTab } from "@/features/person/tabs/MediaTab";
import { NotesTab } from "@/features/person/tabs/NotesTab";
import { CORE_PLUGIN_ID, registerWebPlugin } from "./registry";

registerWebPlugin({
  id: CORE_PLUGIN_ID,
  personTabs: [
    { id: "about", label: "About", order: 10, component: AboutTab },
    { id: "events", label: "Facts & events", order: 20, component: EventsTab },
    { id: "family", label: "Family", order: 30, component: FamilyTab },
    { id: "notes", label: "Notes", order: 40, component: NotesTab },
    { id: "media", label: "Media", order: 50, component: MediaTab },
    { id: "history", label: "History", order: 90, component: HistoryTab },
  ],
});
