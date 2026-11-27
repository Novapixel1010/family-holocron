import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { api, qs } from "@/api/client";
import type { Attribution, UserRef } from "@/api/types";
import { withStatus } from "./model";

export const BASE = "/api/v1/plugins/task";

export type TaskStatus = "open" | "in_progress" | "blocked" | "done";
export type TaskPriority = "low" | "medium" | "high";

export interface Task extends Attribution {
  id: number;
  title: string;
  descriptionMd: string;
  status: TaskStatus;
  priority: TaskPriority;
  tags: string[];
  claimedBy: UserRef | null;
  claimedAt: string | null;
  people: { id: number; name: string; birthDate: string; deathDate: string }[];
  media: { id: number; title: string; originalName: string; mimeType: string }[];
  commentCount: number;
}

export interface TaskComment {
  id: number;
  taskId: number;
  author: UserRef | null;
  bodyMd: string;
  edited: boolean;
  createdAt: string;
  updatedAt: string;
}

/** Statuses a claimed task can be switched to from its card ("open" = release the claim instead). */
export const QUICK_STATUSES: TaskStatus[] = ["in_progress", "blocked", "done"];

export interface TaskInput {
  title: string;
  descriptionMd: string;
  status: TaskStatus;
  priority: TaskPriority;
  tags: string[];
  personIds: number[];
  mediaIds: number[];
}

export interface TaskFilter {
  status?: TaskStatus[];
  priority?: string;
  tag?: string;
  personId?: number;
  claimedBy?: "me";
  q?: string;
}

const KEY = "plugin-task";

export const useTasks = (f: TaskFilter) =>
  useQuery({
    queryKey: [KEY, "list", f],
    queryFn: () => api.get<Task[]>(`${BASE}/tasks${qs({ ...f, status: f.status?.join(",") })}`),
    placeholderData: keepPreviousData,
  });

export const useTask = (id: number | undefined) =>
  useQuery({ queryKey: [KEY, "task", id], queryFn: () => api.get<Task>(`${BASE}/tasks/${id}`), enabled: !!id });

export const useTaskTags = () =>
  useQuery({ queryKey: [KEY, "tags"], queryFn: () => api.get<{ tag: string; count: number }[]>(`${BASE}/tags`) });

function useTaskMutation<V, R>(fn: (v: V) => Promise<R>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [KEY] });
      qc.invalidateQueries({ queryKey: ["audit"] });
    },
  });
}

export const useCreateTask = () => useTaskMutation((in_: TaskInput) => api.post<Task>(`${BASE}/tasks`, in_));
export const useUpdateTask = () =>
  useTaskMutation(({ id, input }: { id: number; input: TaskInput }) => api.put<Task>(`${BASE}/tasks/${id}`, input));
export const useDeleteTask = () => useTaskMutation((id: number) => api.del(`${BASE}/tasks/${id}`));
export const useClaimTask = () =>
  useTaskMutation(({ id, takeOver }: { id: number; takeOver: boolean }) => api.post<Task>(`${BASE}/tasks/${id}/claim`, { takeOver }));
export const useUnclaimTask = () => useTaskMutation((id: number) => api.post<Task>(`${BASE}/tasks/${id}/unclaim`));

export function toInput(t: Task): TaskInput {
  return {
    title: t.title, descriptionMd: t.descriptionMd, status: t.status, priority: t.priority, tags: t.tags,
    personIds: t.people.map((p) => p.id), mediaIds: t.media.map((m) => m.id),
  };
}

/**
 * Quick status switch for a claimed task (PATCH …/status). Optimistic: every cached
 * copy of the task (lists and detail) shows the new status and "updated by me, now"
 * immediately, and is rolled back if the server refuses.
 */
export function useSetTaskStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: number; status: TaskStatus; me: UserRef }) =>
      api.patch<Task>(`${BASE}/tasks/${id}/status`, { status }),
    onMutate: async ({ id, status, me }) => {
      await qc.cancelQueries({ queryKey: [KEY] });
      // Only task lists and task details — comment threads are cached under KEY too.
      const snapshot = qc.getQueriesData<Task | Task[]>({ predicate: (q) => q.queryKey[0] === KEY && (q.queryKey[1] === "list" || q.queryKey[1] === "task") });
      const now = new Date().toISOString();
      for (const [key, data] of snapshot) {
        if (data) qc.setQueryData(key, withStatus(data, id, status, me, now));
      }
      return { snapshot };
    },
    onError: (_e, _v, ctx) => {
      for (const [key, data] of ctx?.snapshot ?? []) qc.setQueryData(key, data);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: [KEY] });
      qc.invalidateQueries({ queryKey: ["audit"] });
    },
  });
}

export const useTaskComments = (taskId: number) =>
  useQuery({ queryKey: [KEY, "comments", taskId], queryFn: () => api.get<TaskComment[]>(`${BASE}/tasks/${taskId}/comments`) });

export const useAddComment = () =>
  useTaskMutation(({ taskId, bodyMd }: { taskId: number; bodyMd: string }) => api.post<TaskComment>(`${BASE}/tasks/${taskId}/comments`, { bodyMd }));
export const useEditComment = () =>
  useTaskMutation(({ taskId, id, bodyMd }: { taskId: number; id: number; bodyMd: string }) =>
    api.patch<TaskComment>(`${BASE}/tasks/${taskId}/comments/${id}`, { bodyMd }));
export const useDeleteComment = () =>
  useTaskMutation(({ taskId, id }: { taskId: number; id: number }) => api.del(`${BASE}/tasks/${taskId}/comments/${id}`));
