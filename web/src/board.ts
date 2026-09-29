import type { Task, TaskStatus } from './api';

/**
 * Board is what the table renders, whichever transport produced it. Keeping this one shape lets
 * the REST hook and the GraphQL hook stay interchangeable, and lets the table stay ignorant of
 * which one is behind it.
 */
export interface Board {
  /** null while the first load is still in flight, which is what the loading state keys off. */
  tasks: Task[] | null;
  readyIds: Set<string>;
  loadError: string | null;
  /** The message announced when a write fails. Empty when there is nothing to say. */
  announcement: string;
  /** Per-task message from a refused dependency, shown inline beside that row's control. */
  dependencyErrors: Record<string, string>;
  changeStatus: (task: Task, status: TaskStatus) => void;
  addTaskDependency: (task: Task, predecessorId: string) => void;
}
