import { useCallback, useEffect, useState } from 'react';

import {
  currentUserId,
  ApiError,
  addDependency,
  listReadyTasks,
  listTasks,
  setTaskStatus,
  STATUS_LABELS,
  type Task,
  type TaskStatus,
} from './api';
import type { Board } from './board';


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

  // The live feed. A pushed update is merged into the row it names and ignored when it names a
  // task this board is not showing; either way the board never asks the server for data it has
  // just been handed. The payload may be the reduced vendor shape, so it is merged over the row
  // rather than replacing it.
  useEffect(() => {
    if (programId === '' || typeof globalThis.EventSource === 'undefined') return;

    const source = new EventSource(
      `/api/programs/${encodeURIComponent(programId)}/stream?userId=${encodeURIComponent(currentUserId())}`,
    );

    const onTaskUpdated = (event: MessageEvent) => {
      let update: Partial<Task>;
      try {
        update = JSON.parse(String(event.data)) as Partial<Task>;
      } catch {
        return;
      }
      if (update.id === undefined) return;
      setTasks((current) =>
        current === null || !current.some((t) => t.id === update.id)
          ? current
          : current.map((t) => (t.id === update.id ? { ...t, ...update } : t)),
      );
    };

    source.addEventListener('task.updated', onTaskUpdated);
    return () => {
      source.removeEventListener('task.updated', onTaskUpdated);
      source.close();
    };
  }, [programId]);

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
        setAnnouncement(message);
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
