export type InsertMode = "append" | "replace";

/**
 * Combines existing About text with a template. Empty text simply becomes the
 * template; "append" keeps every existing character and adds the template after
 * a blank line; "replace" swaps it out (only ever on the user's explicit choice).
 */
export function applyTemplate(current: string, template: string, mode: InsertMode): string {
  if (!current.trim() || mode === "replace") return template;
  return `${current.replace(/\s+$/, "")}\n\n${template}`;
}
