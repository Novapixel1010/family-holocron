import type { ReactNode } from "react";

export function PageHeader({ title, subtitle, actions }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h1 className="font-serif text-2xl font-semibold tracking-tight sm:text-3xl">{title}</h1>
        {subtitle && <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">{subtitle}</div>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-xl bg-white p-5 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800 ${className}`}>{children}</div>;
}

export function Badge({ children, tone = "stone" }: { children: ReactNode; tone?: "stone" | "brand" | "amber" | "red" }) {
  const t = {
    stone: "bg-stone-100 text-stone-700 dark:bg-stone-800 dark:text-stone-300",
    brand: "bg-brand-100 text-brand-800 dark:bg-brand-900/60 dark:text-brand-200",
    amber: "bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200",
    red: "bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-200",
  }[tone];
  return <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${t}`}>{children}</span>;
}

export function Tabs<T extends string>({ tabs, value, onChange }: { tabs: { id: T; label: string }[]; value: T; onChange: (t: T) => void }) {
  return (
    <div role="tablist" className="-mx-1 flex gap-1 overflow-x-auto border-b border-stone-200 px-1 dark:border-stone-800">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          aria-selected={value === t.id}
          onClick={() => onChange(t.id)}
          className={`whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium transition ${
            value === t.id ? "border-brand-600 text-brand-700 dark:text-brand-300" : "border-transparent text-stone-500 hover:text-stone-800 dark:hover:text-stone-200"
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
