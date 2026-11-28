import { lazy, Suspense, useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/api/client";
import { useMe, useStatus } from "@/api/hooks";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { ToastProvider } from "@/components/ui/Toast";
import { usePluginSlots } from "@/plugins/usePluginSlots";
import "@/plugins";
import { AuthProvider } from "./auth";
import { Layout } from "./Layout";
import { SetupPage } from "@/features/auth/SetupPage";
import { AuthCallbackPage } from "@/features/auth/AuthCallbackPage";
import { ChangeTemporaryPasswordPage } from "@/features/auth/ChangeTemporaryPasswordPage";
import { LoginPage } from "@/features/auth/LoginPage";
import { HomePage } from "@/features/home/HomePage";
import { PeoplePage } from "@/features/people/PeoplePage";
import { NewPersonPage, EditPersonPage } from "@/features/people/PersonFormPage";
import { PersonPage } from "@/features/person/PersonPage";
import { TimelinePage } from "@/features/timeline/TimelinePage";
import { MediaPage } from "@/features/media/MediaPage";
import { SettingsPage } from "@/features/settings/SettingsPage";
import { NotFoundPage } from "./NotFoundPage";

// The tree pulls in React Flow + Dagre; load it only when the tree route is opened.
const TreePage = lazy(() => import("@/features/tree/TreePage").then((m) => ({ default: m.TreePage })));
const treeElement = (
  <Suspense fallback={<LoadingState label="Loading the tree view…" className="h-full" />}>
    <TreePage />
  </Suspense>
);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      refetchOnWindowFocus: false,
      retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
    },
  },
});

function PluginRoutes() {
  const slots = usePluginSlots();
  const loc = useLocation();
  if (slots.loading) return <LoadingState />;
  const match = slots.nav.find((n) => loc.pathname === n.href || loc.pathname === `${n.href}/`);
  if (!match) return <NotFoundPage />;
  const C = match.component;
  return <C />;
}

function Gate() {
  const status = useStatus();
  const setupRequired = status.data?.setupRequired;
  const me = useMe(status.isSuccess && !setupRequired);
  const loc = useLocation();
  const navigate = useNavigate();
  const qc = useQueryClient();

  useEffect(() => {
    const onUnauthorized = () => {
      qc.removeQueries({ queryKey: ["me"] });
      navigate(`/auth?next=${encodeURIComponent(window.location.pathname + window.location.search)}`, { replace: true });
    };
    const onPasswordChangeRequired = () => qc.invalidateQueries({ queryKey: ["me"] });
    window.addEventListener("holocron:unauthorized", onUnauthorized);
    window.addEventListener("holocron:password-change-required", onPasswordChangeRequired);
    return () => {
      window.removeEventListener("holocron:unauthorized", onUnauthorized);
      window.removeEventListener("holocron:password-change-required", onPasswordChangeRequired);
    };
  }, [navigate, qc]);

  if (status.isPending) return <LoadingState label="Starting Family Holocron…" className="h-full" />;
  if (status.isError) {
    return (
      <div className="mx-auto max-w-md p-6 pt-24">
        <ErrorState error={status.error} onRetry={() => status.refetch()} title="Can't reach the server" />
      </div>
    );
  }
  if (setupRequired) {
    return loc.pathname === "/setup" ? <SetupPage /> : <Navigate to="/setup" replace />;
  }
  if (loc.pathname === "/setup") return <Navigate to="/" replace />;
  // Sign-in lives at /auth (memos-style); /auth/callback is reserved for identity providers.
  if (loc.pathname === "/login") return <Navigate to={`/auth${loc.search}`} replace />;
  if (loc.pathname === "/auth/callback") return <AuthCallbackPage />;
  if (loc.pathname === "/auth") return <LoginPage />;
  if (me.isPending) return <LoadingState label="Signing you in…" className="h-full" />;
  if (me.isError || !me.data) {
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to={`/auth?next=${encodeURIComponent(loc.pathname + loc.search)}`} replace />;
    }
    return (
      <div className="mx-auto max-w-md p-6 pt-24">
        <ErrorState error={me.error} onRetry={() => me.refetch()} />
      </div>
    );
  }
  // A temporary password locks every route to the change-password screen.
  if (me.data.mustChangePassword) return <ChangeTemporaryPasswordPage user={me.data} />;
  return (
    <AuthProvider user={me.data}>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route path="people" element={<PeoplePage />} />
          <Route path="people/new" element={<NewPersonPage />} />
          <Route path="people/:id" element={<PersonPage />} />
          <Route path="people/:id/:tab" element={<PersonPage />} />
          <Route path="people/:id/edit" element={<EditPersonPage />} />
          <Route path="tree" element={treeElement} />
          <Route path="tree/:id" element={treeElement} />
          <Route path="timeline" element={<TimelinePage />} />
          <Route path="media" element={<MediaPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="settings/:section" element={<SettingsPage />} />
          <Route path="x/*" element={<PluginRoutes />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <BrowserRouter>
          <Gate />
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  );
}
