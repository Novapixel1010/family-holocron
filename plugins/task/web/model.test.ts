import { describe, expect, it } from "vitest";
import { addTags, claimActions } from "./model";

const ann = { id: 1, role: "editor" };
const ref = (id: number) => ({ id, username: `u${id}`, displayName: "" });

describe("claimActions", () => {
  it("follows the server's claim rules", () => {
    expect(claimActions({ status: "open", claimedBy: null }, ann)).toEqual(["claim"]);
    expect(claimActions({ status: "in_progress", claimedBy: ref(1) }, ann)).toEqual(["release"]);
    expect(claimActions({ status: "in_progress", claimedBy: ref(2) }, ann)).toEqual(["takeOver"]);
    expect(claimActions({ status: "in_progress", claimedBy: ref(2) }, { id: 9, role: "admin" })).toEqual(["takeOver", "release"]);
    expect(claimActions({ status: "done", claimedBy: null }, ann)).toEqual([]);
    expect(claimActions({ status: "open", claimedBy: null }, { id: 3, role: "viewer" })).toEqual([]);
  });
});

describe("addTags", () => {
  it("splits, trims and de-duplicates case-insensitively", () => {
    expect(addTags(["census"], " Census, uk ,  parish  records,")).toEqual(["census", "uk", "parish records"]);
  });
});
