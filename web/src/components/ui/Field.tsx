import { forwardRef, useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";

export const inputClass =
  "block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-stone-900 ring-1 ring-inset ring-stone-300 placeholder:text-stone-400 focus:ring-2 focus:ring-brand-500 disabled:opacity-60 dark:bg-stone-900 dark:text-stone-100 dark:ring-stone-700";

export function Field({ label, error, hint, children, htmlFor }: { label: string; error?: string; hint?: ReactNode; children: ReactNode; htmlFor?: string }) {
  return (
    <div className="space-y-1">
      <label htmlFor={htmlFor} className="block text-sm font-medium text-stone-700 dark:text-stone-300">
        {label}
      </label>
      {children}
      {error ? <p className="text-xs text-red-600 dark:text-red-400">{error}</p> : hint ? <p className="text-xs text-stone-500">{hint}</p> : null}
    </div>
  );
}

type InputProps = InputHTMLAttributes<HTMLInputElement> & { label: string; error?: string; hint?: ReactNode };

export const TextInput = forwardRef<HTMLInputElement, InputProps>(function TextInput({ label, error, hint, id, className = "", ...rest }, ref) {
  const gen = useId();
  const fid = id ?? gen;
  return (
    <Field label={label} error={error} hint={hint} htmlFor={fid}>
      <input ref={ref} id={fid} aria-invalid={!!error || undefined} className={`${inputClass} ${error ? "ring-red-400" : ""} ${className}`} {...rest} />
    </Field>
  );
});

type AreaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & { label: string; error?: string; hint?: ReactNode };

export function TextArea({ label, error, hint, id, className = "", ...rest }: AreaProps) {
  const gen = useId();
  const fid = id ?? gen;
  return (
    <Field label={label} error={error} hint={hint} htmlFor={fid}>
      <textarea id={fid} aria-invalid={!!error || undefined} className={`${inputClass} ${className}`} {...rest} />
    </Field>
  );
}

type SelectProps = SelectHTMLAttributes<HTMLSelectElement> & { label: string; error?: string; hint?: ReactNode; options: { value: string; label: string }[] };

export function Select({ label, error, hint, id, options, className = "", ...rest }: SelectProps) {
  const gen = useId();
  const fid = id ?? gen;
  return (
    <Field label={label} error={error} hint={hint} htmlFor={fid}>
      <select id={fid} className={`${inputClass} ${className}`} {...rest}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </Field>
  );
}
