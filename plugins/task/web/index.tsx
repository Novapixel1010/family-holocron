// Web half of the Research Tasks plugin (see ../plugin.go). Slots appear only while
// the server reports the "task" plugin as enabled in Settings → Plugins.
import { registerWebPlugin } from "@/plugins/registry";
import { PersonTasksTab } from "./PersonTasksTab";
import { TasksPage } from "./TasksPage";

registerWebPlugin({
  id: "task",
  nav: [{ id: "tasks", label: "Research tasks", icon: "☑", order: 40, path: "", component: TasksPage }],
  personTabs: [{ id: "tasks", label: "Tasks", order: 60, component: PersonTasksTab }],
});
