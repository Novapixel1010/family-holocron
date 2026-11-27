import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { Background, BackgroundVariant, Panel, ReactFlow, ReactFlowProvider, useReactFlow, useStore, useViewport, type NodeTypes } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import "./tree.css";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { fetchTree, usePerson, usePersons, useTree } from "@/api/hooks";
import type { GraphPerson, TreeDirection } from "@/api/types";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { inputClass } from "@/components/ui/Field";
import { InfoTip } from "@/components/ui/InfoTip";
import { Markdown } from "@/components/ui/Markdown";
import { PageHeader } from "@/components/ui/Page";
import { PersonPicker } from "@/components/ui/PersonPicker";
import { EmptyState, ErrorState, errorMessage, LoadingState, Spinner } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { formatVitalDate, vitalRange } from "@/lib/dates";
import { buildFlow, mergeGraph, visibleGraph, type FlowResult, type PersonNodeData, type TreeGraph } from "./graph";
import { PersonNode } from "./PersonNode";
import { TreeActionsContext, useTreeActions, type TreeActions } from "./TreeContext";
import { UnionNode } from "./UnionNode";

// Defined once at module level so React Flow never remounts nodes on pan/zoom.
const nodeTypes: NodeTypes = { person: PersonNode, union: UnionNode };

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

  // The graph is the server response plus any "load more" sub-graphs merged into it.
  // Merges and collapse state are tied to the response they were made against, so a new
  // response (new root, direction or depth) starts clean without an intermediate frame.
  const base = useMemo<TreeGraph | null>(() => (tree.data ? { people: tree.data.people, links: tree.data.links } : null), [tree.data]);
  const [merged, setMerged] = useState<{ source: TreeGraph; graph: TreeGraph } | null>(null);
  const [collapsedState, setCollapsedState] = useState<{ source: TreeGraph | null; ids: Set<number> }>({ source: null, ids: new Set() });
  const graph = base && merged?.source === base ? merged.graph : base;
  const collapsed = useMemo(() => (collapsedState.source === base ? collapsedState.ids : new Set<number>()), [collapsedState, base]);
  const setCollapsed = useCallback((fn: (c: Set<number>) => Set<number>) => setCollapsedState((st) => ({ source: base, ids: fn(st.source === base ? st.ids : new Set()) })), [base]);
  const [loadingId, setLoadingId] = useState<number | null>(null);
  const [selected, setSelected] = useState<GraphPerson | null>(null);
  useEffect(() => setSelected(null), [rootId, direction, generations]);

  // While a new root is loading, react-query shows the previous tree; key layout to the data actually shown.
  const shownRoot = tree.data?.root.person.id ?? rootId;
  const flow = useMemo<FlowResult | null>(
    () => (graph && tree.data ? buildFlow(visibleGraph(graph, shownRoot, tree.data.direction, collapsed), shownRoot, tree.data.direction) : null),
    [graph, collapsed, shownRoot, tree.data],
  );
  const fitKey = tree.data ? `${shownRoot}-${tree.data.direction}-${tree.data.generations}` : "";

  const setParam = (k: string, v: string) => setParams((p) => { const n = new URLSearchParams(p); n.set(k, v); return n; }, { replace: true });

  const onToggle = useCallback(async (p: GraphPerson) => {
    if (collapsed.has(p.id)) {
      setCollapsed((c) => { const s = new Set(c); s.delete(p.id); return s; });
      return;
    }
    if (p.hasMore) {
      setLoadingId(p.id);
      try {
        const more = await fetchTree(p.id, direction, 3);
        if (base) {
          setMerged((m) => ({ source: base, graph: mergeGraph(m?.source === base ? m.graph : base, { people: more.people, links: more.links }, p.id) }));
        }
      } catch (e) {
        toast(`Couldn't load more generations: ${errorMessage(e)}`, "error");
      } finally {
        setLoadingId(null);
      }
      return;
    }
    setCollapsed((c) => new Set(c).add(p.id));
  }, [base, collapsed, direction, setCollapsed, toast]);

  const actions = useMemo<TreeActions>(() => ({
    direction, selectedId: selected?.id ?? null, loadingId, collapsed,
    onSelect: setSelected, onToggle,
    onFocus: (p) => navigate(`/tree/${p.id}?${params.toString()}`),
  }), [direction, selected, loadingId, collapsed, onToggle, navigate, params]);

  return (
    <div className="flex h-full flex-col">
      {/* The toolbar is a sibling of the canvas (never inside <ReactFlow>) and stacks above it,
          so the picker dropdown and tooltips can't be covered or have their clicks swallowed. */}
      <div className="relative z-30 flex flex-wrap items-end gap-3 border-b border-stone-200 bg-white px-4 py-3 dark:border-stone-800 dark:bg-stone-900">
        <div className="w-full sm:w-72">
          {rootPerson.isPending ? <LoadingState className="py-2" label="Loading…" /> : (
            <PersonPicker label="Root person" clearable={false} value={rootPerson.data ?? null}
              onChange={(p) => p && p.id !== rootId && navigate(`/tree/${p.id}?${params.toString()}`)} />
          )}
        </div>
        <div className="space-y-1">
          <span className="block text-sm font-medium text-stone-700 dark:text-stone-300">Show</span>
          <div className="flex items-center gap-1 rounded-lg bg-stone-100 p-1 text-sm dark:bg-stone-800">
            <div className="flex" role="radiogroup" aria-label="Tree direction">
              {(["ancestors", "descendants"] as const).map((d) => (
                <span key={d} className="flex items-center gap-1 pr-1">
                  <button role="radio" aria-checked={direction === d} onClick={() => setParam("direction", d)}
                    className={`rounded-md px-3 py-1 font-medium capitalize ${direction === d ? "bg-white shadow-sm dark:bg-stone-900" : "text-stone-500"}`}>{d}</button>
                  <InfoTip label={`About the ${d} view`} align={d === "ancestors" ? "left" : "center"}>{DIRECTION_HELP[d]}</InfoTip>
                </span>
              ))}
            </div>
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
      {/* isolate: React Flow's internal z-indexes stay inside the canvas's own stacking context. */}
      <div className="relative isolate min-h-0 flex-1">
        {tree.isPending ? (
          <LoadingState label="Growing the tree…" className="h-full" />
        ) : tree.isError ? (
          <div className="p-6"><ErrorState error={tree.error} onRetry={() => tree.refetch()} title="Couldn't load the tree" /></div>
        ) : flow ? (
          <>
            {flow.nodes.length === 1 && (
              <div className="pointer-events-none absolute inset-x-0 top-4 z-10 flex justify-center px-4">
                <p className="pointer-events-auto rounded-lg bg-white/95 px-4 py-2 text-sm shadow ring-1 ring-stone-200 dark:bg-stone-900/95 dark:ring-stone-700">
                  No {direction === "ancestors" ? "parents" : "children"} recorded for {tree.data.root.person.name} yet.{" "}
                  <Link className="font-medium text-brand-700 underline dark:text-brand-300" to={`/people/${rootId}/family`}>Add family</Link>
                </p>
              </div>
            )}
            <TreeActionsContext.Provider value={actions}>
              <ReactFlowProvider>
                <TreeFlow flow={flow} fitKey={fitKey} />
              </ReactFlowProvider>
            </TreeActionsContext.Provider>
            {selected && <SidePanel person={selected} onClose={() => setSelected(null)} onFocus={() => navigate(`/tree/${selected.id}?${params.toString()}`)} />}
          </>
        ) : null}
      </div>
    </div>
  );
}

