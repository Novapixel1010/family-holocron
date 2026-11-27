import { useState, type FormEvent } from "react";
import { ApiError } from "@/api/client";
import { useBioTemplateMutations, useBioTemplates } from "@/api/hooks";
import type { BioTemplate, BioTemplateInput } from "@/api/types";
import { Attribution } from "@/components/ui/Attribution";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/Field";
import { MarkdownEditor } from "@/components/ui/Markdown";
import { ConfirmDialog, Modal } from "@/components/ui/Modal";
import { Card } from "@/components/ui/Page";
import { EmptyState, InlineError, QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";

const EMPTY: BioTemplateInput = { name: "", description: "", contentMd: "", sortOrder: 0 };

/** Admin management of the Markdown templates offered in every person's About editor. */
export function BioTemplatesSection() {
  const templates = useBioTemplates();
  const [editing, setEditing] = useState<{ template?: BioTemplate } | null>(null);
  const [deleting, setDeleting] = useState<BioTemplate | null>(null);
  const { remove } = useBioTemplateMutations();
  const toast = useToast();
  return (
    <Card>
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="font-semibold">Biography templates</h2>
          <p className="mt-1 max-w-xl text-sm text-stone-500">
            Reusable Markdown outlines that anyone can insert while writing a person’s About section — headings and prompts that make it easier to start a family story.
          </p>
        </div>
        <Button variant="primary" onClick={() => setEditing({})}>+ New template</Button>
      </div>
      <QueryBoundary query={templates} errorTitle="Couldn't load templates">
        {(list) => list.length === 0 ? (
          <EmptyState icon="✎" title="No templates yet" action={<Button variant="primary" onClick={() => setEditing({})}>Create the first template</Button>}>
            Without templates, the “Insert template” button is hidden from the About editor.
          </EmptyState>
        ) : (
          <ul className="divide-y divide-stone-200 dark:divide-stone-800">
            {list.map((t) => (
              <li key={t.id} className="flex flex-wrap items-start gap-3 py-3">
                <div className="min-w-0 flex-1">
                  <div className="font-medium">{t.name} <span className="ml-1 text-xs font-normal text-stone-400" title="Sort order">#{t.sortOrder}</span></div>
                  {t.description && <p className="text-sm text-stone-500">{t.description}</p>}
                  <p className="mt-0.5 truncate font-mono text-xs text-stone-400">{t.contentMd.split("\n").filter((l) => l.startsWith("#")).map((l) => l.replace(/^#+\s*/, "")).join(" · ") || t.contentMd.slice(0, 80)}</p>
                  <div className="mt-1">{t.createdBy ? <Attribution value={t} /> : <p className="text-xs text-stone-500">Starter template added by Family Holocron</p>}</div>
                </div>
                <div className="flex gap-1">
                  <Button size="sm" onClick={() => setEditing({ template: t })}>Edit</Button>
                  <Button size="sm" variant="ghost" className="text-red-700 dark:text-red-400" onClick={() => setDeleting(t)}>Delete</Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </QueryBoundary>
      {editing && <TemplateEditor template={editing.template} onClose={() => setEditing(null)} />}
      <ConfirmDialog open={!!deleting} title={`Delete “${deleting?.name ?? ""}”?`} onClose={() => { setDeleting(null); remove.reset(); }} loading={remove.isPending}
        error={<InlineError error={remove.error} />}
        onConfirm={async () => {
          try { await remove.mutateAsync(deleting!.id); toast("Template deleted"); setDeleting(null); } catch { /* inline */ }
        }}>
        <p>It will no longer be offered in the About editor. Biographies already written with it are not changed.</p>
      </ConfirmDialog>
    </Card>
  );
}

function TemplateEditor({ template, onClose }: { template?: BioTemplate; onClose: () => void }) {
  const [form, setForm] = useState<BioTemplateInput>(template ? { name: template.name, description: template.description, contentMd: template.contentMd, sortOrder: template.sortOrder } : EMPTY);
  const { create, update } = useBioTemplateMutations();
  const toast = useToast();
  const m = template ? update : create;
  const fields = m.error instanceof ApiError ? m.error.fields : {};
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      if (template) await update.mutateAsync({ id: template.id, input: form });
      else await create.mutateAsync(form);
      toast(template ? "Template saved" : "Template created");
      onClose();
    } catch { /* inline */ }
  };
  return (
    <Modal open onClose={onClose} title={template ? `Edit “${template.name}”` : "New biography template"} size="lg">
      <form onSubmit={submit} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-[1fr,8rem]">
          <TextInput label="Name" required maxLength={80} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} error={fields.name}
            placeholder="e.g. Quick Memories" autoFocus />
          <TextInput label="Sort order" type="number" value={String(form.sortOrder)} onChange={(e) => setForm({ ...form, sortOrder: Number(e.target.value) || 0 })}
            error={fields.sortOrder} hint="Lower shows first" />
        </div>
        <TextInput label="Description (optional)" maxLength={200} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
          error={fields.description} placeholder="Shown under the name in the Insert template menu" />
        <MarkdownEditor label="Template content" value={form.contentMd} onChange={(v) => setForm({ ...form, contentMd: v })} rows={14} error={fields.contentMd}
          placeholder={"## Places Lived\n- _Town — years — why they moved_\n\n## Memorable Stories\n_A story the family still tells…_"} />
        {template && <Attribution value={template} />}
        <InlineError error={m.error} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose} disabled={m.isPending}>Cancel</Button>
          <Button type="submit" variant="primary" loading={m.isPending}>{template ? "Save template" : "Create template"}</Button>
        </div>
      </form>
    </Modal>
  );
}

