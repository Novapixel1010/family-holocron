import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { addDays, addMonths, daysInMonth, firstWeekday, isoOf, MONTHS, parseISODate, todayISO } from "@/lib/dates";
import { inputClass } from "./Field";

const WEEKDAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
const POPOVER_W = 296;
const POPOVER_H = 352;
const GAP = 6;

/**
 * Date input standardized on YYYY-MM-DD. Users can type a date (YYYY-MM / YYYY are
 * accepted for partly known dates), clear it, or click the box to pick a day from a
 * calendar. The calendar is portaled above modals and flips/clamps to stay on screen.
 */
export function DateField({
  label, value, onChange, onBlur, error, hint, warning, max = todayISO(), id: idProp,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  /** Called when the user leaves the field or picks/clears a date (e.g. to start showing errors). */
  onBlur?: () => void;
  error?: string;
  hint?: ReactNode;
  warning?: string;
  max?: string; // latest selectable YYYY-MM-DD (default: today)
  id?: string;
}) {
  const gen = useId();
  const id = idProp ?? gen;
  const msgId = `${id}-msg`;
  const popId = `${id}-calendar`;
  const input = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [focusGrid, setFocusGrid] = useState(false);

  const close = useCallback((returnFocus: boolean) => {
    setOpen(false);
    setFocusGrid(false);
    if (returnFocus) input.current?.focus();
  }, []);

  const normalizeOnBlur = () => {
    const p = parseISODate(value.trim());
    if (p.ok && p.date.value !== value) onChange(p.date.value);
    else if (value !== value.trim()) onChange(value.trim());
    onBlur?.();
  };

  const message = error ?? warning;
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="block text-sm font-medium text-stone-700 dark:text-stone-300">{label}</label>
      <div className="relative">
        <input
          ref={input}
          id={id}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          spellCheck={false}
          maxLength={20}
          placeholder="YYYY-MM-DD"
          value={value}
          aria-invalid={!!error || undefined}
          aria-describedby={msgId}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-controls={open ? popId : undefined}
          onChange={(e) => onChange(e.target.value)}
          onClick={() => setOpen(true)}
          onBlur={normalizeOnBlur}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || (e.altKey && e.key === "ArrowDown")) {
              e.preventDefault();
              setOpen(true);
              setFocusGrid(true);
            } else if (e.key === "Escape" && open) {
              e.preventDefault();
              e.stopPropagation(); // don't also close an enclosing modal
              close(false);
            } else if (e.key === "Tab" || e.key === "Enter") {
              if (e.key === "Enter" && open) e.preventDefault();
              close(false);
            }
          }}
          className={`${inputClass} pr-16 font-mono tabular-nums ${error ? "ring-red-400" : warning ? "ring-amber-400" : ""}`}
        />
        <div className="absolute inset-y-0 right-1 flex items-center gap-0.5">
          {value && (
            <button type="button" aria-label={`Clear ${label.toLowerCase()}`} title="Clear"
              onClick={() => { onChange(""); onBlur?.(); close(true); }}
              className="rounded p-1 text-stone-400 hover:bg-stone-100 hover:text-stone-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:hover:bg-stone-700 dark:hover:text-stone-200">
              ✕
            </button>
          )}
          <button type="button" aria-label={`Choose ${label.toLowerCase()} from a calendar`} title="Open calendar" tabIndex={-1}
            onClick={() => { if (open) close(true); else { setOpen(true); setFocusGrid(true); } }}
            className="rounded p-1 text-stone-500 hover:bg-stone-100 hover:text-stone-800 dark:hover:bg-stone-700 dark:hover:text-stone-100">
            <svg viewBox="0 0 20 20" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
              <rect x="3" y="4.5" width="14" height="12" rx="2" /><path d="M3 8.5h14M7 2.5v4M13 2.5v4" />
            </svg>
          </button>
        </div>
      </div>
      <p id={msgId} className={`text-xs ${error ? "text-red-600 dark:text-red-400" : warning ? "text-amber-700 dark:text-amber-400" : "text-stone-500"}`}
        role={message ? "alert" : undefined}>
        {message ?? hint ?? "YYYY-MM-DD — click to pick a date, or type YYYY-MM / YYYY if unknown"}
      </p>
      {open && (
        <CalendarPopover id={popId} anchor={input} label={label} value={value} max={max} focusGrid={focusGrid}
          onPick={(iso) => { onChange(iso); onBlur?.(); close(true); }} onClose={close} />
      )}
    </div>
  );
}

