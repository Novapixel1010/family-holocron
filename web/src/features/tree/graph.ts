// Pure data pipeline for the tree view:
//
//   API graph (people + links) ──merge──▶ full graph ──visibleGraph──▶ shown subset
//     ──buildFlow──▶ family units + Dagre layout ──▶ React Flow nodes/edges
//
// Every person is keyed by `p-<personId>` and every relationship by its unique id,
// so nothing can be overwritten by a duplicate key. Couples and parent sets become
// small "union" junction nodes (parents → union → children); this keeps partners
// side by side in Dagre (which cannot pin two nodes to the same rank directly).
import dagre from "@dagrejs/dagre";
import type { Edge, Node } from "@xyflow/react";
import type { GraphLink, GraphPerson, TreeDirection } from "@/api/types";

export const PERSON_W = 232;
export const PERSON_H = 92;
export const UNION_SIZE = 14;

export interface TreeGraph {
  people: GraphPerson[];
  links: GraphLink[];
}

export const personKey = (id: number) => `p-${id}`;

// ---- merge -------------------------------------------------------------------

/**
 * Merges a sub-graph fetched for `anchorId` (e.g. "load more generations") into
 * the current graph. Generations in `extra` are relative to the anchor, so they
 * are offset by the anchor's generation. People and links are de-duplicated by id.
 */
export function mergeGraph(base: TreeGraph, extra: TreeGraph, anchorId: number): TreeGraph {
  const offset = base.people.find((p) => p.id === anchorId)?.generation ?? 0;
  const byId = new Map(base.people.map((p) => [p.id, { ...p }]));
  for (const p of extra.people) {
    const existing = byId.get(p.id);
    if (existing) existing.hasMore = p.hasMore; // the newer fetch knows more about this person
    else byId.set(p.id, { ...p, generation: p.generation + offset });
  }
  const a = byId.get(anchorId);
  if (a) a.hasMore = false;
  const links = new Map(base.links.map((l) => [l.id, l]));
  for (const l of extra.links) links.set(l.id, l);
  return { people: [...byId.values()], links: [...links.values()] };
}

// ---- visibility ----------------------------------------------------------------

export interface VisibleGraph extends TreeGraph {
  /** Number of next-generation lineage links per person (for the expand/collapse toggle). */
  outward: Map<number, number>;
}

/**
 * Walks from the root along the tree direction, not descending into collapsed
 * people. A shared ancestor stays visible while any non-collapsed path reaches it.
 * Partners are shown while someone they are linked to is shown.
 */
export function visibleGraph(g: TreeGraph, rootId: number, direction: TreeDirection, collapsed: Set<number>): VisibleGraph {
  const people = new Map(g.people.map((p) => [p.id, p]));
  const outward = new Map<number, number[]>();
  const isPartner = (id: number) => people.get(id)?.role === "partner";
  for (const l of g.links) {
    // Lineage links only: partners hang off the lineage, they don't extend it.
    if (l.type !== "parent" || isPartner(l.source) || isPartner(l.target)) continue;
    const [from, to] = direction === "ancestors" ? [l.target, l.source] : [l.source, l.target];
    if (!outward.has(from)) outward.set(from, []);
    outward.get(from)!.push(to);
  }
  const shown = new Set<number>();
  if (people.has(rootId)) {
    const queue = [rootId];
    shown.add(rootId);
    while (queue.length) {
      const id = queue.shift()!;
      if (collapsed.has(id)) continue;
      for (const next of outward.get(id) ?? []) {
        if (!shown.has(next) && people.has(next)) {
          shown.add(next);
          queue.push(next);
        }
      }
    }
  }
  // Partners: visible when linked (as spouse, or as co-parent of a shown child) to a shown person.
  for (const l of g.links) {
    for (const [a, b] of [[l.source, l.target], [l.target, l.source]]) {
      if (people.get(a)?.role !== "partner" || shown.has(a) || !shown.has(b)) continue;
      if (l.type === "spouse" || (l.type === "parent" && a === l.source)) shown.add(a);
    }
  }
  return {
    people: g.people.filter((p) => shown.has(p.id)),
    links: g.links.filter((l) => shown.has(l.source) && shown.has(l.target)),
    outward: new Map([...outward].map(([k, v]) => [k, v.length])),
  };
}

