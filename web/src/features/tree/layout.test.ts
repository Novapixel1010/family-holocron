import { describe, expect, it } from "vitest";
import type { TreeNode } from "@/api/types";
import { fitView, layoutTree, NODE_H, NODE_W, GAP_X, GAP_Y, zoomAt } from "./layout";

const p = (id: number) => ({ id, name: `P${id}`, givenName: `P${id}`, surname: "", sex: "U" as const, birthDate: "", deathDate: "", birthYear: 0, deathYear: 0, living: true, portraitMediaId: null });
const node = (id: number, children: TreeNode[] = [], hasMore = false): TreeNode => ({ person: p(id), children, hasMore });

describe("layoutTree", () => {
  const tree = node(1, [node(2, [node(4), node(5)]), node(3, [], true)]);

  it("places generations in columns and centres parents on children", () => {
    const l = layoutTree(tree);
    const by = Object.fromEntries(l.nodes.map((n) => [n.person.id, n]));
    expect(by[1].x).toBe(0);
    expect(by[2].x).toBe(NODE_W + GAP_X);
    expect(by[4].x).toBe(2 * (NODE_W + GAP_X));
    expect(by[4].y).toBe(0);
    expect(by[5].y).toBe(NODE_H + GAP_Y);
    expect(by[2].y).toBe((by[4].y + by[5].y) / 2);
    expect(by[1].y).toBe((by[2].y + by[3].y) / 2);
    expect(l.edges).toHaveLength(4);
    expect(by[3].hasMore).toBe(true);
  });

  it("hides collapsed subtrees and applies grafts", () => {
    const key2 = "0:1/0:2";
    const collapsed = layoutTree(tree, { collapsed: new Set([key2]) });
    expect(collapsed.nodes.map((n) => n.person.id).sort()).toEqual([1, 2, 3]);
    expect(collapsed.nodes.find((n) => n.person.id === 2)!.collapsed).toBe(true);

    const grafted = layoutTree(tree, { grafts: new Map([["0:1/1:3", [node(6), node(7)]]]) });
    expect(grafted.nodes).toHaveLength(7);
    expect(grafted.nodes.find((n) => n.person.id === 3)!.hasMore).toBe(false);
  });

  it("gives duplicate ancestors distinct keys", () => {
    const l = layoutTree(node(1, [node(2, [node(9)]), node(3, [node(9)])]));
    const keys = l.nodes.filter((n) => n.person.id === 9).map((n) => n.key);
    expect(new Set(keys).size).toBe(2);
  });
});

describe("view math", () => {
  it("zooms around a fixed point", () => {
    const v = zoomAt({ x: 0, y: 0, k: 1 }, 2, 100, 100);
    expect(v).toEqual({ k: 2, x: -100, y: -100 });
  });
  it("fits content", () => {
    const v = fitView({ nodes: [], edges: [], width: 1000, height: 500 }, 600, 400, 0);
    expect(v.k).toBeCloseTo(0.6);
  });
});
