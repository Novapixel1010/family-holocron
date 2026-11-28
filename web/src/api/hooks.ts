import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { api, qs } from "./client";
import type {
  AccessToken, AuditPage, BackupInfo, BioTemplate, BioTemplateInput, CreatedAccessToken, EventInput, Family, HolocronEvent, ImportResult, Media, MediaInput, Note, Page, Person,
  PersonInput, PluginInfo, Relationship, RelationshipInput, RestoreResult, Stats, Status, TimelineResponse,
  TreeDirection, TreeResponse, User,
} from "./types";

// ---- status & auth ---------------------------------------------------------------

export const useStatus = () => useQuery({ queryKey: ["status"], queryFn: () => api.get<Status>("/api/v1/status") });

export const useMe = (enabled = true) =>
  useQuery({ queryKey: ["me"], queryFn: () => api.get<User>("/api/v1/auth/me"), enabled, retry: false });

export function useInvalidateAll() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries();
}

// After any mutation of genealogical data, refresh everything that might show it.
function useDataMutation<TVars, TResult>(fn: (v: TVars) => Promise<TResult>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ predicate: (q) => !["status", "me", "backups"].includes(String(q.queryKey[0])) });
    },
  });
}

// ---- people ----------------------------------------------------------------------

export const usePersons = (params: { q?: string; sort?: string; limit?: number; offset?: number; ids?: string }, enabled = true) =>
  useQuery({
    queryKey: ["persons", params],
    queryFn: () => api.get<Page<Person>>(`/api/v1/persons${qs(params)}`),
    placeholderData: keepPreviousData,
    enabled,
  });

export const usePerson = (id: number | undefined) =>
  useQuery({ queryKey: ["person", id], queryFn: () => api.get<Person>(`/api/v1/persons/${id}`), enabled: !!id });

export const useFamily = (id: number) =>
  useQuery({ queryKey: ["family", id], queryFn: () => api.get<Family>(`/api/v1/persons/${id}/family`) });

export const useCreatePerson = () => useDataMutation((in_: PersonInput) => api.post<Person>("/api/v1/persons", in_));
export const useUpdatePerson = () =>
  useDataMutation(({ id, input }: { id: number; input: PersonInput }) => api.put<Person>(`/api/v1/persons/${id}`, input));
export function useDeletePerson() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.del(`/api/v1/persons/${id}`),
    onSuccess: (_, id) => {
      // Drop the deleted person's queries first so they aren't refetched into a 404.
      for (const key of ["person", "family", "notes", "tree"]) qc.removeQueries({ queryKey: [key, id] });
      qc.invalidateQueries({ predicate: (q) => !["status", "me", "backups"].includes(String(q.queryKey[0])) });
    },
  });
}

export function toPersonInput(p: Person): PersonInput {
  return {
    givenName: p.givenName, surname: p.surname, nickname: p.nickname, sex: p.sex, birthDate: p.birthDate,
    birthPlace: p.birthPlace, deathDate: p.deathDate, deathPlace: p.deathPlace, deceased: p.deceased,
    about: p.about, portraitMediaId: p.portraitMediaId,
  };
}

// ---- relationships ---------------------------------------------------------------

export const useCreateRelationship = () =>
  useDataMutation((in_: RelationshipInput) => api.post<Relationship>("/api/v1/relationships", in_));
export const useUpdateRelationship = () =>
  useDataMutation(({ id, input }: { id: number; input: RelationshipInput }) => api.put<Relationship>(`/api/v1/relationships/${id}`, input));
export const useDeleteRelationship = () => useDataMutation((id: number) => api.del(`/api/v1/relationships/${id}`));

// ---- notes -----------------------------------------------------------------------

export const useNotes = (personId: number) =>
  useQuery({ queryKey: ["notes", personId], queryFn: () => api.get<Note[]>(`/api/v1/persons/${personId}/notes`) });
export const useCreateNote = () =>
  useDataMutation(({ personId, title, body }: { personId: number; title: string; body: string }) =>
    api.post<Note>(`/api/v1/persons/${personId}/notes`, { title, body }));
export const useUpdateNote = () =>
  useDataMutation(({ id, title, body }: { id: number; title: string; body: string }) => api.put<Note>(`/api/v1/notes/${id}`, { title, body }));
