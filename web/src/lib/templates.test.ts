import { describe, expect, it } from "vitest";
import { applyTemplate } from "./templates";

describe("applyTemplate", () => {
  const t = "## Places Lived\n- ";
  it("inserts directly into empty text", () => {
    expect(applyTemplate("", t, "append")).toBe(t);
    expect(applyTemplate("  \n", t, "append")).toBe(t);
  });
  it("appends after existing text without losing any of it", () => {
    expect(applyTemplate("Grew up in Leeds.\n\n\n", t, "append")).toBe("Grew up in Leeds.\n\n" + t);
    expect(applyTemplate("  indented start", t, "append")).toBe("  indented start\n\n" + t);
  });
  it("replaces only when asked", () => {
    expect(applyTemplate("Old story", t, "replace")).toBe(t);
  });
});
