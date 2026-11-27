import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useBioTemplates } from "@/api/hooks";
import type { BioTemplate } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Spinner } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { applyTemplate, type InsertMode } from "@/lib/templates";

/**
 * "Insert template" menu for the About editor. Empty text gets the template at once;
 * existing text asks whether to append or replace, so stories are never overwritten
 * by accident. Hidden when there are no templates.
 */
export function InsertTemplateButton({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const templates = useBioTemplates();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [pending, setPending] = useState<BioTemplate | null>(null);
  const box = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const items = useRef<(HTMLButtonElement | null)[]>([]);
  const toast = useToast();

  useEffect(() => {
    if (open) items.current[active]?.focus();
  }, [open, active]);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("pointerdown", onDown, true);
    return () => document.removeEventListener("pointerdown", onDown, true);
  }, [open]);

  if (templates.isPending) {
    return <span className="inline-flex items-center gap-1 text-stone-400" role="status"><Spinner className="h-3 w-3" /> Loading templates…</span>;
  }
  if (templates.isError) {
    return (
      <span className="text-stone-500" role="status">
        Templates unavailable · <button type="button" className="underline hover:text-stone-800 dark:hover:text-stone-200" onClick={() => templates.refetch()}>Retry</button>
      </span>
    );
  }
  const list = templates.data;
  if (list.length === 0) return null;

  const insert = (t: BioTemplate, mode: InsertMode) => {
    onChange(applyTemplate(value, t.contentMd, mode));
    setPending(null);
    toast(mode === "replace" ? `Replaced with “${t.name}”` : value.trim() ? `Added “${t.name}” at the end` : `Inserted “${t.name}”`);
  };
  const choose = (t: BioTemplate) => {
    setOpen(false);
    button.current?.focus();
    if (!value.trim()) insert(t, "append");
    else setPending(t);
  };
  const onMenuKey = (e: KeyboardEvent) => {
    const n = list.length;
    if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => (a + 1) % n); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => (a - 1 + n) % n); }
    else if (e.key === "Home") { e.preventDefault(); setActive(0); }
    else if (e.key === "End") { e.preventDefault(); setActive(n - 1); }
    else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); setOpen(false); button.current?.focus(); }
    else if (e.key === "Tab") setOpen(false);
  };

  return (
    <div ref={box} className="relative">
      <button ref={button} type="button" aria-haspopup="menu" aria-expanded={open}
        onClick={() => { setActive(0); setOpen(!open); }}
        onKeyDown={(e) => { if (e.key === "ArrowDown") { e.preventDefault(); setActive(0); setOpen(true); } }}
        className="inline-flex items-center gap-1 rounded px-2 py-0.5 font-medium text-brand-700 ring-1 ring-brand-200 hover:bg-brand-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:text-brand-300 dark:ring-brand-800 dark:hover:bg-brand-900/40">
        Insert template <span aria-hidden="true">▾</span>
      </button>
      {open && (
        <div role="menu" aria-label="Biography templates" onKeyDown={onMenuKey}
          className="absolute left-0 top-full z-40 mt-1 max-h-72 w-72 overflow-auto rounded-lg bg-white py-1 text-sm shadow-lg ring-1 ring-stone-200 dark:bg-stone-800 dark:ring-stone-700">
          {list.map((t, i) => (
            <button key={t.id} ref={(el) => { items.current[i] = el; }} type="button" role="menuitem" tabIndex={i === active ? 0 : -1}
              onClick={() => choose(t)} onMouseEnter={() => setActive(i)}
              className={`block w-full px-3 py-2 text-left focus:outline-none ${i === active ? "bg-stone-100 dark:bg-stone-700" : ""}`}>
              <span className="block font-medium text-stone-800 dark:text-stone-100">{t.name}</span>
              {t.description && <span className="block text-xs text-stone-500 dark:text-stone-400">{t.description}</span>}
            </button>
          ))}
        </div>
      )}
      <Modal open={!!pending} onClose={() => setPending(null)} title={`Insert “${pending?.name ?? ""}”?`} size="sm"
        footer={pending && (
          <>
            <Button onClick={() => setPending(null)}>Cancel</Button>
            <Button variant="danger" onClick={() => insert(pending, "replace")}>Replace existing text</Button>
            <Button variant="primary" onClick={() => insert(pending, "append")}>Append to end</Button>
          </>
        )}>
        <div className="space-y-2 text-sm text-stone-700 dark:text-stone-300">
          <p>This About section already has text. Add the template after it, or replace what’s there?</p>
          <p className="text-stone-500">Nothing is saved until you press Save — you can still Cancel the edit afterwards.</p>
        </div>
      </Modal>
    </div>
  );
}
