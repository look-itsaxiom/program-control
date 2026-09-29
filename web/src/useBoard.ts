import { useCallback, useEffect, useState } from 'react';

import {
  ApiError,
  addDependency,
  listReadyTasks,
  listTasks,
  setTaskStatus,
  STATUS_LABELS,
  type Task,
  type TaskStatus,
} from './api';

export interface Board {
  /** null while the first load is still in flight, which is what the loading state keys off. */
  tasks: Task[] | null;
  readyIds: Set<string>;
  loadError: string | null;
  /** The message read out when a status change is rolled back. Empty when there is nothing to say. */
  announcement: string;
  /** Per-task message from a refused dependency, shown inline beside that row's control. */
  dependencyErrors: Record<string, string>;
  changeStatus: (task: Task, status: TaskStatus) => Promise<void>;
  addTaskDependency: (task: Task, predecessorId: string) => Promise<void>;
}

/** ignoreAbort swallows the rejection a fetch produces when its request is cancelled. */
function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}

/**
 * useBoard owns everything the board knows: the program's tasks, which of them are ready, and the
 * two writes the board can make. Status changes are applied optimistically and rolled back when
 * the save fails, so the board feels instant without ever showing something that is not saved.
 */
export function useBoard(programId: string): Board {
  const [tasks, setTasks] = useState<Task[] | null>(null);
  const [readyIds, setReadyIds] = useState<Set<string>>(() => new Set());
  const [loadError, setLoadError] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState('');
  const [dependencyErrors, setDependencyErrors] = useState<Record<string, string>>({});

  const loadReady = useCallback(
    async (signal?: AbortSignal) => {
      const ready = await listReadyTasks(programId, signal);
      setReadyIds(new Set(ready.tasks.map((task) => task.id)));
    },
    [programId],
  );

  const load = useCallback(
    async (signal?: AbortSignal) => {
      const [all] = await Promise.all([listTasks(programId, signal), loadReady(signal)]);
      setTasks(all.tasks);
    },
    [programId, loadReady],
  );

  useEffect(() => {
    if (programId === '') {
      setTasks([]);
      return;
    }
    const controller = new AbortController();
    setTasks(null);
    setLoadError(null);
    load(controller.signal).catch((error: unknown) => {
      if (isAbort(error)) return;
      setTasks([]);
      setLoadError(error instanceof Error ? error.message : 'Could not load this program.');
    });
    return () => controller.abort();
  }, [load, programId]);

  const patchStatus = useCallback((taskId: string, status: TaskStatus) => {
    setTasks((current) =>
      current === null ? current : current.map((t) => (t.id === taskId ? { ...t, status } : t)),
    );
  }, []);

  const changeStatus = useCallback(
    async (task: Task, status: TaskStatus) => {
      const previous = task.status;
      patchStatus(task.id, status);
      setAnnouncement('');
      try {
        await setTaskStatus(task.id, status);
        await loadReady();
      } catch {
        patchStatus(task.id, previous);
        setAnnouncement(
          `Could not save "${task.title}" as ${STATUS_LABELS[status]}. It is back to ${STATUS_LABELS[previous]}.`,
        );
      }
    },
    [patchStatus, loadReady],
  );

  const addTaskDependency = useCallback(
    async (task: Task, predecessorId: string) => {
      setDependencyErrors((current) => ({ ...current, [task.id]: '' }));
      try {
        await addDependency(task.id, predecessorId);
        await load();
      } catch (error) {
        const predecessor = tasks?.find((t) => t.id === predecessorId);
        const name = predecessor ? `"${predecessor.title}"` : 'that task';
        const message =
          error instanceof ApiError && error.code === 'cycle'
            ? `${name} already has to wait for this one, so this would be a cycle. Nothing was changed.`
            : `Could not add ${name} as a dependency.`;
        setDependencyErrors((current) => ({ ...current, [task.id]: message }));
      }
    },
    [load, tasks],
  );

  return {
    tasks,
    readyIds,
    loadError,
    announcement,
    dependencyErrors,
    changeStatus,
    addTaskDependency,
  };
}
