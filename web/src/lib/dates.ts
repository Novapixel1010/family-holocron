// Birth and death dates use ISO calendar dates: YYYY-MM-DD, or YYYY-MM / YYYY when
// the day or month is unknown. Mirrors internal/gendate/iso.go on the server.

export type DatePrecision = "day" | "month" | "year";

export interface ISODate {
  value: string; // normalized, zero-padded
  precision: DatePrecision;
  y: number;
  m: number; // 1-12, 0 when unknown
  d: number; // 1-31, 0 when unknown
}

export const ISO_HINT = "YYYY-MM-DD — or YYYY-MM / YYYY if the day or month is unknown";
export const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

export function daysInMonth(y: number, m: number): number {
  const d = new Date(0);
  d.setUTCFullYear(y, m, 0); // day 0 of the next month = last day of m
  return d.getUTCDate();
}

/** Parses YYYY, YYYY-M(M), YYYY-M(M)-D(D). Returns an error message for anything else. */
export function parseISODate(raw: string): { ok: true; date: ISODate } | { ok: false; error: string } {
  const m = /^(\d{4})(?:-(\d{1,2})(?:-(\d{1,2}))?)?$/.exec(raw);
  if (!m) return { ok: false, error: `Use ${ISO_HINT}.` };
  const y = Number(m[1]);
  if (y < 1) return { ok: false, error: "Year must be 0001 or later." };
  const pad = (n: number, w = 2) => String(n).padStart(w, "0");
  if (m[2] === undefined) return { ok: true, date: { value: pad(y, 4), precision: "year", y, m: 0, d: 0 } };
  const mo = Number(m[2]);
  if (mo < 1 || mo > 12) return { ok: false, error: "Month must be between 01 and 12." };
  if (m[3] === undefined) return { ok: true, date: { value: `${pad(y, 4)}-${pad(mo)}`, precision: "month", y, m: mo, d: 0 } };
  const d = Number(m[3]);
  if (d < 1 || d > daysInMonth(y, mo)) return { ok: false, error: `${pad(y, 4)}-${pad(mo)} has no day ${d}.` };
  return { ok: true, date: { value: `${pad(y, 4)}-${pad(mo)}-${pad(d)}`, precision: "day", y, m: mo, d } };
}

export function isISODate(raw: string): boolean {
  const p = parseISODate(raw);
  return p.ok && p.date.value === raw;
}

/** Compares normalized ISO dates at their shared precision ("1850" equals "1850-06-01"). */
export function compareISO(a: string, b: string): number {
  const n = Math.min(a.length, b.length);
  return a.slice(0, n) < b.slice(0, n) ? -1 : a.slice(0, n) > b.slice(0, n) ? 1 : 0;
}

export function todayISO(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}

export interface VitalsCheck {
  birthError?: string;
  deathError?: string;
  warnings: string[];
  birth?: string; // normalized values to save
  death?: string;
}

/**
 * Validates birth/death input. `initial*` are the stored values: a pre-standard value
 * (e.g. "ABT 1850" from an import) is accepted only while it is left unchanged.
 */
export function checkVitals(birth: string, death: string, initialBirth = "", initialDeath = "", today = todayISO()): VitalsCheck {
  const out: VitalsCheck = { warnings: [] };
  const one = (raw: string, initial: string): { iso?: string; error?: string; value: string } => {
    const v = raw.trim();
    if (!v) return { value: "" };
    const p = parseISODate(v);
    if (!p.ok) return v === initial ? { value: v } : { error: p.error, value: v };
    if (compareISO(p.date.value, today) > 0) return { error: "Can't be in the future.", value: v };
    return { iso: p.date.value, value: p.date.value };
  };
  const b = one(birth, initialBirth);
  const d = one(death, initialDeath);
  out.birthError = b.error;
  out.deathError = d.error;
  out.birth = b.value;
  out.death = d.value;
  if (b.iso && d.iso) {
    if (compareISO(d.iso, b.iso) < 0) out.deathError = `Can't be before the birth date (${b.iso}).`;
    else if (Number(d.iso.slice(0, 4)) - Number(b.iso.slice(0, 4)) > 120) out.warnings.push("That’s a lifespan of more than 120 years — please double-check the dates.");
  }
  if (b.iso && !d.iso && !out.deathError && Number(today.slice(0, 4)) - Number(b.iso.slice(0, 4)) > 120 && !death.trim()) {
    out.warnings.push("Born more than 120 years ago with no death date — consider marking them deceased.");
  }
  return out;
}