// ---- family units + layout ---------------------------------------------------------

export interface PersonNodeData extends Record<string, unknown> {
  person: GraphPerson;
  outward: number;
  isRoot: boolean;
}

export interface UnionNodeData extends Record<string, unknown> {
  couple: boolean; // backed by a spouse/partner relationship
  subtype: string; // spouse subtype when couple
  parents: number[];
}

export type PersonFlowNode = Node<PersonNodeData, "person">;
export type UnionFlowNode = Node<UnionNodeData, "union">;
export type FlowNode = PersonFlowNode | UnionFlowNode;

export interface FlowEdgeData extends Record<string, unknown> {
  relationshipIds: number[];
  kind: "parent" | "child" | "partner";
}

export interface FlowResult {
  nodes: FlowNode[];
  edges: Edge<FlowEdgeData>[];
  width: number;
  height: number;
}

interface Unit {
  key: string;
  parents: number[];
  children: { id: number; relIds: number[]; subtypes: string[] }[];
  spouse?: GraphLink;
}

const NON_BIO = (s: string) => s !== "biological" && s !== "unknown" && s !== "";
const ENDED = ["divorced", "separated"];

/** Groups parent links into family units keyed by the child's full parent set, and adds couples. */
export function familyUnits(g: TreeGraph): { units: Unit[]; direct: { parent: number; child: number; rel: GraphLink }[] } {
  const parentsOf = new Map<number, GraphLink[]>();
  for (const l of g.links) {
    if (l.type !== "parent") continue;
    if (!parentsOf.has(l.target)) parentsOf.set(l.target, []);
    parentsOf.get(l.target)!.push(l);
  }
  const units = new Map<string, Unit>();
  const direct: { parent: number; child: number; rel: GraphLink }[] = [];
  for (const [child, rels] of parentsOf) {
    // A single parent, or parents of different kinds (e.g. birth + adoptive), get their own
    // edges so each link keeps its style; otherwise the parents share one junction.
    const mixed = new Set(rels.map((r) => NON_BIO(r.subtype))).size > 1;
    if (rels.length === 1 || mixed) {
      for (const r of rels) direct.push({ parent: r.source, child, rel: r });
      continue;
    }
    const parents = [...new Set(rels.map((r) => r.source))].sort((a, b) => a - b);
    const key = `u-${parents.join("-")}`;
    if (!units.has(key)) units.set(key, { key, parents, children: [] });
    units.get(key)!.children.push({ id: child, relIds: rels.map((r) => r.id), subtypes: rels.map((r) => r.subtype) });
  }
  for (const l of g.links) {
    if (l.type !== "spouse") continue;
    const parents = [l.source, l.target].sort((a, b) => a - b);
    const key = `u-${parents.join("-")}`;
    if (!units.has(key)) units.set(key, { key, parents, children: [] });
    units.get(key)!.spouse = l;
  }
  return { units: [...units.values()], direct };
}

