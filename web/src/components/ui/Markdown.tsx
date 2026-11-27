import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { useState } from "react";
import { TextArea } from "./Field";

/** Renders user Markdown safely (raw HTML is not rendered). */
export function Markdown({ children, className = "" }: { children: string; className?: string }) {
  return (
    <div className={`prose-holocron ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{ a: ({ node: _n, ...props }) => <a {...props} target="_blank" rel="noopener noreferrer nofollow" /> }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}

/** Textarea with a Write / Preview toggle. */
export function MarkdownEditor({
  label, value, onChange, rows = 10, placeholder, error,
}: { label: string; value: string; onChange: (v: string) => void; rows?: number; placeholder?: string; error?: string }) {
  const [preview, setPreview] = useState(false);
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-end gap-1 text-xs">
        <button type="button" onClick={() => setPreview(false)} className={`rounded px-2 py-0.5 ${!preview ? "bg-stone-200 dark:bg-stone-700" : "text-stone-500"}`}>
          Write
        </button>
        <button type="button" onClick={() => setPreview(true)} className={`rounded px-2 py-0.5 ${preview ? "bg-stone-200 dark:bg-stone-700" : "text-stone-500"}`}>
          Preview
        </button>
      </div>
      {preview ? (
        <div>
          <div className="mb-1 text-sm font-medium text-stone-700 dark:text-stone-300">{label}</div>
          <div className="min-h-[8rem] rounded-lg bg-stone-50 p-3 ring-1 ring-stone-200 dark:bg-stone-900 dark:ring-stone-700">
            {value.trim() ? <Markdown>{value}</Markdown> : <p className="text-sm text-stone-400">Nothing to preview yet.</p>}
          </div>
        </div>
      ) : (
        <TextArea label={label} value={value} onChange={(e) => onChange(e.target.value)} rows={rows} placeholder={placeholder} error={error}
          hint="Markdown supported: **bold**, *italic*, # headings, - lists, [links](https://…)" className="font-mono" />
      )}
    </div>
  );
}
