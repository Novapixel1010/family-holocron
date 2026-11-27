import type { TreeNode, TreeSummary } from "@/api/types";

export const NODE_W = 210;
export const NODE_H = 68;
export const GAP_X = 56;
export const GAP_Y = 14;

export interface LaidOutNode {
  key: string; // path key, unique even when an ancestor appears twice (pedigree collapse)
  person: TreeSummary;
  spouses: TreeSummary[];
  subtype?: string;
  x: number;
  y: number;
  depth: number;
  hasMore: boolean; // further generations exist on the server
  collapsed: boolean; // children hidden by the user
  childCount: number;
}

export interface Edge {
  from: string;
  to: string;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  dashed: boolean;
}

export interface Layout {
  nodes: LaidOutNode[];
  edges: Edge[];
  width: number;
  height: number;
}

/**
 * Lays out a tree left-to-right: generation = column, leaves stacked top to bottom,
 * each parent centred on its children. Grafts replace a node's children (used when
 * the user expands beyond the fetched depth); collapsed keys hide subtrees.
 */
export function layoutTree(root: TreeNode, opts: { collapsed?: Set<string>; grafts?: Map<string, TreeNode[]> } = {}): Layout {
  const collapsed = opts.collapsed ?? new Set<string>();
  const grafts = opts.grafts ?? new Map<string, TreeNode[]>();
  const nodes: LaidOutNode[] = [];
  const edges: Edge[] = [];
  let nextY = 0;
  let maxDepth = 0;

  const visit = (n: TreeNode, key: string, depth: number): LaidOutNode => {
    maxDepth = Math.max(maxDepth, depth);
    const kids = grafts.get(key) ?? n.children;
    const isCollapsed = collapsed.has(key) && kids.length > 0;
    const shown = isCollapsed ? [] : kids;
    const childNodes = shown.map((c, i) => visit(c, `${key}/${i}:${c.person.id}`, depth + 1));
    let y: number;
    if (childNodes.length === 0) {
      y = nextY;
      nextY += NODE_H + GAP_Y;
    } else {
      y = (childNodes[0].y + childNodes[childNodes.length - 1].y) / 2;
    }
    const ln: LaidOutNode = {
      key, person: n.person, spouses: n.spouses ?? [], subtype: n.subtype, x: depth * (NODE_W + GAP_X), y, depth,
      hasMore: n.hasMore && !grafts.has(key), collapsed: isCollapsed, childCount: kids.length,
    };
    nodes.push(ln);
    for (const c of childNodes) {
      edges.push({
        from: key, to: c.key, x1: ln.x + NODE_W, y1: ln.y + NODE_H / 2, x2: c.x, y2: c.y + NODE_H / 2,
        dashed: !!c.subtype && c.subtype !== "biological" && c.subtype !== "unknown",
      });
    }
    return ln;
  };

  visit(root, `0:${root.person.id}`, 0);
  return {
    nodes,
    edges,
    width: (maxDepth + 1) * (NODE_W + GAP_X) - GAP_X,
    height: Math.max(nextY - GAP_Y, NODE_H),
  };
}

/** Smooth horizontal elbow connector. */
export function edgePath(e: Edge): string {
  const mx = (e.x1 + e.x2) / 2;
  return `M${e.x1},${e.y1} C${mx},${e.y1} ${mx},${e.y2} ${e.x2},${e.y2}`;
}

export interface View {
  x: number;
  y: number;
  k: number;
}

export const MIN_ZOOM = 0.2;
export const MAX_ZOOM = 2.5;

export function clampZoom(k: number) {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, k));
}

/** Zoom by factor around a screen point, keeping that point fixed. */
export function zoomAt(v: View, factor: number, px: number, py: number): View {
  const k = clampZoom(v.k * factor);
  const f = k / v.k;
  return { k, x: px - (px - v.x) * f, y: py - (py - v.y) * f };
}

/** View that fits the layout into a viewport with padding. */
export function fitView(l: Layout, vw: number, vh: number, pad = 40): View {
  const k = clampZoom(Math.min((vw - pad * 2) / l.width, (vh - pad * 2) / l.height, 1));
  return { k, x: (vw - l.width * k) / 2, y: (vh - l.height * k) / 2 };
}
