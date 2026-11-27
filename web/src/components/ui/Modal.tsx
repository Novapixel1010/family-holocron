import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Button } from "./Button";

export function Modal({
  open, onClose, title, children, footer, size = "md",
}: { open: boolean; onClose: () => void; title: string; children: ReactNode; footer?: ReactNode; size?: "sm" | "md" | "lg" }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const prev = document.activeElement as HTMLElement | null;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    const first = ref.current?.querySelector<HTMLElement>("input, textarea, select, button:not([data-close])");
    first?.focus();
    return () => {
      document.removeEventListener("keydown", onKey);
      prev?.focus?.();
    };
  }, [open, onClose]);
  if (!open) return null;
  const width = size === "sm" ? "max-w-md" : size === "lg" ? "max-w-3xl" : "max-w-xl";
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-stone-900/50 p-0 sm:items-center sm:p-4" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} role="dialog" aria-modal="true" aria-label={title} className={`flex max-h-[92vh] w-full ${width} flex-col rounded-t-2xl bg-white shadow-xl dark:bg-stone-900 sm:rounded-2xl`}>
        <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3 dark:border-stone-800">
          <h2 className="text-lg font-semibold">{title}</h2>
          <button data-close onClick={onClose} className="rounded-md p-1 text-stone-500 hover:bg-stone-100 dark:hover:bg-stone-800" aria-label="Close">
            ✕
          </button>
        </div>
        <div className="overflow-y-auto px-5 py-4">{children}</div>
        {footer && <div className="flex flex-wrap justify-end gap-2 border-t border-stone-200 px-5 py-3 dark:border-stone-800">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

export function ConfirmDialog({
  open, title, children, confirmLabel = "Delete", onConfirm, onClose, loading, danger = true, error,
}: {
  open: boolean; title: string; children: ReactNode; confirmLabel?: string; onConfirm: () => void; onClose: () => void;
  loading?: boolean; danger?: boolean; error?: ReactNode;
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="sm"
      footer={
        <>
          <Button onClick={onClose} disabled={loading}>Cancel</Button>
          <Button variant={danger ? "danger" : "primary"} onClick={onConfirm} loading={loading}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="space-y-3 text-sm text-stone-700 dark:text-stone-300">
        {children}
        {error}
      </div>
    </Modal>
  );
}
