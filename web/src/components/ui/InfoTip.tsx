import { useEffect, useId, useState } from "react";

/**
 * Small "i" button with a tooltip shown on hover and keyboard focus.
 * Escape dismisses it until the pointer leaves or focus moves away.
 */
export function InfoTip({ label, children, align = "center" }: { label: string; children: string; align?: "left" | "center" | "right" }) {
  const id = useId();
  const [hover, setHover] = useState(false);
  const [focus, setFocus] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const open = (hover || focus) && !dismissed;

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setDismissed(true);
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  const pos = align === "left" ? "left-0" : align === "right" ? "right-0" : "left-1/2 -translate-x-1/2";
  return (
    <span className="relative inline-flex" onMouseEnter={() => setHover(true)} onMouseLeave={() => { setHover(false); setDismissed(false); }}>
      <button
        type="button"
        aria-label={label}
        aria-describedby={open ? id : undefined}
        onFocus={() => setFocus(true)}
        onBlur={() => { setFocus(false); setDismissed(false); }}
        onClick={() => setDismissed(false)}
        className="flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold italic text-stone-500 ring-1 ring-stone-300 hover:bg-stone-200 hover:text-stone-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:text-stone-400 dark:ring-stone-600 dark:hover:bg-stone-700 dark:hover:text-stone-100"
      >
        i
      </button>
      {open && (
        <span id={id} role="tooltip"
          className={`absolute top-full z-50 mt-2 w-64 rounded-lg bg-stone-900 px-3 py-2 text-xs font-normal not-italic leading-relaxed text-white shadow-lg dark:bg-stone-100 dark:text-stone-900 ${pos}`}>
          {children}
        </span>
      )}
    </span>
  );
}
