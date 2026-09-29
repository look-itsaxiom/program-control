import { useEffect, useState } from 'react';

import { TaskBoard } from './TaskBoard';
import { useBoard } from './useBoard';

/** programIdFromPath reads the program out of /programs/{programId}, or "" when there is none. */
function programIdFromPath(pathname: string): string {
  return /^\/programs\/([^/]+)/.exec(pathname)?.[1] ?? '';
}

/**
 * useProgramId tracks the program in the address bar. There is no router here on purpose: one
 * path shape, one screen, so a regular expression and the history event are the whole of it.
 */
function useProgramId(): string {
  const [programId, setProgramId] = useState(() => programIdFromPath(window.location.pathname));

  useEffect(() => {
    const onPopState = () => setProgramId(programIdFromPath(window.location.pathname));
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  return programId;
}

export default function App() {
  const programId = useProgramId();
  const board = useBoard(programId);

  if (programId === '') {
    return (
      <main data-testid="placeholder" className="mx-auto max-w-5xl px-6 py-8">
        <h1 className="text-2xl font-semibold text-slate-900">Program Control</h1>
        <p className="mt-2 text-slate-600">
          Open a program at <code className="rounded bg-slate-100 px-1">/programs/&lt;id&gt;</code>{' '}
          to see its board.
        </p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-slate-50 text-slate-900">
      <TaskBoard board={board} />
    </main>
  );
}
