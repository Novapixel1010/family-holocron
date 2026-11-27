import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as RPointerEvent } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { fetchTree, usePerson, usePersons, useTree } from "@/api/hooks";
import type { TreeDirection, TreeNode } from "@/api/types";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { Markdown } from "@/components/ui/Markdown";
import { PageHeader } from "@/components/ui/Page";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { EmptyState, ErrorState, errorMessage, LoadingState, Spinner } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { formatGenDate, lifespan } from "@/lib/format";
import { edgePath, fitView, layoutTree, NODE_H, NODE_W, zoomAt, type LaidOutNode, type View } from "./layout";

const sexStroke: Record<string, string> = { M: "#0284c7", F: "#e11d48", X: "#7c3aed", U: "#a8a29e" };

export function TreePage() {
  const id = Number(useParams().id) || undefined;
  return id ? <TreeView rootId={id} /> : <ChooseRoot />;
}

function ChooseRoot() {
  const { canEdit } = useAuth();
  const navigate = useNavigate();
  const recent = usePersons({ sort: "updated", limit: 1 });
  useEffect(() => {
    const first = recent.data?.items[0];
    if (first) navigate(`/tree/${first.id}`, { replace: true });
  }, [recent.data, navigate]);
  return (
    <PageContainer>
      <PageHeader title="Family tree" />
      {recent.isPending || recent.data?.items.length ? (
        <LoadingState label="Opening the tree…" />
      ) : recent.isError ? (
        <ErrorState error={recent.error} onRetry={() => recent.refetch()} />
      ) : (
        <EmptyState icon="⟟" title="No one to show yet" action={canEdit && <Link to="/people/new" className="rounded-lg bg-brand-600 px-3.5 py-2 text-sm font-medium text-white">Add a person</Link>}>
          The tree appears once people and their parents have been added.
        </EmptyState>
      )}
    </PageContainer>
  );
}

