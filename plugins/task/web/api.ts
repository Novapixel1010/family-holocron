import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { api, qs } from "@/api/client";
import type { Attribution, UserRef } from "@/api/types";

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
}

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
