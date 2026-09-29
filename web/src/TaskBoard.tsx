import { useMemo, useState } from 'react';

import { STATUSES, STATUS_LABELS, type Task, type TaskStatus } from './api';
import type { Board } from './board';

type Column = 'title' | 'status' | 'assignee' | 'due';
type Direction = 'ascending' | 'descending';

const COLUMNS: { key: Column; label: string }[] = [
  { key: 'title', label: 'Title' },
  { key: 'status', label: 'Status' },
  { key: 'assignee', label: 'Assignee' },
  { key: 'due', label: 'Due' },
];

function sortKey(task: Task, column: Column): string {
  switch (column) {
    case 'title':
      return task.title;
    case 'status':
      return task.status;
    case 'assignee':
      return task.assigneeId ?? '';
    case 'due':
      return task.dueOn ?? '';
  }
}

/** compare sorts ascending, with missing values last so blanks never lead the board. */
function compare(a: string, b: string): number {
  if (a === b) return 0;
  if (a === '') return 1;
  if (b === '') return -1;
  return a.localeCompare(b);
}

export interface TaskBoardProps {
  board: Board;
}

export function TaskBoard({ board }: TaskBoardProps) {
  const { tasks, readyIds, loadError, announcement, dependencyErrors } = board;
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<{ column: Column; direction: Direction } | null>(null);

  const visible = useMemo(() => {
    const all = tasks ?? [];
    const needle = query.trim().toLowerCase();
    const filtered = needle === '' ? all : all.filter((t) => t.title.toLowerCase().includes(needle));
    if (sort === null) return filtered;
    const sign = sort.direction === 'ascending' ? 1 : -1;
    return [...filtered].sort(
      (a, b) => sign * compare(sortKey(a, sort.column), sortKey(b, sort.column)),
    );
  }, [tasks, query, sort]);

  function toggleSort(column: Column) {
    setSort((current) =>
      current?.column === column
        ? { column, direction: current.direction === 'ascending' ? 'descending' : 'ascending' }
        : { column, direction: 'ascending' },
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-6 py-8">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold text-slate-900">Program board</h1>
        <p className="mt-1 text-sm text-slate-600">
          Every task in this program, what it is waiting on, and what you can start now.
        </p>
      </header>

      {/* One polite live region for the whole board: a rolled-back status change is announced
          here rather than only shown as a pixel changing back. */}
      <p aria-live="polite" className="mb-4 min-h-6 text-sm font-medium text-rose-700">
        {announcement}
      </p>

      <div className="mb-4">
        <label htmlFor="task-filter" className="block text-sm font-medium text-slate-700">
          Filter tasks
        </label>
        <input
          id="task-filter"
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Type part of a title"
          className="mt-1 w-full max-w-sm rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200"
        />
      </div>

      {loadError !== null && (
        <p className="mb-4 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-800">{loadError}</p>
      )}

      {tasks === null ? (
        <p role="status" className="py-8 text-sm text-slate-600">
          Loading tasks…
        </p>
      ) : (
        <>
          <table role="table" className="w-full border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-slate-300">
                {COLUMNS.map(({ key, label }) => (
                  <th
                    key={key}
                    scope="col"
                    aria-sort={sort?.column === key ? sort.direction : 'none'}
                    className="px-3 py-2 font-semibold text-slate-700"
                  >
                    <button
                      type="button"
                      onClick={() => toggleSort(key)}
                      className="flex items-center gap-1 rounded focus:outline-none focus:ring-2 focus:ring-sky-300"
                    >
                      {label}
                      <span aria-hidden="true" className="text-xs text-slate-400">
                        {sort?.column === key ? (sort.direction === 'ascending' ? '▲' : '▼') : '↕'}
                      </span>
                    </button>
                  </th>
                ))}
                <th scope="col" className="px-3 py-2 font-semibold text-slate-700">
                  Dependencies
                </th>
              </tr>
            </thead>
            <tbody>
              {visible.map((task) => (
                <TaskRow
                  key={task.id}
                  task={task}
                  siblings={tasks}
                  isReady={readyIds.has(task.id)}
                  dependencyError={dependencyErrors[task.id] ?? ''}
                  onStatusChange={board.changeStatus}
                  onAddDependency={board.addTaskDependency}
                />
              ))}
            </tbody>
          </table>

          {tasks.length === 0 && (
            <p className="py-8 text-sm text-slate-600">
              This program has no tasks yet. Create one to get started.
            </p>
          )}
          {tasks.length > 0 && visible.length === 0 && (
            <p className="py-8 text-sm text-slate-600">No task title matches that filter.</p>
          )}
        </>
      )}
    </div>
  );
}

interface TaskRowProps {
  task: Task;
  siblings: Task[];
  isReady: boolean;
  dependencyError: string;
  onStatusChange: (task: Task, status: TaskStatus) => void;
  onAddDependency: (task: Task, predecessorId: string) => void;
}

function TaskRow({
  task,
  siblings,
  isReady,
  dependencyError,
  onStatusChange,
  onAddDependency,
}: TaskRowProps) {
  const note = task.internalNote;
  const candidates = siblings.filter((other) => other.id !== task.id);

  return (
    <tr className="border-b border-slate-200 align-top">
      <td className="px-3 py-2">
        <span className="font-medium text-slate-900">{task.title}</span>
        {isReady && (
          <span className="ml-2 rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-semibold text-emerald-800">
            Ready
          </span>
        )}
        {/* The API omits internalNote entirely for a caller outside this task's organization, so
            there is nothing here to hide: the field simply is not there. */}
        {typeof note === 'string' && note !== '' && (
          <span className="mt-1 block text-xs text-slate-500">{note}</span>
        )}
      </td>

      <td className="px-3 py-2">
        <select
          aria-label={task.title}
          value={task.status}
          onChange={(event) => onStatusChange(task, event.target.value as TaskStatus)}
          className="rounded-md border border-slate-300 px-2 py-1 text-sm focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200"
        >
          {STATUSES.map((status) => (
            <option key={status} value={status}>
              {STATUS_LABELS[status]}
            </option>
          ))}
        </select>
      </td>

      <td className="px-3 py-2 text-slate-700">{task.assigneeId ?? 'Unassigned'}</td>
      <td className="px-3 py-2 text-slate-700">{task.dueOn ?? 'No due date'}</td>

      <td className="px-3 py-2">
        <select
          aria-label={`Add dependency to ${task.title}`}
          value=""
          onChange={(event) => {
            const predecessorId = event.target.value;
            if (predecessorId !== '') onAddDependency(task, predecessorId);
          }}
          className="rounded-md border border-slate-300 px-2 py-1 text-sm focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200"
        >
          <option value="">Add a predecessor</option>
          {/* Each option is named for the task it would add, and nothing else: S4.6 reaches a
              predecessor by the option's accessible name, so "Assemble and test" has to mean
              that task here exactly as it does in its own row. The select's own label carries
              the "add a dependency to..." sense, so the option does not have to. */}
          {candidates.map((other) => (
            <option key={other.id} value={other.id}>
              {other.title}
            </option>
          ))}
        </select>
        {dependencyError !== '' && (
          <span className="mt-1 block text-xs text-rose-700">{dependencyError}</span>
        )}
      </td>
    </tr>
  );
}