export const useDeleteNote = () => useDataMutation((id: number) => api.del(`/api/v1/notes/${id}`));

// ---- events & timeline -----------------------------------------------------------

export const useEventTypes = () =>
  useQuery({
    queryKey: ["event-types"],
    queryFn: () => api.get<{ eventTypes: string[]; vitalTypes: string[] }>("/api/v1/event-types"),
    staleTime: Infinity,
  });

export const useEvent = (id: number | undefined) =>
  useQuery({ queryKey: ["event", id], queryFn: () => api.get<HolocronEvent>(`/api/v1/events/${id}`), enabled: !!id });

export const useCreateEvent = () => useDataMutation((in_: EventInput) => api.post<HolocronEvent>("/api/v1/events", in_));
export const useUpdateEvent = () =>
  useDataMutation(({ id, input }: { id: number; input: EventInput }) => api.put<HolocronEvent>(`/api/v1/events/${id}`, input));
export const useDeleteEvent = () => useDataMutation((id: number) => api.del(`/api/v1/events/${id}`));

export interface TimelineFilter {
  personId?: number;
  types?: string[];
  q?: string;
  from?: string;
  to?: string;
}

export const useTimeline = (f: TimelineFilter) =>
  useQuery({
    queryKey: ["timeline", f],
    queryFn: () =>
      api.get<TimelineResponse>(`/api/v1/timeline${qs({ personId: f.personId, types: f.types?.join(","), q: f.q, from: f.from, to: f.to })}`),
    placeholderData: keepPreviousData,
  });

// ---- tree ------------------------------------------------------------------------

export const useTree = (id: number | undefined, direction: TreeDirection, generations: number) =>
  useQuery({
    queryKey: ["tree", id, direction, generations],
    queryFn: () => api.get<TreeResponse>(`/api/v1/tree/${id}${qs({ direction, generations })}`),
    enabled: !!id,
    placeholderData: keepPreviousData,
  });

export const fetchTree = (id: number, direction: TreeDirection, generations: number) =>
  api.get<TreeResponse>(`/api/v1/tree/${id}${qs({ direction, generations })}`);

// ---- media -----------------------------------------------------------------------

export const useMediaList = (params: { personId?: number; q?: string; limit?: number; offset?: number }) =>
  useQuery({
    queryKey: ["media", params],
    queryFn: () => api.get<Page<Media>>(`/api/v1/media${qs(params)}`),
    placeholderData: keepPreviousData,
  });

export const useUploadMedia = () =>
  useDataMutation(({ file, personIds, title }: { file: File; personIds?: number[]; title?: string }) => {
    const fd = new FormData();
    fd.append("file", file);
    if (personIds?.length) fd.append("personIds", personIds.join(","));
    if (title) fd.append("title", title);
    return api.upload<Media>("/api/v1/media", fd);
  });
export const useUpdateMedia = () =>
  useDataMutation(({ id, input }: { id: number; input: MediaInput }) => api.put<Media>(`/api/v1/media/${id}`, input));
export const useDeleteMedia = () => useDataMutation((id: number) => api.del(`/api/v1/media/${id}`));

// ---- audit, stats, users ---------------------------------------------------------

export const useAudit = (params: { personId?: number; entityType?: string; userId?: number; before?: number; limit?: number }) =>
  useQuery({ queryKey: ["audit", params], queryFn: () => api.get<AuditPage>(`/api/v1/audit${qs(params)}`), placeholderData: keepPreviousData });

export const useStats = () => useQuery({ queryKey: ["stats"], queryFn: () => api.get<Stats>("/api/v1/stats") });

export const useUsers = () => useQuery({ queryKey: ["users"], queryFn: () => api.get<User[]>("/api/v1/users") });

export function useUserMutations() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: ["users"] });
    qc.invalidateQueries({ queryKey: ["audit"] });
  };
  return {
    create: useMutation({
      mutationFn: (in_: { username: string; password: string; displayName: string; email: string; role: string; mustChangePassword: boolean }) =>
        api.post<User>("/api/v1/users", in_),
      onSuccess: done,
    }),
    update: useMutation({
      mutationFn: ({ id, ...patch }: { id: number; role?: string; status?: string; displayName?: string; email?: string; password?: string; mustChangePassword?: boolean }) =>
        api.patch<User>(`/api/v1/users/${id}`, patch),
      onSuccess: done,
    }),
  };
}

