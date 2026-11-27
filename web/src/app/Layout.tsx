import { useState, type ReactNode } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { useStatus } from "@/api/hooks";
import { usePluginSlots } from "@/plugins/usePluginSlots";
import { Avatar } from "@/components/ui/Avatar";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/components/ui/States";
import { useAuth } from "./auth";

const core = [
  { to: "/", label: "Home", icon: "⌂", end: true },
  { to: "/people", label: "People", icon: "☺" },
  { to: "/tree", label: "Family tree", icon: "⟟" },
  { to: "/timeline", label: "Timeline", icon: "⌛" },
  { to: "/media", label: "Media", icon: "▣" },
];

function NavItem({ to, icon, end, children, onClick }: { to: string; icon: string; end?: boolean; children: ReactNode; onClick?: () => void }) {
  return (
    <NavLink
      to={to}
      end={end}
      onClick={onClick}
      className={({ isActive }) =>
        `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition ${
          isActive ? "bg-brand-100 text-brand-800 dark:bg-brand-900/50 dark:text-brand-100" : "text-stone-600 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-800"
        }`
      }
    >
      <span className="w-5 text-center text-base" aria-hidden="true">{icon}</span>
      {children}
    </NavLink>
  );
}

export function Layout() {
  const { user } = useAuth();
  const status = useStatus();
  const slots = usePluginSlots();
  const [open, setOpen] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const close = () => setOpen(false);

  const signOut = async () => {
    setSigningOut(true);
    try {
      await api.post("/api/v1/auth/logout");
      qc.clear();
      navigate("/login", { replace: true });
    } catch (e) {
      toast(errorMessage(e), "error");
      setSigningOut(false);
    }
  };

  const nav = (
    <nav className="flex h-full flex-col gap-1 p-3" aria-label="Main">
      <div className="mb-4 flex items-center gap-2 px-2 pt-1">
        <img src="/favicon.svg" alt="" className="h-8 w-8" />
        <span className="truncate font-serif text-lg font-semibold">{status.data?.instanceName ?? "Family Holocron"}</span>
      </div>
      {core.map((n) => (
        <NavItem key={n.to} to={n.to} icon={n.icon} end={n.end} onClick={close}>
          {n.label}
        </NavItem>
      ))}
      {slots.nav.length > 0 && <div className="mx-3 my-2 border-t border-stone-200 dark:border-stone-800" />}
      {slots.nav.map((n) => (
        <NavItem key={`${n.pluginId}/${n.id}`} to={n.href} icon={n.icon} onClick={close}>
          {n.label}
        </NavItem>
      ))}
      <div className="mt-auto space-y-1 border-t border-stone-200 pt-3 dark:border-stone-800">
        <NavItem to="/settings" icon="⚙" onClick={close}>Settings</NavItem>
        <div className="flex items-center gap-2 px-3 py-2">
          <Avatar name={user.displayName || user.username} size={32} />
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium">{user.displayName || user.username}</div>
            <div className="text-xs capitalize text-stone-500">{user.role}</div>
          </div>
          <button onClick={signOut} disabled={signingOut} className="rounded-md px-2 py-1 text-xs text-stone-500 hover:bg-stone-100 hover:text-stone-800 disabled:opacity-50 dark:hover:bg-stone-800 dark:hover:text-stone-200">
            {signingOut ? "Signing out…" : "Sign out"}
          </button>
        </div>
        {status.data && <div className="px-3 text-[11px] text-stone-400">v{status.data.version}</div>}
      </div>
    </nav>
  );

  return (
    <div className="flex h-full">
      <aside className="hidden w-60 shrink-0 border-r border-stone-200 bg-white dark:border-stone-800 dark:bg-stone-900 lg:block">{nav}</aside>
      {open && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-stone-900/40" onClick={close} />
          <aside className="absolute inset-y-0 left-0 w-64 bg-white shadow-xl dark:bg-stone-900">{nav}</aside>
        </div>
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center gap-3 border-b border-stone-200 bg-white px-4 py-2 dark:border-stone-800 dark:bg-stone-900 lg:hidden">
          <button onClick={() => setOpen(true)} className="rounded-md p-2 hover:bg-stone-100 dark:hover:bg-stone-800" aria-label="Open menu">☰</button>
          <span className="font-serif font-semibold">{status.data?.instanceName ?? "Family Holocron"}</span>
        </header>
        <main className="min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export function PageContainer({ children, wide = false }: { children: ReactNode; wide?: boolean }) {
  return <div className={`mx-auto w-full px-4 py-6 sm:px-6 lg:py-8 ${wide ? "max-w-7xl" : "max-w-5xl"}`}>{children}</div>;
}
