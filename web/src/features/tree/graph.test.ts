import { describe, expect, it } from "vitest";
import type { GraphLink, GraphPerson, GraphRole } from "@/api/types";
import { buildFlow, familyUnits, mergeGraph, overlaps, personKey, visibleGraph, PERSON_H, type TreeGraph } from "./graph";

const person = (id: number, generation: number, role: GraphRole = "ancestor", hasMore = false): GraphPerson => ({
  id, name: `P${id}`, givenName: `P${id}`, surname: "", sex: "U", birthDate: "", deathDate: "", birthYear: 0, deathYear: 0,
  living: true, portraitMediaId: null, hasAbout: false, generation, role, hasMore,
});
let relId = 100;
const parent = (p: number, c: number, subtype = "biological"): GraphLink => ({ id: relId++, type: "parent", source: p, target: c, subtype });
const spouse = (a: number, b: number, subtype = "married"): GraphLink => ({ id: relId++, type: "spouse", source: a, target: b, subtype });

// Kid(1) ← Dad(2) & Mom(3); Dad ← AuntA(4); Mom ← UncleB(5); AuntA & UncleB ← GP1(6) & GP2(7).
// Dad and Mom are cousins, so GP1/GP2 are reached twice (pedigree collapse).
const pedigree: TreeGraph = {
  people: [person(1, 0, "root"), person(2, 1), person(3, 1), person(4, 2), person(5, 2), person(6, 3), person(7, 3, "ancestor", true)],
  links: [parent(2, 1), parent(3, 1), parent(4, 2), parent(5, 3), parent(6, 4), parent(7, 4), parent(6, 5), parent(7, 5), spouse(2, 3), spouse(6, 7)],
};

const personIds = (nodes: { id: string; type?: string }[]) => nodes.filter((n) => n.type === "person").map((n) => n.id).sort();

describe("ancestor flow", () => {
  const vis = visibleGraph(pedigree, 1, "ancestors", new Set());
  const flow = buildFlow(vis, 1, "ancestors");

  it("renders every person exactly once with unique node and edge ids", () => {
    expect(personIds(flow.nodes)).toEqual([1, 2, 3, 4, 5, 6, 7].map(personKey).sort());
    expect(new Set(flow.nodes.map((n) => n.id)).size).toBe(flow.nodes.length);
    expect(new Set(flow.edges.map((e) => e.id)).size).toBe(flow.edges.length);
  });

  it("represents every relationship as an edge", () => {
    const covered = new Set(flow.edges.flatMap((e) => e.data!.relationshipIds));
    for (const l of pedigree.links) expect(covered.has(l.id), `relationship ${l.id}`).toBe(true);
    // Every edge connects existing nodes.
    const ids = new Set(flow.nodes.map((n) => n.id));
    for (const e of flow.edges) expect(ids.has(e.source) && ids.has(e.target)).toBe(true);
  });

  it("shares one couple junction for GP1+GP2 feeding both of their children", () => {
    const { units } = familyUnits(vis);
    const gp = units.find((u) => u.key === "u-6-7")!;
    expect(gp.spouse).toBeDefined();
    expect(gp.children.map((c) => c.id).sort()).toEqual([4, 5]);
  });

  it("lays out without overlapping cards, root at the bottom and ancestors above", () => {
    expect(overlaps(flow.nodes)).toEqual([]);
    const y = (id: number) => flow.nodes.find((n) => n.id === personKey(id))!.position.y;
    expect(y(2)).toBeLessThan(y(1) - PERSON_H);
    expect(y(4)).toBeLessThan(y(2) - PERSON_H);
    expect(y(6)).toBeLessThan(y(4) - PERSON_H);
    expect(y(2)).toBe(y(3)); // co-parents share a row
  });

  it("flags expandability from lineage links and server hasMore", () => {
    const data = (id: number) => flow.nodes.find((n) => n.id === personKey(id))!.data as { outward: number; person: GraphPerson };
    expect(data(1).outward).toBe(2);
    expect(data(6).outward).toBe(0);
    expect(data(7).person.hasMore).toBe(true);
  });
});