// ---- personal access tokens -------------------------------------------------------

export const useAccessTokens = () =>
  useQuery({ queryKey: ["access-tokens"], queryFn: () => api.get<AccessToken[]>("/api/v1/user/access-tokens") });

export function useAccessTokenMutations() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: ["access-tokens"] });
    qc.invalidateQueries({ queryKey: ["audit"] });
  };
  return {
    create: useMutation({
      mutationFn: (in_: { description: string; expiresInDays: number | null }) => api.post<CreatedAccessToken>("/api/v1/user/access-tokens", in_),
      onSuccess: done,
    }),
    revoke: useMutation({ mutationFn: (id: number) => api.del(`/api/v1/user/access-tokens/${id}`), onSuccess: done }),
  };
}

// ---- settings, plugins, backups ----------------------------------------------------

export const useSettings = () =>
  useQuery({ queryKey: ["settings"], queryFn: () => api.get<{ instanceName: string }>("/api/v1/settings") });

export const usePlugins = () =>
  useQuery({ queryKey: ["plugins"], queryFn: () => api.get<PluginInfo[]>("/api/v1/plugins"), staleTime: 60_000 });

export function useSetPlugin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => api.put<PluginInfo[]>(`/api/v1/plugins/${id}`, { enabled }),
    // Optimistic: the switch (and the plugin's nav items/tabs) respond immediately.
    onMutate: async ({ id, enabled }) => {
      await qc.cancelQueries({ queryKey: ["plugins"] });
      const previous = qc.getQueryData<PluginInfo[]>(["plugins"]);
      qc.setQueryData<PluginInfo[]>(["plugins"], (list) => list?.map((p) => (p.id === id ? { ...p, enabled } : p)));
      return { previous };
    },
    onError: (_err, _vars, ctx) => ctx?.previous && qc.setQueryData(["plugins"], ctx.previous),
    onSuccess: (data) => qc.setQueryData(["plugins"], data),
  });
}

export const useBackups = () => useQuery({ queryKey: ["backups"], queryFn: () => api.get<BackupInfo[]>("/api/v1/backups") });

export function useBackupMutations() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["backups"] });
  return {
    create: useMutation({ mutationFn: () => api.post<BackupInfo>("/api/v1/backups"), onSuccess: refresh }),
    remove: useMutation({ mutationFn: (name: string) => api.del(`/api/v1/backups/${encodeURIComponent(name)}`), onSuccess: refresh }),
    upload: useMutation({
      mutationFn: (file: File) => {
        const fd = new FormData();
        fd.append("file", file);
        return api.upload<BackupInfo>("/api/v1/backups/upload", fd);
      },
      onSuccess: refresh,
    }),
    restore: useMutation({
      mutationFn: (name: string) =>
        api.post<{ result: RestoreResult; durationMs: number }>(`/api/v1/backups/${encodeURIComponent(name)}/restore`, { confirm: "RESTORE" }),
      onSuccess: () => qc.invalidateQueries(),
    }),
  };
}

export const useImportGedcom = () =>
  useDataMutation((file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    return api.upload<ImportResult>("/api/v1/gedcom/import", fd);
  });

// ---- biography templates -------------------------------------------------------

export const useBioTemplates = () =>
  useQuery({ queryKey: ["bio-templates"], queryFn: () => api.get<BioTemplate[]>("/api/v1/bio-templates"), staleTime: 60_000 });

export function useBioTemplateMutations() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: ["bio-templates"] });
    qc.invalidateQueries({ queryKey: ["audit"] });
  };
  return {
    create: useMutation({ mutationFn: (in_: BioTemplateInput) => api.post<BioTemplate>("/api/v1/bio-templates", in_), onSuccess: done }),
    update: useMutation({
      mutationFn: ({ id, input }: { id: number; input: BioTemplateInput }) => api.put<BioTemplate>(`/api/v1/bio-templates/${id}`, input),
      onSuccess: done,
    }),
    remove: useMutation({ mutationFn: (id: number) => api.del(`/api/v1/bio-templates/${id}`), onSuccess: done }),
  };
}
