import type { ReactNode } from "react";
import { ApiError } from "@/api/client";

export function Spinner({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg className={`animate-spin ${className}`} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" className="opacity-25" />
      <path d="M22 12a10 10 0 0 0-10-10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

export function LoadingState({ label = "Loading…", className = "" }: { label?: string; className?: string }) {
  return (
    <div role="status" className={`flex items-center justify-center gap-3 py-12 text-stone-500 dark:text-stone-400 ${className}`}>
      <Spinner />
      <span>{label}</span>
    </div>
  );
}

export function EmptyState({ icon = "❦", title, children, action }: { icon?: string; title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-stone-300 px-6 py-12 text-center dark:border-stone-700">
      <div className="mb-3 text-3xl text-stone-400" aria-hidden="true">{icon}</div>
      <h3 className="font-semibold text-stone-800 dark:text-stone-100">{title}</h3>
      {children && <div className="mt-1 max-w-md text-sm text-stone-500 dark:text-stone-400">{children}</div>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return "Something went wrong.";
}

export function ErrorState({ error, onRetry, title = "Couldn't load this" }: { error: unknown; onRetry?: () => void; title?: string }) {
  const notFound = error instanceof ApiError && error.status === 404;
  return (
    <div role="alert" className="rounded-xl border border-red-200 bg-red-50 px-5 py-6 text-center dark:border-red-900/60 dark:bg-red-950/40">
      <h3 className="font-semibold text-red-800 dark:text-red-200">{notFound ? "Not found" : title}</h3>
      <p className="mt-1 text-sm text-red-700 dark:text-red-300">{errorMessage(error)}</p>
      {onRetry && !notFound && (
        <button onClick={onRetry} className="mt-3 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-red-700 ring-1 ring-red-200 hover:bg-red-100 dark:bg-red-900/40 dark:text-red-100 dark:ring-red-800">
          Try again
        </button>
      )}
    </div>
  );
}

export function InlineError({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/50 dark:text-red-300">
      {errorMessage(error)}
    </p>
  );
}

/** Renders loading / error / content for a react-query result. */
export function QueryBoundary<T>({
  query, children, loading, errorTitle,
}: {
  query: { data: T | undefined; isPending: boolean; isError: boolean; error: unknown; refetch: () => unknown };
  children: (data: T) => ReactNode;
  loading?: ReactNode;
  errorTitle?: string;
}) {
  if (query.isPending) return <>{loading ?? <LoadingState />}</>;
  if (query.isError || query.data === undefined) return <ErrorState error={query.error} onRetry={() => query.refetch()} title={errorTitle} />;
  return <>{children(query.data)}</>;
}
