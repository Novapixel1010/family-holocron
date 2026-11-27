import { createContext, useContext, type ReactNode } from "react";
import type { User } from "@/api/types";

interface AuthCtx {
  user: User;
  canEdit: boolean;
  isAdmin: boolean;
}

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ user, children }: { user: User; children: ReactNode }) {
  const value = { user, canEdit: user.role === "admin" || user.role === "editor", isAdmin: user.role === "admin" };
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthCtx {
  const v = useContext(Ctx);
  if (!v) throw new Error("useAuth outside AuthProvider");
  return v;
}