function TreeView({ rootId }: { rootId: number }) {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const direction: TreeDirection = params.get("direction") === "descendants" ? "descendants" : "ancestors";
  const generations = Math.min(8, Math.max(1, Number(params.get("generations")) || 4));
  const tree = useTree(rootId, direction, generations);
  const rootPerson = usePerson(rootId);
  const toast = useToast();

  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [grafts, setGrafts] = useState<Map<string, TreeNode[]>>(new Map());
  const [loadingKey, setLoadingKey] = useState<string | null>(null);
  const [selected, setSelected] = useState<LaidOutNode | null>(null);

  useEffect(() => {
    setCollapsed(new Set());
    setGrafts(new Map());
    setSelected(null);
  }, [rootId, direction, generations]);

  const layout = useMemo(() => (tree.data ? layoutTree(tree.data.root, { collapsed, grafts }) : null), [tree.data, collapsed, grafts]);

  const setParam = (k: string, v: string) => setParams((p) => { const n = new URLSearchParams(p); n.set(k, v); return n; }, { replace: true });

  const toggle = useCallback(async (n: LaidOutNode) => {
    if (n.hasMore) {
      setLoadingKey(n.key);
      try {
        const more = await fetchTree(n.person.id, direction, 3);
        setGrafts((g) => new Map(g).set(n.key, more.root.children));
        setCollapsed((c) => { const s = new Set(c); s.delete(n.key); return s; });
      } catch (e) {
        toast(`Couldn't load more generations: ${errorMessage(e)}`, "error");
      } finally {
        setLoadingKey(null);
      }
      return;
    }
    setCollapsed((c) => {
      const s = new Set(c);
      s.has(n.key) ? s.delete(n.key) : s.add(n.key);
      return s;
    });
  }, [direction, toast]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-end gap-3 border-b border-stone-200 bg-white px-4 py-3 dark:border-stone-800 dark:bg-stone-900">
        <div className="w-full sm:w-72">
          {rootPerson.isPending ? <LoadingState className="py-2" label="Loading…" /> : (
            <PersonPicker label="Root person" value={rootPerson.data ?? null} onChange={(p) => p && navigate(`/tree/${p.id}?${params.toString()}`)} />
          )}
        </div>
        <div className="space-y-1">
          <span className="block text-sm font-medium text-stone-700 dark:text-stone-300">Show</span>
          <div className="flex rounded-lg bg-stone-100 p-1 text-sm dark:bg-stone-800" role="radiogroup" aria-label="Tree direction">
            {(["ancestors", "descendants"] as const).map((d) => (
              <button key={d} role="radio" aria-checked={direction === d} onClick={() => setParam("direction", d)}
                className={`rounded-md px-3 py-1 font-medium capitalize ${direction === d ? "bg-white shadow-sm dark:bg-stone-900" : "text-stone-500"}`}>{d}</button>
            ))}
          </div>
        </div>
        <div className="space-y-1">
          <label htmlFor="gens" className="block text-sm font-medium text-stone-700 dark:text-stone-300">Generations</label>
          <select id="gens" className={`${inputClass} w-24`} value={generations} onChange={(e) => setParam("generations", e.target.value)}>
            {[1, 2, 3, 4, 5, 6, 7, 8].map((g) => <option key={g} value={g}>{g}</option>)}
          </select>
        </div>
        {tree.isFetching && <Spinner className="mb-2 h-5 w-5 text-stone-400" />}
      </div>
      <div className="relative min-h-0 flex-1">
        {tree.isPending ? (
          <LoadingState label="Growing the tree…" className="h-full" />
        ) : tree.isError ? (
          <div className="p-6"><ErrorState error={tree.error} onRetry={() => tree.refetch()} title="Couldn't load the tree" /></div>
        ) : layout ? (
          <>
            {layout.nodes.length === 1 && (
              <div className="pointer-events-none absolute inset-x-0 top-4 z-10 flex justify-center px-4">
                <p className="pointer-events-auto rounded-lg bg-white/95 px-4 py-2 text-sm shadow ring-1 ring-stone-200 dark:bg-stone-900/95 dark:ring-stone-700">
                  No {direction === "ancestors" ? "parents" : "children"} recorded for {tree.data.root.person.name} yet.{" "}
                  <Link className="font-medium text-brand-700 underline dark:text-brand-300" to={`/people/${rootId}/family`}>Add family</Link>
                </p>
              </div>
            )}
            <Canvas key={`${rootId}-${direction}-${generations}`} layout={layout} selectedKey={selected?.key} loadingKey={loadingKey}
              onSelect={setSelected} onToggle={toggle} />
            {selected && <SidePanel node={selected} onClose={() => setSelected(null)} onFocus={() => navigate(`/tree/${selected.person.id}?${params.toString()}`)} />}
          </>
        ) : null}
      </div>
    </div>
  );
}