const DIRECTION_HELP: Record<TreeDirection, string> = {
  ancestors: "Shows people this person came from — e.g., selecting Mom shows her parents and grandparents.",
  descendants: "Shows people who came from this person — e.g., selecting Mom shows her children and grandchildren.",
};

const FIT = { padding: 0.15, maxZoom: 1.1, duration: 250 } as const;

function TreeFlow({ flow, fitKey }: { flow: FlowResult; fitKey: string }) {
  const rf = useReactFlow();
  const actions = useTreeActions();
  const { zoom } = useViewport();
  // Nodes carry explicit Dagre dimensions, so fitting only needs them in the store and a sized pane.
  const nodeCount = useStore((st) => st.nodeLookup.size);
  const paneReady = useStore((st) => st.width > 0 && st.height > 0);
  const pendingFit = useRef<string | null>(fitKey);
  const people = useMemo(() => flow.nodes.filter((n) => n.type === "person").length, [flow]);

  // Fit whenever the root person, direction or depth changes.
  useEffect(() => { pendingFit.current = fitKey; }, [fitKey]);
  useEffect(() => {
    if (!paneReady || nodeCount === 0 || pendingFit.current === null) return;
    // Clear the pending flag only once the fit has actually run: a re-render that
    // cancels this frame must not drop the fit.
    const id = requestAnimationFrame(() => {
      pendingFit.current = null;
      rf.fitView(FIT);
    });
    return () => cancelAnimationFrame(id);
  }, [paneReady, nodeCount, flow, fitKey, rf]);

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.target as HTMLElement).closest("button, input, select, a")) return;
    const step = 60;
    const pan = (dx: number, dy: number) => {
      const v = rf.getViewport();
      rf.setViewport({ ...v, x: v.x + dx, y: v.y + dy }, { duration: 100 });
    };
    const map: Record<string, () => void> = {
      ArrowLeft: () => pan(step, 0), ArrowRight: () => pan(-step, 0), ArrowUp: () => pan(0, step), ArrowDown: () => pan(0, -step),
      "+": () => rf.zoomIn(), "=": () => rf.zoomIn(), "-": () => rf.zoomOut(), "0": () => rf.fitView(FIT),
    };
    if (map[e.key]) { e.preventDefault(); map[e.key](); }
  };

  return (
    <div className="h-full w-full" tabIndex={0} onKeyDown={onKeyDown}
      aria-label="Family tree canvas. Drag or use arrow keys to pan, scroll or +/- to zoom, 0 to fit.">
      <ReactFlow
        className="holocron-flow"
        nodes={flow.nodes}
        edges={flow.edges}
        nodeTypes={nodeTypes}
        colorMode="system"
        minZoom={0.1}
        maxZoom={2.5}
        nodesDraggable={false}
        nodesConnectable={false}
        nodesFocusable={false}
        edgesFocusable={false}
        elementsSelectable={false}
        disableKeyboardA11y
        zoomOnDoubleClick={false}
        // A click handler also keeps nodes interactive (React Flow disables pointer
        // events on nodes that are neither selectable, draggable nor clickable).
        onNodeClick={(_, node) => node.type === "person" && actions.onSelect((node.data as PersonNodeData).person)}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1} />
        <Panel position="bottom-left">
          <div className="flex flex-col overflow-hidden rounded-lg bg-white shadow ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700">
            <button className="px-3 py-2 text-lg hover:bg-stone-100 dark:hover:bg-stone-800" onClick={() => rf.zoomIn({ duration: 150 })} aria-label="Zoom in">+</button>
            <button className="px-3 py-2 text-lg hover:bg-stone-100 dark:hover:bg-stone-800" onClick={() => rf.zoomOut({ duration: 150 })} aria-label="Zoom out">−</button>
            <button className="px-3 py-2 text-xs font-medium hover:bg-stone-100 dark:hover:bg-stone-800" onClick={() => rf.fitView(FIT)} aria-label="Fit tree to screen">Fit</button>
          </div>
        </Panel>
        <Panel position="top-left">
          <div className="hidden rounded-lg bg-white/90 px-3 py-1.5 text-xs text-stone-500 shadow ring-1 ring-stone-200 dark:bg-stone-900/90 dark:ring-stone-700 sm:block">
            {people} {people === 1 ? "person" : "people"} · {Math.round(zoom * 100)}%
          </div>
        </Panel>
      </ReactFlow>
    </div>
  );
}

