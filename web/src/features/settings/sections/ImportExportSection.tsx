import { useRef, useState } from "react";
import { download } from "@/api/client";
import { useImportGedcom } from "@/api/hooks";
import type { ImportResult } from "@/api/types";
import { useAuth } from "@/app/auth";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Page";
import { errorMessage, InlineError } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

export function ImportExportSection() {
  const { canEdit } = useAuth();
  const imp = useImportGedcom();
  const input = useRef<HTMLInputElement>(null);
  const toast = useToast();
  const [result, setResult] = useState<ImportResult | null>(null);
  const [exporting, setExporting] = useState<"" | "ged" | "json">("");

  const exp = async (kind: "ged" | "json") => {
    setExporting(kind);
    try {
      const name = await download(kind === "ged" ? "/api/v1/gedcom/export" : "/api/v1/export/json", kind === "ged" ? "family-holocron.ged" : "family-holocron.json");
      toast(`Downloaded ${name}`);
    } catch (e) {
      toast(`Export failed: ${errorMessage(e)}`, "error");
    } finally {
      setExporting("");
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <h2 className="font-semibold">Export</h2>
        <p className="mb-4 mt-1 text-sm text-stone-500">Your records are never locked in. GEDCOM 5.5.1 works with nearly every genealogy program; JSON includes every field and attribution.</p>
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => exp("ged")} loading={exporting === "ged"}>Download GEDCOM (.ged)</Button>
          <Button onClick={() => exp("json")} loading={exporting === "json"}>Download JSON</Button>
        </div>
      </Card>
      <Card>
        <h2 className="font-semibold">Import GEDCOM</h2>
        <p className="mb-4 mt-1 text-sm text-stone-500">
          Adds the people, families, events and notes in a .ged file to this archive (nothing existing is changed). Everything imported is attributed to you.
        </p>
        {canEdit ? (
          <>
            <input ref={input} type="file" accept=".ged,.gedcom,text/plain" className="hidden" onChange={async (e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (!f) return;
              setResult(null);
              try { const r = await imp.mutateAsync(f); setResult(r); toast(`Imported ${r.persons} people`); } catch { /* inline */ }
            }} />
            <Button variant="primary" onClick={() => input.current?.click()} loading={imp.isPending}>{imp.isPending ? "Importing…" : "Choose a GEDCOM file"}</Button>
            <div className="mt-3"><InlineError error={imp.error} /></div>
            {result && (
              <div className="mt-4 rounded-lg bg-brand-50 p-4 text-sm dark:bg-brand-900/30">
                <p className="font-medium">Import complete: {result.persons} people, {result.relationships} relationships, {result.events} events, {result.notes} notes.</p>
                {result.warnings.length > 0 && (
                  <details className="mt-2">
                    <summary className="cursor-pointer text-amber-800 dark:text-amber-300">{result.warnings.length} item(s) skipped</summary>
                    <ul className="mt-1 list-disc pl-5 text-xs">{result.warnings.slice(0, 100).map((w, i) => <li key={i}>{w}</li>)}</ul>
                  </details>
                )}
              </div>
            )}
          </>
        ) : (
          <p className="text-sm text-stone-500">Only editors and admins can import.</p>
        )}
      </Card>
    </div>
  );
}
