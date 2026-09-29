// The browser's half of the phase 2 REST contract. Every call goes to a relative /api URL, which
// Vite proxies to the Go service in development and the phase tests replace with a fake.

export type TaskStatus = 'todo' | 'doing' | 'blocked' | 'done';

export const STATUSES: TaskStatus[] = ['todo', 'doing', 'blocked', 'done'];

export const STATUS_LABELS: Record<TaskStatus, string> = {
  todo: 'To do',
  doing: 'Doing',
  blocked: 'Blocked',
  done: 'Done',
};

export interface Task {
  id: string;
  programId: string;
  orgId: string;
  title: string;
  status: TaskStatus;
  estimateMinutes: number | null;
  assigneeId: string | null;
  dueOn: string | null;
  /** Absent, not null, when the caller is outside the task's owning organization. */
  internalNote?: string | null;
  createdAt: string;
  updatedAt: string;
}

/** ApiError carries the envelope's error code, which is what callers branch on. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/** The signed-in user, stood up by phase 2's X-User-Id header instead of a real session. */
export function currentUserId(): string {
  try {
    return localStorage.getItem('programControl.userId') ?? '';
  } catch {
    return '';
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-User-Id': currentUserId(),
      ...init.headers,
    },
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const body = (await response.json().catch(() => null)) as
    | { error?: { code?: string; message?: string } }
    | null;

  if (!response.ok) {
    const code = body?.error?.code ?? 'unknown';
    throw new ApiError(response.status, code, body?.error?.message ?? response.statusText);
  }
  return body as T;
}

export function listTasks(programId: string, signal?: AbortSignal): Promise<{ tasks: Task[] }> {
  return request(`/programs/${encodeURIComponent(programId)}/tasks`, { signal });
}

export function listReadyTasks(programId: string, signal?: AbortSignal): Promise<{ tasks: Task[] }> {
  return request(`/programs/${encodeURIComponent(programId)}/tasks/ready`, { signal });
}

export function setTaskStatus(taskId: string, status: TaskStatus): Promise<Task> {
  return request(`/tasks/${encodeURIComponent(taskId)}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

export function addDependency(taskId: string, predecessorId: string): Promise<unknown> {
  return request(`/tasks/${encodeURIComponent(taskId)}/dependencies`, {
    method: 'POST',
    body: JSON.stringify({ predecessorId }),
  });
}
