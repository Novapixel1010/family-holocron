import { describe, expect, it } from "vitest";
import type { Task } from "./api";
import { addTags, claimActions, withStatus } from "./model";

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

describe("withStatus", () => {
  const task = (id: number): Task => ({
    id, title: `t${id}`, descriptionMd: "", status: "in_progress", priority: "medium", tags: [], claimedBy: ref(1), claimedAt: null,
    people: [], media: [], commentCount: 2, createdAt: "c", updatedAt: "old", createdBy: ref(2), updatedBy: ref(2),
  });
  it("updates status and attribution of the matching task only", () => {
    const me = ref(7);
    const [a, b] = withStatus([task(1), task(2)], 2, "blocked", me, "now");
    expect(a).toEqual(task(1));
    expect(b).toMatchObject({ status: "blocked", updatedAt: "now", updatedBy: me, createdBy: ref(2), commentCount: 2, claimedBy: ref(1) });
    expect(withStatus(task(3), 3, "done", me, "now")).toMatchObject({ status: "done", updatedBy: me });
    expect(withStatus(task(3), 4, "done", me, "now")).toEqual(task(3));
  });
});
