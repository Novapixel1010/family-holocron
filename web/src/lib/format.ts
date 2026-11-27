import type { Person, UserRef } from "@/api/types";

export function personName(p: Pick<Person, "givenName" | "surname"> | null | undefined): string {
  if (!p) return "Unknown person";
  const n = [p.givenName, p.surname].filter(Boolean).join(" ").trim();
  return n || "Unnamed person";
}

const MONTHS = ["", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const QUAL: Record<string, string> = { ABT: "c.", ABOUT: "c.", CIRCA: "c.", EST: "est.", CAL: "calc.", BEF: "before", AFT: "after" };

/** Renders stored genealogical dates readably: "1850-03-12" → "12 Mar 1850", "ABT 1850" → "c. 1850". */
export function formatGenDate(raw: string | null | undefined): string {
  const s = (raw ?? "").trim();
  if (!s) return "";
  const iso = /^(\d{3,4})(?:-(\d{1,2}))?(?:-(\d{1,2}))?$/.exec(s);
  if (iso) {
    const [, y, m, d] = iso;
    const mm = m ? Number(m) : 0;
    const dd = d ? Number(d) : 0;
    if (mm > 12) return s;
    return [dd ? String(dd) : "", mm ? MONTHS[mm] : "", String(Number(y))].filter(Boolean).join(" ");
  }
  const parts = s.split(/\s+/);
  const q = QUAL[parts[0].toUpperCase()];
  if (q) return `${q} ${formatGenDate(parts.slice(1).join(" ")) || parts.slice(1).join(" ")}`;
  return s.replace(/\b([A-Z]{3})\b/g, (m) => (MONTHS.includes(m[0] + m.slice(1).toLowerCase()) ? m[0] + m.slice(1).toLowerCase() : m));
}

/** Extracts a year from a stored date, or null. */
export function yearOf(raw: string | null | undefined): number | null {
  const m = /(\d{3,4})/.exec(raw ?? "");
  return m ? Number(m[1]) : null;
}

/** Recognises dates the server can sort. Mirrors internal/gendate. */
export function isSortableDate(raw: string): boolean {
  const s = raw.trim().toUpperCase().replace(/,/g, " ");
  if (!s) return true;
  const months = "JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|SEPT|OCT|NOV|DEC|JANUARY|FEBRUARY|MARCH|APRIL|JUNE|JULY|AUGUST|SEPTEMBER|OCTOBER|NOVEMBER|DECEMBER";
  const core = `(\\d{3,4}(-(0?[1-9]|1[0-2])(-(0?[1-9]|[12]\\d|3[01]))?)?|((\\d{1,2}\\s+)?(${months})\\s+\\d{3,4})|((${months})\\s+\\d{1,2}\\s+\\d{3,4}))`;
  const re = new RegExp(`^((ABT|ABOUT|CIRCA|C\\.|CA\\.?|EST|CAL|BEF|BEFORE|AFT|AFTER|INT|~)\\s+)?${core}$`);
  const range = new RegExp(`^(BET|BETWEEN|FROM)\\s+${core}(\\s+(AND|TO|-)\\s+.*)?$`);
  const norm = s.replace(/\s+/g, " ");
  return re.test(norm) || range.test(norm);
}

export function lifespan(p: { birthDate: string; deathDate: string; deceased?: boolean }): string {
  const b = yearOf(p.birthDate);
  const d = yearOf(p.deathDate);
  if (!b && !d) return p.deceased ? "Deceased" : "";
  if (b && d) return `${b}–${d}`;
  if (b) return p.deceased ? `${b}–?` : `b. ${b}`;
  return `d. ${d}`;
}

export function userLabel(u: UserRef | null | undefined, fallback = "a removed user"): string {
  if (!u) return fallback;
  return u.displayName || u.username;
}

const rtf = typeof Intl !== "undefined" && "RelativeTimeFormat" in Intl ? new Intl.RelativeTimeFormat(undefined, { numeric: "auto" }) : null;

export function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = (t - Date.now()) / 1000;
  const abs = Math.abs(diff);
  if (!rtf) return new Date(iso).toLocaleString();
  if (abs < 45) return "just now";
  if (abs < 3600) return rtf.format(Math.round(diff / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(diff / 3600), "hour");
  if (abs < 86400 * 30) return rtf.format(Math.round(diff / 86400), "day");
  return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function dateTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

export function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = bytes / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}

export function titleCase(s: string): string {
  return s.replace(/(^|[\s_-])(\w)/g, (_, sep, c) => (sep === "_" || sep === "-" ? " " : sep) + c.toUpperCase());
}

export const SEX_LABEL: Record<string, string> = { M: "Male", F: "Female", X: "Other", U: "Unknown" };

export const EVENT_ICON: Record<string, string> = {
  birth: "✦", death: "✝", marriage: "⚭", divorce: "⚮", engagement: "◇", residence: "⌂", baptism: "≈", christening: "≈",
  burial: "▭", cremation: "▭", education: "✎", graduation: "✎", occupation: "⚒", military: "⚔", immigration: "⛵",
  emigration: "⛵", naturalization: "⚑", religion: "✧", medical: "✚", retirement: "☼", census: "☷", custom: "•",
  family: "♥", historical: "⌛",
};
