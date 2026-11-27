import { useState } from "react";
import { toPersonInput, useUpdatePerson } from "@/api/hooks";
import { Button } from "@/components/ui/Button";
import { Markdown, MarkdownEditor } from "@/components/ui/Markdown";
import { EmptyState, InlineError } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { personName } from "@/lib/format";
import type { PersonTabProps } from "@/plugins/registry";
import { InsertTemplateButton } from "../InsertTemplateButton";

export function AboutTab({ person, canEdit }: PersonTabProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(person.about);
  const update = useUpdatePerson();
  const toast = useToast();

  if (editing) {
    return (
      <div className="space-y-3">
        <MarkdownEditor label={`About ${personName(person)}`} value={draft} onChange={setDraft} rows={14}
          toolbar={<InsertTemplateButton value={draft} onChange={setDraft} />}
          placeholder="Write their story: where they grew up, what they did, what people remember about them…" />
        <InlineError error={update.error} />
        <div className="flex justify-end gap-2">
          <Button onClick={() => { setEditing(false); setDraft(person.about); update.reset(); }} disabled={update.isPending}>Cancel</Button>
          <Button variant="primary" loading={update.isPending}
            onClick={async () => {
              try {
                await update.mutateAsync({ id: person.id, input: { ...toPersonInput(person), about: draft } });
                toast("Biography saved");
                setEditing(false);
              } catch { /* inline */ }
            }}>Save biography</Button>
        </div>
      </div>
    );
  }
  if (!person.about.trim()) {
    return (
      <EmptyState icon="✎" title="No biography yet"
        action={canEdit && <Button variant="primary" onClick={() => { setDraft(""); setEditing(true); }}>Write their story</Button>}>
        The About section holds family stories, memories and personal context about {personName(person)}.
      </EmptyState>
    );
  }
  return (
    <div>
      {canEdit && <div className="mb-2 flex justify-end"><Button size="sm" onClick={() => { setDraft(person.about); setEditing(true); }}>Edit biography</Button></div>}
      <article className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-800">
        <Markdown>{person.about}</Markdown>
      </article>
    </div>
  );
}