const QUAL: Record<string, string> = { ABT: "c.", ABOUT: "c.", CIRCA: "c.", EST: "est.", CAL: "calc.", BEF: "before", AFT: "after" };
const MON: Record<string, number> = { JAN: 1, FEB: 2, MAR: 3, APR: 4, MAY: 5, JUN: 6, JUL: 7, AUG: 8, SEP: 9, OCT: 10, NOV: 11, DEC: 12 };

/**
 * Display form for birth/death dates: always YYYY-MM-DD (or YYYY-MM / YYYY).
 * Older stored values are shown gracefully: "12 MAR 1850" → "1850-03-12",
 * "ABT 1850" → "c. 1850"; anything unrecognized is shown as written.
 */
export function formatVitalDate(raw: string | null | undefined): string {
  const s = (raw ?? "").trim();
  if (!s) return "";
  const p = parseISODate(s);
  if (p.ok) return p.date.value;
  const parts = s.toUpperCase().split(/\s+/);
  const qual = QUAL[parts[0]];
  const rest = qual ? parts.slice(1) : parts;
  let iso: string | null = null;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (rest.length === 1 && /^\d{3,4}$/.test(rest[0])) iso = rest[0].padStart(4, "0");
  else if (rest.length === 2 && MON[rest[0]] && /^\d{3,4}$/.test(rest[1])) iso = `${rest[1].padStart(4, "0")}-${pad(MON[rest[0]])}`;
  else if (rest.length === 3 && /^\d{1,2}$/.test(rest[0]) && MON[rest[1]] && /^\d{3,4}$/.test(rest[2])) {
    const cand = `${rest[2].padStart(4, "0")}-${pad(MON[rest[1]])}-${pad(Number(rest[0]))}`;
    if (parseISODate(cand).ok) iso = cand;
  }
  if (!iso) return s;
  return qual ? `${qual} ${iso}` : iso;
}

/** "1890-03-12 – 1960-01-02", "b. 1890-03-12", "d. 1960", "Deceased" or "". */
export function vitalRange(p: { birthDate: string; deathDate: string; deceased?: boolean }): string {
  const b = formatVitalDate(p.birthDate);
  const d = formatVitalDate(p.deathDate);
  if (b && d) return `${b} – ${d}`;
  if (b) return p.deceased ? `${b} – ?` : `b. ${b}`;
  if (d) return `d. ${d}`;
  return p.deceased ? "Deceased" : "";
}

// ---- calendar helpers ------------------------------------------------------------

/** Weekday (0 = Sunday) of the first day of month m (1-12) of year y. */
export function firstWeekday(y: number, m: number): number {
  const d = new Date(0);
  d.setUTCFullYear(y, m - 1, 1);
  return d.getUTCDay();
}

/** Adds days to a (y, m, d) date, returning the new date. */
export function addDays(y: number, m: number, d: number, delta: number): [number, number, number] {
  const t = new Date(0);
  t.setUTCFullYear(y, m - 1, d + delta);
  return [t.getUTCFullYear(), t.getUTCMonth() + 1, t.getUTCDate()];
}

/** Moves by whole months, clamping the day ("Jan 31" + 1 month → "Feb 28/29"). */
export function addMonths(y: number, m: number, d: number, delta: number): [number, number, number] {
  const idx = y * 12 + (m - 1) + delta;
  const ny = Math.floor(idx / 12);
  const nm = (idx % 12) + 1;
  return [ny, nm, Math.min(d, daysInMonth(ny, nm))];
}

export function isoOf(y: number, m: number, d: number): string {
  return `${String(y).padStart(4, "0")}-${String(m).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
}
