import type { ReactNode } from "react";

export function AuthShell({ title, subtitle, children }: { title: string; subtitle: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-full items-center justify-center bg-gradient-to-b from-brand-50 to-stone-50 px-4 py-12 dark:from-stone-900 dark:to-stone-950">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <img src="/favicon.svg" alt="" className="mx-auto h-14 w-14" />
          <h1 className="mt-4 font-serif text-2xl font-semibold">{title}</h1>
          <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">{subtitle}</p>
        </div>
        <div className="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">{children}</div>
      </div>
    </div>
  );
}
