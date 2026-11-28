// Minimal fetch wrapper. Every call sends the session cookie and throws ApiError on failure.

export class ApiError extends Error {
  status: number;
  code: string;
  fields: Record<string, string>;
  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

async function parseError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json();
    const e = body?.error ?? {};
    return new ApiError(res.status, e.code ?? "error", e.message ?? res.statusText, e.fields ?? {});
  } catch {
    return new ApiError(res.status, "error", res.status >= 500 ? "The server could not be reached or had an error." : res.statusText);
  }
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body !== undefined && !(body instanceof FormData) ? { "Content-Type": "application/json" } : undefined,
      body: body === undefined ? undefined : body instanceof FormData ? body : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network", "Could not reach the Family Holocron server. Check your connection.");
  }
  if (!res.ok) {
    const err = await parseError(res);
    if (res.status === 401 && !path.startsWith("/api/v1/auth/")) {
      window.dispatchEvent(new CustomEvent("holocron:unauthorized"));
    }
    // e.g. an admin assigned a temporary password while this tab was open.
    if (res.status === 403 && err.code === "password_change_required") {
      window.dispatchEvent(new CustomEvent("holocron:password-change-required"));
    }
    throw err;
  }
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get("Content-Type") ?? "";
  return (ct.includes("application/json") ? res.json() : res.text()) as Promise<T>;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body ?? {}),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, body),
  del: (path: string) => request<void>("DELETE", path),
  upload: <T>(path: string, form: FormData) => request<T>("POST", path, form),
};

export function qs(params: Record<string, string | number | undefined | null>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== "") p.set(k, String(v));
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}

export function mediaFileUrl(id: number, download = false): string {
  return `/api/v1/media/${id}/file${download ? "?download=1" : ""}`;
}

/** Triggers a browser download of a GET endpoint, surfacing API errors. */
export async function download(path: string, fallbackName: string): Promise<string> {
  let res: Response;
  try {
    res = await fetch(path, { credentials: "same-origin" });
  } catch {
    throw new ApiError(0, "network", "Could not reach the server.");
  }
  if (!res.ok) throw await parseError(res);
  const blob = await res.blob();
  const cd = res.headers.get("Content-Disposition") ?? "";
  const m = /filename="?([^";]+)"?/.exec(cd);
  const name = m?.[1] ?? fallbackName;
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  return name;
}
