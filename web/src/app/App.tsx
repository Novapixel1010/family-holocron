import { useEffect } from "react";
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
import { LoginPage } from "@/features/auth/LoginPage";
import { HomePage } from "@/features/home/HomePage";
import { PeoplePage } from "@/features/people/PeoplePage";
import { NewPersonPage, EditPersonPage } from "@/features/people/PersonFormPage";
import { PersonPage } from "@/features/person/PersonPage";
import { TreePage } from "@/features/tree/TreePage";
import { TimelinePage } from "@/features/timeline/TimelinePage";
import { MediaPage } from "@/features/media/MediaPage";
import { SettingsPage } from "@/features/settings/SettingsPage";
import { NotFoundPage } from "./NotFoundPage";

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
      navigate(`/login?next=${encodeURIComponent(window.location.pathname + window.location.search)}`, { replace: true });
    };
    window.addEventListener("holocron:unauthorized", onUnauthorized);
    return () => window.removeEventListener("holocron:unauthorized", onUnauthorized);
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
  if (loc.pathname === "/login") return <LoginPage />;
  if (me.isPending) return <LoadingState label="Signing you in…" className="h-full" />;
  if (me.isError || !me.data) {
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to={`/login?next=${encodeURIComponent(loc.pathname + loc.search)}`} replace />;
    }
    return (
      <div className="mx-auto max-w-md p-6 pt-24">
        <ErrorState error={me.error} onRetry={() => me.refetch()} />
      </div>
    );
  }
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
          <Route path="tree" element={<TreePage />} />
          <Route path="tree/:id" element={<TreePage />} />
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
