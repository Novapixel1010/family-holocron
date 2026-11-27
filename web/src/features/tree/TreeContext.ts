import { createContext, useContext } from "react";
import type { GraphPerson, TreeDirection } from "@/api/types";

/** Actions and UI state shared with custom nodes (kept out of node data so nodes stay stable). */
export interface TreeActions {
  direction: TreeDirection;
  selectedId: number | null;
  loadingId: number | null;
  collapsed: Set<number>;
  onSelect: (p: GraphPerson) => void;
  onToggle: (p: GraphPerson) => void;
  onFocus: (p: GraphPerson) => void;
}

export const TreeActionsContext = createContext<TreeActions | null>(null);

export function useTreeActions(): TreeActions {
  const v = useContext(TreeActionsContext);
  if (!v) throw new Error("useTreeActions outside TreeActionsContext");
  return v;
}