function Canvas({ layout, selectedKey, loadingKey, onSelect, onToggle }: {
  layout: ReturnType<typeof layoutTree>; selectedKey?: string; loadingKey: string | null;
  onSelect: (n: LaidOutNode) => void; onToggle: (n: LaidOutNode) => void;
}) {
  const box = useRef<HTMLDivElement>(null);
  const [view, setView] = useState<View>({ x: 40, y: 40, k: 1 });
  const drag = useRef<{ x: number; y: number; vx: number; vy: number; moved: boolean } | null>(null);
  const fitted = useRef(false);

  const fit = useCallback(() => {
    const el = box.current;
    if (el) setView(fitView(layout, el.clientWidth, el.clientHeight));
  }, [layout]);

  useEffect(() => {
    if (!fitted.current) {
      fit();
      fitted.current = true;
    }
  }, [fit]);

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      setView((v) => zoomAt(v, Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  const zoomCenter = (f: number) => {
    const el = box.current;
    if (el) setView((v) => zoomAt(v, f, el.clientWidth / 2, el.clientHeight / 2));
  };

  const onPointerDown = (e: RPointerEvent) => {
    if ((e.target as Element).closest("[data-node]")) return;
    drag.current = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y, moved: false };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: RPointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const dx = e.clientX - d.x;
    const dy = e.clientY - d.y;
    if (Math.abs(dx) + Math.abs(dy) > 3) d.moved = true;
    setView((v) => ({ ...v, x: d.vx + dx, y: d.vy + dy }));
  };
  const onPointerUp = () => { drag.current = null; };

  const onKey = (e: React.KeyboardEvent) => {
    const step = 60;
    const map: Record<string, () => void> = {
      ArrowLeft: () => setView((v) => ({ ...v, x: v.x + step })), ArrowRight: () => setView((v) => ({ ...v, x: v.x - step })),
      ArrowUp: () => setView((v) => ({ ...v, y: v.y + step })), ArrowDown: () => setView((v) => ({ ...v, y: v.y - step })),
      "+": () => zoomCenter(1.2), "=": () => zoomCenter(1.2), "-": () => zoomCenter(1 / 1.2), "0": fit,
    };
    if (map[e.key] && !(e.target as Element).closest("[data-node]")) { e.preventDefault(); map[e.key](); }
  };

  return (
    <div ref={box} tabIndex={0} aria-label="Family tree canvas. Drag or use arrow keys to pan, scroll or +/- to zoom."
      className="h-full w-full cursor-grab touch-none select-none overflow-hidden bg-[radial-gradient(circle,_#d6d3d1_1px,_transparent_1px)] [background-size:24px_24px] focus:outline-none active:cursor-grabbing dark:bg-[radial-gradient(circle,_#292524_1px,_transparent_1px)]"
      onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={onPointerUp} onKeyDown={onKey}>
      <svg width="100%" height="100%" role="tree">
        <g transform={`translate(${view.x},${view.y}) scale(${view.k})`}>
          {layout.edges.map((e) => (
            <path key={`${e.from}>${e.to}`} d={edgePath(e)} fill="none" strokeWidth={2} strokeDasharray={e.dashed ? "6 5" : undefined}
              className="stroke-stone-300 dark:stroke-stone-600" />
          ))}
          {layout.nodes.map((n) => (
            <NodeBox key={n.key} n={n} selected={n.key === selectedKey} loading={loadingKey === n.key} onSelect={onSelect} onToggle={onToggle} />
          ))}
        </g>
      </svg>
      <div className="absolute bottom-4 left-4 flex flex-col overflow-hidden rounded-lg bg-white shadow ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700">
        <button className="px-3 py-2 text-lg hover:bg-stone-100 dark:hover:bg-stone-800" onClick={() => zoomCenter(1.25)} aria-label="Zoom in">+</button>
        <button className="px-3 py-2 text-lg hover:bg-stone-100 dark:hover:bg-stone-800" onClick={() => zoomCenter(0.8)} aria-label="Zoom out">−</button>
        <button className="px-3 py-2 text-xs font-medium hover:bg-stone-100 dark:hover:bg-stone-800" onClick={fit} aria-label="Fit tree to screen">Fit</button>
      </div>
      <div className="absolute bottom-4 right-4 hidden rounded-lg bg-white/90 px-3 py-1.5 text-xs text-stone-500 shadow ring-1 ring-stone-200 dark:bg-stone-900/90 dark:ring-stone-700 sm:block">
        {layout.nodes.length} people · {Math.round(view.k * 100)}%
      </div>
    </div>
  );
}

function truncate(s: string, n: number) {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}

function NodeBox({ n, selected, loading, onSelect, onToggle }: {
  n: LaidOutNode; selected: boolean; loading: boolean; onSelect: (n: LaidOutNode) => void; onToggle: (n: LaidOutNode) => void;
}) {
  const p = n.person;
  const years = lifespan({ birthDate: p.birthDate, deathDate: p.deathDate, deceased: !p.living });
  const canToggle = n.childCount > 0 || n.hasMore;
  const spouse = n.spouses.length ? `⚭ ${n.spouses.map((s) => s.givenName || s.name).join(", ")}` : "";
  return (
    <g transform={`translate(${n.x},${n.y})`} data-node>
      <g role="treeitem" aria-selected={selected} aria-label={`${p.name}${years ? `, ${years}` : ""}`} tabIndex={0} className="cursor-pointer focus:outline-none"
        onClick={() => onSelect(n)} onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && (e.preventDefault(), onSelect(n))}>
        <rect width={NODE_W} height={NODE_H} rx={12} strokeWidth={selected ? 3 : 1.5}
          className={selected ? "fill-brand-50 stroke-brand-600 dark:fill-brand-900" : "fill-white stroke-stone-300 dark:fill-stone-900 dark:stroke-stone-700"} />
        <rect width={5} height={NODE_H - 20} x={0} y={10} rx={2} fill={sexStroke[p.sex] ?? sexStroke.U} />
        <text x={16} y={26} className="fill-stone-900 text-[14px] font-semibold dark:fill-stone-100">{truncate(p.name, 24)}</text>
        <text x={16} y={45} className="fill-stone-500 text-[12px] dark:fill-stone-400">{years || (p.living ? "Living" : "Dates unknown")}</text>
        {spouse && <text x={16} y={60} className="fill-stone-400 text-[11px]">{truncate(spouse, 30)}</text>}
      </g>
      {canToggle && (
        <g transform={`translate(${NODE_W},${NODE_H / 2})`} role="button" tabIndex={0} className="cursor-pointer focus:outline-none"
          aria-label={n.hasMore ? `Load more generations for ${p.name}` : n.collapsed ? `Expand ${p.name}` : `Collapse ${p.name}`}
          onClick={(e) => { e.stopPropagation(); if (!loading) onToggle(n); }}
          onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && (e.preventDefault(), !loading && onToggle(n))}>
          <circle r={11} className="fill-white stroke-stone-400 dark:fill-stone-800 dark:stroke-stone-500" strokeWidth={1.5} />
          {loading ? (
            <circle r={6} fill="none" strokeWidth={2} strokeDasharray="20 10" className="animate-spin stroke-brand-600" style={{ transformOrigin: "0 0" }} />
          ) : (
            <text textAnchor="middle" dominantBaseline="central" className="fill-stone-600 text-[14px] font-bold dark:fill-stone-200">
              {n.hasMore || n.collapsed ? "+" : "−"}
            </text>
          )}
        </g>
      )}
    </g>
  );
}

