import { describe, expect, it } from "vitest";
import { addDays, addMonths, checkVitals, compareISO, daysInMonth, firstWeekday, formatVitalDate, parseISODate, vitalRange } from "./dates";

describe("parseISODate", () => {
  it("accepts and normalizes ISO dates", () => {
    const ok = { "1890-03-12": "1890-03-12", "1890-3-5": "1890-03-05", "1890-03": "1890-03", "1890": "1890", "2000-02-29": "2000-02-29" };
    for (const [input, want] of Object.entries(ok)) {
      const p = parseISODate(input);
      expect(p.ok && p.date.value, input).toBe(want);
    }
  });
  it("rejects malformed and impossible dates with a message", () => {
    for (const bad of ["12 MAR 1850", "ABT 1850", "1900-02-29", "1890-13-01", "1890-04-31", "0000", "90-3-12", "1890/03/12"]) {
      const p = parseISODate(bad);
      expect(p.ok, bad).toBe(false);
      if (!p.ok) expect(p.error.length).toBeGreaterThan(5);
    }
  });
});

describe("checkVitals", () => {
  const today = "2026-11-27";
  it("normalizes valid values", () => {
    expect(checkVitals("1890-3-5", "1960", "", "", today)).toMatchObject({ birth: "1890-03-05", death: "1960", birthError: undefined, deathError: undefined });
  });
  it("rejects death before birth at the shared precision", () => {
    expect(checkVitals("1890-03-12", "1890-03-11", "", "", today).deathError).toMatch(/before the birth date/);
    expect(checkVitals("1890", "1889-12", "", "", today).deathError).toMatch(/before/);
    expect(checkVitals("1890-06", "1890", "", "", today).deathError).toBeUndefined();
  });
  it("rejects future dates", () => {
    expect(checkVitals("2026-11-28", "", "", "", today).birthError).toMatch(/future/);
    expect(checkVitals("2026", "", "", "", today).birthError).toBeUndefined();
  });
  it("keeps an unchanged legacy value but rejects new non-ISO text", () => {
    expect(checkVitals("ABT 1850", "", "ABT 1850", "", today)).toMatchObject({ birthError: undefined, birth: "ABT 1850" });
    expect(checkVitals("ABT 1851", "", "ABT 1850", "", today).birthError).toBeDefined();
  });
  it("warns gently about unusual values", () => {
    expect(checkVitals("1800", "1950", "", "", today).warnings[0]).toMatch(/120 years/);
    expect(checkVitals("1800", "", "", "", today).warnings[0]).toMatch(/deceased/);
    expect(checkVitals("1950", "2010", "", "", today).warnings).toEqual([]);
  });
});

describe("display", () => {
  it("shows ISO consistently and older values gracefully", () => {
    expect(formatVitalDate("1890-03-12")).toBe("1890-03-12");
    expect(formatVitalDate("12 MAR 1850")).toBe("1850-03-12");
    expect(formatVitalDate("MAR 1850")).toBe("1850-03");
    expect(formatVitalDate("ABT 1850")).toBe("c. 1850");
    expect(formatVitalDate("BEF 2 JAN 1900")).toBe("before 1900-01-02");
    expect(formatVitalDate("family bible")).toBe("family bible");
    expect(vitalRange({ birthDate: "1890-03-12", deathDate: "1960" })).toBe("1890-03-12 – 1960");
    expect(vitalRange({ birthDate: "1890-03-12", deathDate: "" })).toBe("b. 1890-03-12");
    expect(vitalRange({ birthDate: "", deathDate: "", deceased: true })).toBe("Deceased");
  });
});

describe("compareISO", () => {
  it("compares at the shared precision", () => {
    expect(compareISO("1850", "1850-06-01")).toBe(0);
    expect(compareISO("1849-12", "1850")).toBe(-1);
    expect(compareISO("1850-03-12", "1850-03-11")).toBe(1);
  });
});

describe("calendar math", () => {
  it("handles month lengths, weekdays and arithmetic (incl. early years)", () => {
    expect(daysInMonth(1900, 2)).toBe(28);
    expect(daysInMonth(2000, 2)).toBe(29);
    expect(firstWeekday(2026, 11)).toBe(0); // 1 Nov 2026 is a Sunday
    expect(addDays(1890, 12, 31, 1)).toEqual([1891, 1, 1]);
    expect(addMonths(1890, 1, 31, 1)).toEqual([1890, 2, 28]);
    expect(addMonths(1890, 1, 15, -1)).toEqual([1889, 12, 15]);
    expect(addDays(50, 3, 1, -1)).toEqual([50, 2, 28]); // no 1900s remapping for small years
  });
});