describe("collapse", () => {
  it("hides a subtree but keeps ancestors still reachable through another path", () => {
    const dadCollapsed = visibleGraph(pedigree, 1, "ancestors", new Set([2]));
    expect(dadCollapsed.people.map((p) => p.id).sort()).toEqual([1, 2, 3, 5, 6, 7]); // AuntA hidden, GP1/GP2 via Mom
    const both = visibleGraph(pedigree, 1, "ancestors", new Set([2, 3]));
    expect(both.people.map((p) => p.id).sort()).toEqual([1, 2, 3]);
    expect(both.links.every((l) => [1, 2, 3].includes(l.source) && [1, 2, 3].includes(l.target))).toBe(true);
  });
});

describe("descendant flow with partners", () => {
  // Root Dad(2) + Mom(3, partner) → Kid(1); Kid + Partner(8, no spouse record) → Grandkid(9); Kid & Spouse2(10) childless.
  const g: TreeGraph = {
    people: [person(2, 0, "root"), person(3, 0, "partner"), person(1, 1, "descendant"), person(8, 1, "partner"), person(10, 1, "partner"), person(9, 2, "descendant")],
    links: [parent(2, 1), parent(3, 1), parent(1, 9), parent(8, 9), spouse(2, 3), spouse(1, 10)],
  };
  it("includes every partner and places partners beside the person they are linked to", () => {
    const flow = buildFlow(visibleGraph(g, 2, "descendants", new Set()), 2, "descendants");
    expect(personIds(flow.nodes)).toEqual([1, 2, 3, 8, 9, 10].map(personKey).sort());
    expect(overlaps(flow.nodes)).toEqual([]);
    const y = (id: number) => flow.nodes.find((n) => n.id === personKey(id))!.position.y;
    expect(y(3)).toBe(y(2));
    expect(y(8)).toBe(y(1));
    expect(y(10)).toBe(y(1));
    expect(y(9)).toBeGreaterThan(y(1) + PERSON_H);
    const covered = new Set(flow.edges.flatMap((e) => e.data!.relationshipIds));
    for (const l of g.links) expect(covered.has(l.id)).toBe(true);
  });
  it("drops partners whose only link is hidden by a collapse", () => {
    const vis = visibleGraph(g, 2, "descendants", new Set([1]));
    expect(vis.people.map((p) => p.id).sort()).toEqual([1, 10, 2, 3].sort());
  });
});

describe("mergeGraph", () => {
  it("adds a fetched sub-graph with generation offsets and no duplicates", () => {
    const base: TreeGraph = { people: [person(1, 0, "root"), person(2, 1, "ancestor", true)], links: [pedigree.links[0]] };
    const extra: TreeGraph = {
      people: [person(2, 0, "root"), person(4, 1), person(6, 2, "ancestor", true)],
      links: [pedigree.links[2], pedigree.links[4]],
    };
    const merged = mergeGraph(base, extra, 2);
    const by = new Map(merged.people.map((p) => [p.id, p]));
    expect(merged.people).toHaveLength(4);
    expect(by.get(2)!.hasMore).toBe(false);
    expect(by.get(2)!.role).toBe("ancestor");
    expect(by.get(4)!.generation).toBe(2);
    expect(by.get(6)!.generation).toBe(3);
    expect(merged.links).toHaveLength(3);
    expect(mergeGraph(merged, extra, 2).links).toHaveLength(3);
  });
});

describe("parent kinds", () => {
  it("keeps separate, dashed edges when a child has a birth parent and an adoptive parent", () => {
    const g: TreeGraph = {
      people: [person(1, 0, "root"), person(2, 1), person(3, 1)],
      links: [parent(2, 1), parent(3, 1, "adopted"), spouse(2, 3, "divorced")],
    };
    const flow = buildFlow(visibleGraph(g, 1, "ancestors", new Set()), 1, "ancestors");
    const adopted = flow.edges.find((e) => e.source === personKey(3) && e.target === personKey(1))!;
    const birth = flow.edges.find((e) => e.source === personKey(2) && e.target === personKey(1))!;
    expect(adopted.className).toBe("holocron-edge-dashed");
    expect(birth.className).toBeUndefined();
    // The divorced couple still gets its own (ended) junction.
    expect(flow.edges.filter((e) => e.className === "holocron-edge-ended")).toHaveLength(2);
    expect(overlaps(flow.nodes)).toEqual([]);
  });
});