function SidePanel({ node, onClose, onFocus }: { node: LaidOutNode; onClose: () => void; onFocus: () => void }) {
  const person = usePerson(node.person.id);
  return (
    <aside aria-label="Selected person" className="absolute inset-x-2 bottom-2 z-20 max-h-[60%] overflow-y-auto rounded-2xl bg-white p-4 shadow-xl ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700 sm:inset-x-auto sm:bottom-auto sm:right-4 sm:top-4 sm:w-80">
      <div className="flex items-start gap-3">
        <Avatar name={node.person.name} sex={node.person.sex} mediaId={node.person.portraitMediaId} size={52} />
        <div className="min-w-0 flex-1">
          <h2 className="font-serif text-lg font-semibold leading-tight">{node.person.name}</h2>
          <p className="text-sm text-stone-500">{lifespan({ birthDate: node.person.birthDate, deathDate: node.person.deathDate, deceased: !node.person.living }) || "Dates unknown"}</p>
        </div>
        <button onClick={onClose} className="rounded p-1 text-stone-500 hover:bg-stone-100 dark:hover:bg-stone-800" aria-label="Close panel">✕</button>
      </div>
      <div className="mt-3 text-sm">
        {person.isPending ? (
          <LoadingState className="py-4" />
        ) : person.isError ? (
          <ErrorState error={person.error} onRetry={() => person.refetch()} />
        ) : (
          <div className="space-y-2">
            {(person.data.birthDate || person.data.birthPlace) && <p><span className="font-medium">Born:</span> {[formatGenDate(person.data.birthDate), person.data.birthPlace].filter(Boolean).join(", ")}</p>}
            {(person.data.deathDate || person.data.deathPlace) && <p><span className="font-medium">Died:</span> {[formatGenDate(person.data.deathDate), person.data.deathPlace].filter(Boolean).join(", ")}</p>}
            {person.data.about ? (
              <div className="line-clamp-6 text-stone-600 dark:text-stone-300"><Markdown>{person.data.about.slice(0, 600)}</Markdown></div>
            ) : (
              <p className="text-stone-500">No biography written yet.</p>
            )}
          </div>
        )}
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Link to={`/people/${node.person.id}`} className="rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700">Open profile</Link>
        <Button size="sm" onClick={onFocus}>Center tree here</Button>
      </div>
    </aside>
  );
}