function CalendarPopover({
  id, anchor, label, value, max, focusGrid, onPick, onClose,
}: {
  id: string; anchor: React.RefObject<HTMLInputElement>; label: string; value: string; max: string; focusGrid: boolean;
  onPick: (iso: string) => void; onClose: (returnFocus: boolean) => void;
}) {
  const parsed = parseISODate(value.trim());
  const selected = parsed.ok && parsed.date.precision === "day" ? parsed.date.value : "";
  const [maxY, maxM, maxD] = max.split("-").map(Number);
  // Initial view: the typed date (even partial), else today's month — clamped to max,
  // which may lie far in the future (e.g. 9999-12-31 for upcoming family milestones).
  const init = (): [number, number, number] => {
    if (parsed.ok) return [parsed.date.y, parsed.date.m || 1, parsed.date.d || 1];
    const t = todayISO();
    const start = t < max ? t : max;
    const [ty, tm, td] = start.split("-").map(Number);
    return [ty, tm, td];
  };
  const [cursor, setCursor] = useState<[number, number, number]>(init);
  const [y, m, d] = cursor;
  const [yearText, setYearText] = useState(String(y));
  const pop = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number; above: boolean } | null>(null);

  // Follow the typed value while the calendar is open.
  useEffect(() => {
    if (parsed.ok) setCursor([parsed.date.y, parsed.date.m || 1, parsed.date.d || 1]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);
  useEffect(() => setYearText(String(y)), [y]);

  // Position: below the field, or above when there's no room; clamped to the viewport.
  const place = useCallback(() => {
    const el = anchor.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    const above = r.bottom + GAP + POPOVER_H > vh && r.top - GAP - POPOVER_H >= 0;
    const top = above ? r.top - GAP - POPOVER_H : Math.min(r.bottom + GAP, Math.max(8, vh - POPOVER_H - 8));
    const left = Math.min(Math.max(8, r.left), Math.max(8, vw - POPOVER_W - 8));
    setPos({ top, left, above });
  }, [anchor]);
  useLayoutEffect(() => {
    place();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true); // capture: any scrolling ancestor (e.g. a modal body)
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [place]);

  // Close on outside pointer-down (capture phase, so nothing can swallow it).
  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (pop.current?.contains(t) || anchor.current?.parentElement?.contains(t)) return;
      onClose(false);
    };
    document.addEventListener("pointerdown", onDown, true);
    return () => document.removeEventListener("pointerdown", onDown, true);
  }, [anchor, onClose]);

  // Move keyboard focus to the active day when requested.
  useEffect(() => {
    if (!focusGrid || !pos) return;
    pop.current?.querySelector<HTMLButtonElement>(`[data-iso="${isoOf(y, m, d)}"]`)?.focus();
  }, [focusGrid, pos, y, m, d]);

  const afterMax = (iso: string) => iso > max;
  const canNext = isoOf(y, m, 1) < isoOf(maxY, maxM, 1);
  const move = (next: [number, number, number]) => {
    const iso = isoOf(...next);
    setCursor(afterMax(iso) ? [maxY, maxM, maxD] : next);
  };

  const onGridKey = (e: KeyboardEvent) => {
    const map: Record<string, () => [number, number, number]> = {
      ArrowLeft: () => addDays(y, m, d, -1), ArrowRight: () => addDays(y, m, d, 1),
      ArrowUp: () => addDays(y, m, d, -7), ArrowDown: () => addDays(y, m, d, 7),
      PageUp: () => addMonths(y, m, d, e.shiftKey ? -12 : -1), PageDown: () => addMonths(y, m, d, e.shiftKey ? 12 : 1),
      Home: () => addDays(y, m, d, -firstWeekdayOf(y, m, d)), End: () => addDays(y, m, d, 6 - firstWeekdayOf(y, m, d)),
    };
    if (map[e.key]) {
      e.preventDefault();
      move(map[e.key]());
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const iso = isoOf(y, m, d);
      if (!afterMax(iso)) onPick(iso);
    }
  };

  const commitYear = () => {
    const n = Number(yearText);
    if (Number.isInteger(n) && n >= 1 && n <= maxY) move([n, m, Math.min(d, daysInMonth(n, m))]);
    else setYearText(String(y));
  };

  const lead = firstWeekday(y, m);
  const total = daysInMonth(y, m);
  const cells: (number | null)[] = [...Array(lead).fill(null), ...Array.from({ length: total }, (_, i) => i + 1)];
  while (cells.length % 7) cells.push(null);
  const today = todayISO();
  const btn = "rounded-md px-2 py-1 text-sm hover:bg-stone-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:opacity-30 dark:hover:bg-stone-700";

  return createPortal(
    <div
      ref={pop}
      id={id}
      role="dialog"
      aria-label={`Choose ${label.toLowerCase()}`}
      // Above modals (z-50) and toasts (z-60).
      className="fixed z-[70] rounded-xl bg-white p-3 shadow-2xl ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700"
      style={{ width: POPOVER_W, top: pos?.top ?? -9999, left: pos?.left ?? -9999, visibility: pos ? "visible" : "hidden" }}
      data-placement={pos?.above ? "top" : "bottom"}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation(); // keep an enclosing modal open
          onClose(true);
        }
      }}
    >
      <div className="mb-2 flex items-center gap-1">
        <button type="button" className={btn} aria-label="Previous month" onClick={() => move(addMonths(y, m, d, -1))}>‹</button>
        <select aria-label="Month" value={m} onChange={(e) => move([y, Number(e.target.value), Math.min(d, daysInMonth(y, Number(e.target.value)))])}
          className="min-w-0 flex-1 rounded-md border-0 bg-stone-100 py-1 pl-2 pr-6 text-sm ring-0 focus:ring-2 focus:ring-brand-500 dark:bg-stone-800">
          {MONTHS.map((name, i) => (
            <option key={name} value={i + 1} disabled={y === maxY && i + 1 > maxM}>{name}</option>
          ))}
        </select>
        <input aria-label="Year" inputMode="numeric" value={yearText} maxLength={4}
          onChange={(e) => setYearText(e.target.value.replace(/\D/g, ""))}
          onBlur={commitYear}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commitYear(); } }}
          className="w-16 rounded-md border-0 bg-stone-100 px-2 py-1 text-center font-mono text-sm tabular-nums focus:ring-2 focus:ring-brand-500 dark:bg-stone-800" />
        <button type="button" className={btn} aria-label="Next month" disabled={!canNext} onClick={() => move(addMonths(y, m, d, 1))}>›</button>
      </div>
      <div role="grid" aria-label={`${MONTHS[m - 1]} ${y}`} onKeyDown={onGridKey}>
        <div role="row" className="grid grid-cols-7 text-center text-[11px] font-medium uppercase text-stone-400">
          {WEEKDAYS.map((w) => <div key={w} role="columnheader" className="py-1">{w}</div>)}
        </div>
        {Array.from({ length: cells.length / 7 }, (_, row) => (
          <div key={row} role="row" className="grid grid-cols-7">
            {cells.slice(row * 7, row * 7 + 7).map((day, i) => {
              if (!day) return <div key={i} role="gridcell" />;
              const iso = isoOf(y, m, day);
              const disabled = afterMax(iso);
              const isSel = iso === selected;
              const isActive = day === d;
              return (
                <div key={i} role="gridcell" aria-selected={isSel}>
                  <button
                    type="button"
                    data-iso={iso}
                    tabIndex={isActive ? 0 : -1}
                    disabled={disabled}
                    aria-label={`${day} ${MONTHS[m - 1]} ${y}${iso === today ? ", today" : ""}`}
                    aria-current={iso === today ? "date" : undefined}
                    onClick={() => onPick(iso)}
                    onFocus={() => !isActive && setCursor([y, m, day])}
                    className={`mx-auto flex h-9 w-9 items-center justify-center rounded-full text-sm tabular-nums focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-25 ${
                      isSel ? "bg-brand-600 font-semibold text-white" : iso === today ? "font-semibold text-brand-700 ring-1 ring-brand-400 dark:text-brand-300" : "hover:bg-stone-100 dark:hover:bg-stone-800"
                    }`}
                  >
                    {day}
                  </button>
                </div>
              );
            })}
          </div>
        ))}
      </div>
      <div className="mt-2 flex items-center justify-between border-t border-stone-200 pt-2 text-sm dark:border-stone-700">
        <button type="button" className={btn} onClick={() => onPick(String(y).padStart(4, "0"))} title="When only the year is known">
          Year only ({String(y).padStart(4, "0")})
        </button>
        <button type="button" className={btn} onClick={() => onClose(true)}>Close</button>
      </div>
    </div>,
    document.body,
  );
}

function firstWeekdayOf(y: number, m: number, d: number): number {
  const t = new Date(0);
  t.setUTCFullYear(y, m - 1, d);
  return t.getUTCDay();
}
