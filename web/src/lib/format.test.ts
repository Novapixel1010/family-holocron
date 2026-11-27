import { describe, expect, it } from "vitest";
import { formatGenDate, isSortableDate, lifespan, titleCase, yearOf } from "./format";

describe("formatGenDate", () => {
  it("formats ISO and GEDCOM dates", () => {
    expect(formatGenDate("1850-03-12")).toBe("12 Mar 1850");
    expect(formatGenDate("1850-03")).toBe("Mar 1850");
    expect(formatGenDate("1850")).toBe("1850");
    expect(formatGenDate("ABT 1850")).toBe("c. 1850");
    expect(formatGenDate("BEF 12 MAR 1850")).toBe("before 12 Mar 1850");
    expect(formatGenDate("")).toBe("");
  });
});

describe("isSortableDate", () => {
  it("accepts the same formats as the server", () => {
    for (const ok of ["1850", "1850-03-12", "12 MAR 1850", "abt 1850", "BET 1850 AND 1860", "March 3, 1901", ""]) {
      expect(isSortableDate(ok), ok).toBe(true);
    }
    for (const bad of ["spring", "1850-13-01", "12/03/1850"]) expect(isSortableDate(bad), bad).toBe(false);
  });
});

describe("helpers", () => {
  it("computes lifespans and years", () => {
    expect(lifespan({ birthDate: "1900", deathDate: "1980" })).toBe("1900–1980");
    expect(lifespan({ birthDate: "1900", deathDate: "" })).toBe("b. 1900");
    expect(lifespan({ birthDate: "", deathDate: "", deceased: true })).toBe("Deceased");
    expect(yearOf("ABT 1850")).toBe(1850);
    expect(titleCase("family_milestone")).toBe("Family Milestone");
  });
});