function SidePanel({ person: node, onClose, onFocus }: { person: GraphPerson; onClose: () => void; onFocus: () => void }) {
  const person = usePerson(node.id);
  return (
    <aside aria-label="Selected person" className="absolute inset-x-2 bottom-2 z-20 max-h-[60%] overflow-y-auto rounded-2xl bg-white p-4 shadow-xl ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700 sm:inset-x-auto sm:bottom-auto sm:right-4 sm:top-4 sm:w-80">
      <div className="flex items-start gap-3">
        <Avatar name={node.name} sex={node.sex} mediaId={node.portraitMediaId} size={52} />
        <div className="min-w-0 flex-1">
          <h2 className="font-serif text-lg font-semibold leading-tight">{node.name}</h2>
          <p className="text-sm tabular-nums text-stone-500">
            {/* Each date stays on one line; wrapping may only happen at the dash between them. */}
            {(vitalRange({ birthDate: node.birthDate, deathDate: node.deathDate, deceased: !node.living }) || "Dates unknown")
              .split(" – ").map((part, i) => <span key={i} className="whitespace-nowrap">{i > 0 && " – "}{part}</span>)}
          </p>
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
            {(person.data.birthDate || person.data.birthPlace) && <p><span className="font-medium">Born:</span> {[formatVitalDate(person.data.birthDate), person.data.birthPlace].filter(Boolean).join(", ")}</p>}
            {(person.data.deathDate || person.data.deathPlace) && <p><span className="font-medium">Died:</span> {[formatVitalDate(person.data.deathDate), person.data.deathPlace].filter(Boolean).join(", ")}</p>}
            {person.data.about ? (
              <div className="line-clamp-6 text-stone-600 dark:text-stone-300"><Markdown>{person.data.about.slice(0, 600)}</Markdown></div>
            ) : (
              <p className="text-stone-500">No biography written yet.</p>
            )}
          </div>
        )}
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Link to={`/people/${node.id}`} className="rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700">Open profile</Link>
        <Button size="sm" onClick={onFocus}>Center tree here</Button>
      </div>
    </aside>
  );
}