/** Builds React Flow nodes/edges positioned by Dagre (top-left anchored). */
export function buildFlow(g: VisibleGraph, rootId: number, direction: TreeDirection): FlowResult {
  const { units, direct } = familyUnits(g);
  const dg = new dagre.graphlib.Graph();
  // rank 0 is the root: ancestors grow upward (BT), descendants downward (TB).
  dg.setGraph({ rankdir: direction === "ancestors" ? "BT" : "TB", nodesep: 48, ranksep: 36, edgesep: 16, marginx: 24, marginy: 24 });
  dg.setDefaultEdgeLabel(() => ({}));
  for (const p of g.people) dg.setNode(personKey(p.id), { width: PERSON_W, height: PERSON_H });
  for (const u of units) dg.setNode(u.key, { width: UNION_SIZE, height: UNION_SIZE });

  // Dagre edges point away from the root so ranks grow with generation.
  const outward = (parent: string, child: string) => (direction === "ancestors" ? [child, parent] : [parent, child]);
  for (const d of direct) {
    const [a, b] = outward(personKey(d.parent), personKey(d.child));
    dg.setEdge(a, b, { weight: 2 });
  }
  for (const u of units) {
    for (const p of u.parents) {
      if (u.children.length === 0) {
        // Childless couple: both partners feed the junction, which sits on the side away from the root.
        const [a, b] = direction === "ancestors" ? [u.key, personKey(p)] : [personKey(p), u.key];
        dg.setEdge(a, b, { weight: 4 });
      } else {
        const [a, b] = outward(personKey(p), u.key);
        dg.setEdge(a, b, { weight: 4 });
      }
    }
    for (const c of u.children) {
      const [a, b] = outward(u.key, personKey(c.id));
      dg.setEdge(a, b, { weight: 2 });
    }
  }
  dagre.layout(dg);

  const nodes: FlowNode[] = [];
  for (const p of g.people) {
    const pos = dg.node(personKey(p.id));
    nodes.push({
      id: personKey(p.id), type: "person", width: PERSON_W, height: PERSON_H,
      // Dagre positions are centre-anchored; React Flow's are top-left.
      position: { x: pos.x - PERSON_W / 2, y: pos.y - PERSON_H / 2 },
      data: { person: p, outward: g.outward.get(p.id) ?? 0, isRoot: p.id === rootId },
      draggable: false, selectable: false, connectable: false,
    });
  }
  for (const u of units) {
    const pos = dg.node(u.key);
    nodes.push({
      id: u.key, type: "union", width: UNION_SIZE, height: UNION_SIZE,
      position: { x: pos.x - UNION_SIZE / 2, y: pos.y - UNION_SIZE / 2 },
      data: { couple: !!u.spouse, subtype: u.spouse?.subtype ?? "", parents: u.parents },
      draggable: false, selectable: false, connectable: false, focusable: false,
    });
  }

  // In both directions parents end up above their junction and children below it
  // (a childless couple's junction also sits below the partners), so every edge
  // runs from a bottom handle into a top handle.
  const edges: Edge<FlowEdgeData>[] = [];
  const base = { type: "smoothstep", focusable: false, selectable: false } as const;
  for (const d of direct) {
    const dashed = NON_BIO(d.rel.subtype);
    edges.push({
      ...base, id: `rel-${d.rel.id}`, source: personKey(d.parent), target: personKey(d.child), sourceHandle: "bottom", targetHandle: "top",
      data: { relationshipIds: [d.rel.id], kind: "child" }, className: dashed ? "holocron-edge-dashed" : undefined,
    });
  }
  for (const u of units) {
    const ended = !!u.spouse && ENDED.includes(u.spouse.subtype);
    for (const p of u.parents) {
      edges.push({
        ...base, id: `${u.key}-from-${p}`, source: personKey(p), target: u.key, sourceHandle: "bottom", targetHandle: "top",
        data: { relationshipIds: u.spouse ? [u.spouse.id] : [], kind: u.children.length ? "parent" : "partner" },
        className: ended ? "holocron-edge-ended" : undefined,
      });
    }
    for (const c of u.children) {
      const dashed = c.subtypes.every(NON_BIO);
      edges.push({
        ...base, id: `${u.key}-to-${c.id}`, source: u.key, target: personKey(c.id), sourceHandle: "bottom", targetHandle: "top",
        data: { relationshipIds: c.relIds, kind: "child" }, className: dashed ? "holocron-edge-dashed" : undefined,
      });
    }
  }
  const gl = dg.graph();
  return { nodes, edges, width: gl.width ?? 0, height: gl.height ?? 0 };
}

/** Axis-aligned overlap check between person cards (used by tests and as a dev guard). */
export function overlaps(nodes: FlowNode[]): [string, string][] {
  const cards = nodes.filter((n) => n.type === "person");
  const out: [string, string][] = [];
  for (let i = 0; i < cards.length; i++) {
    for (let j = i + 1; j < cards.length; j++) {
      const a = cards[i], b = cards[j];
      if (a.position.x < b.position.x + PERSON_W && b.position.x < a.position.x + PERSON_W &&
          a.position.y < b.position.y + PERSON_H && b.position.y < a.position.y + PERSON_H) out.push([a.id, b.id]);
    }
  }
  return out;
}
