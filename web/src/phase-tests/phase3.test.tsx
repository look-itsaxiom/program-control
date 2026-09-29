import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it } from 'vitest';
import App from '../App';
import { findTitle, phaseGate, rowFor, rowsInOrder, setStatus, statusOf, titleNodes } from './helpers';
import { installFakeApi, SEED, type FakeApiControls } from './fake-api';

const describe = phaseGate(3);

let api: FakeApiControls;

function goToProgram(programId: string, userId: string) {
  localStorage.setItem('programControl.userId', userId);
  window.history.pushState({}, '', `/programs/${programId}`);
}

// titleOfRow names the task a row is showing, used to assert the board's order
// without depending on which column the title happens to live in. It asks titleNodes rather than
// the row's raw text, because a row's dependency picker may offer every other task by title.
function titleOfRow(row: HTMLElement): string {
  const found = Object.values(SEED.titles).find((title) => titleNodes(title, row).length > 0);
  if (!found) throw new Error('row does not contain a known task title');
  return found;
}

beforeEach(() => {
  localStorage.clear();
  api = installFakeApi();
});

describe('phase 3 react ui', () => {
  it('S3.1 renders a row and the required column headers for the seeded program', async () => {
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);

    const table = await screen.findByRole('table');
    for (const title of Object.values(SEED.titles)) {
      expect(titleNodes(title, table)).toHaveLength(1);
    }
    expect(screen.getByRole('columnheader', { name: 'Title' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Assignee' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Due' })).toBeInTheDocument();
  });

  it('S3.1 shows a loading state before the tasks arrive', async () => {
    api.latency(200);
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);

    expect(titleNodes(SEED.titles.root)).toHaveLength(0);
    expect(await findTitle(SEED.titles.root)).toBeInTheDocument();
  });

  it('S3.1 shows an empty state for a program with no tasks', async () => {
    goToProgram(SEED.emptyProgramId, SEED.users.alice);
    render(<App />);

    const table = await screen.findByRole('table');
    await waitFor(() => {
      expect(titleNodes(SEED.titles.root)).toHaveLength(0);
    });
    expect(within(table).getAllByRole('row')).toHaveLength(1); // header row only
  });

  it('S3.2 narrows the board with the filter input', async () => {
    const user = userEvent.setup();
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.branchA);

    // "housing" appears in exactly one seeded title, so a correct filter leaves
    // that one row standing and drops the other five.
    await user.type(screen.getByLabelText('Filter tasks'), 'housing');

    await waitFor(() => {
      expect(titleNodes(SEED.titles.branchA)).toHaveLength(1);
      expect(titleNodes(SEED.titles.blocked)).toHaveLength(0);
      expect(titleNodes(SEED.titles.independent)).toHaveLength(0);
      expect(titleNodes(SEED.titles.root)).toHaveLength(0);
    });
  });

  it('S3.2 sorts the board when a column header is clicked, reflected in aria-sort', async () => {
    const user = userEvent.setup();
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.root);

    const titleHeader = screen.getByRole('columnheader', { name: 'Title' });
    const clickTarget = within(titleHeader).queryByRole('button') ?? titleHeader;

    await user.click(clickTarget);
    await waitFor(() => {
      expect(['ascending', 'descending']).toContain(titleHeader.getAttribute('aria-sort'));
    });
    const firstDirection = titleHeader.getAttribute('aria-sort');
    const firstOrder = rowsInOrder().map(titleOfRow);

    const ascending = [...Object.values(SEED.titles)].sort((a, b) => a.localeCompare(b));
    const descending = [...ascending].reverse();
    expect(firstOrder).toEqual(firstDirection === 'ascending' ? ascending : descending);

    await user.click(clickTarget);
    await waitFor(() => {
      expect(titleHeader.getAttribute('aria-sort')).not.toBe(firstDirection);
    });
  });

  it('S3.3 shows a status change immediately, before the save resolves, and it persists', async () => {
    const user = userEvent.setup();
    api.latency(300);
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.branchA);

    await setStatus(user, SEED.titles.branchA, 'doing');

    // Optimistic: visible immediately, well before the fake's delay elapses.
    expect(statusOf(SEED.titles.branchA)).toBe('doing');

    // Persists once the fake resolves.
    await waitFor(
      () => {
        expect(statusOf(SEED.titles.branchA)).toBe('doing');
      },
      { timeout: 1000 },
    );

    const patch = api.requests.find(
      (r) => r.method === 'PATCH' && r.path === `/tasks/${SEED.tasks.branchA}`,
    );
    expect(patch).toBeTruthy();
  });

  it('S3.3 rolls back a status change and announces the failure when the save fails', async () => {
    const user = userEvent.setup();
    api.latency(150);
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.branchA);

    const before = statusOf(SEED.titles.branchA);
    api.failNext('server_error', 500);

    await setStatus(user, SEED.titles.branchA, 'done');
    expect(statusOf(SEED.titles.branchA)).toBe('done');

    await waitFor(
      () => {
        expect(statusOf(SEED.titles.branchA)).toBe(before);
      },
      { timeout: 1000 },
    );

    const live = document.querySelector('[aria-live="polite"]');
    expect(live).toBeTruthy();
    await waitFor(() => {
      expect((live?.textContent ?? '').trim().length).toBeGreaterThan(0);
    });
  });

  it('S3.5 shows a Ready badge exactly on tasks that are todo with every predecessor done', async () => {
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.root);

    // The diamond: root -> branchA, root -> branchB, both -> join, every one of
    // them todo. root and independent are ready, each being todo with nothing
    // to wait on. branchA and branchB are not, because root is still todo, and
    // join is not, because neither branch is done. blocked has no predecessors
    // either, but a person has already called it unworkable, so it is never
    // ready no matter what its dependencies look like.
    const readyTitles = [SEED.titles.root, SEED.titles.independent];
    const notReadyTitles = [
      SEED.titles.branchA,
      SEED.titles.branchB,
      SEED.titles.join,
      SEED.titles.blocked,
    ];

    for (const title of readyTitles) {
      expect(within(rowFor(title)).getByText(/ready/i)).toBeInTheDocument();
    }
    for (const title of notReadyTitles) {
      expect(within(rowFor(title)).queryByText(/ready/i)).not.toBeInTheDocument();
    }
  });

  it('S3.4 rejects a dependency that would create a cycle, with an inline error, and does not add it', async () => {
    const user = userEvent.setup();
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await findTitle(SEED.titles.root);

    // root has no predecessors today; branchA already depends on root.
    // Making branchA a predecessor of root would close a cycle.
    const select = screen.getByLabelText(`Add dependency to ${SEED.titles.root}`);
    try {
      await user.selectOptions(select, [SEED.titles.branchA]);
    } catch {
      await user.selectOptions(select, [SEED.tasks.branchA]);
    }

    await waitFor(() => {
      expect(within(rowFor(SEED.titles.root)).getByText(/cycle/i)).toBeInTheDocument();
    });

    expect(api.hasDependency(SEED.tasks.branchA, SEED.tasks.root)).toBe(false);
  });

  it('S3.6 does not show internal note text to a caller outside the owning org', async () => {
    goToProgram(SEED.programId, SEED.users.bob);
    render(<App />);
    await findTitle(SEED.titles.root);

    expect(screen.queryByText(SEED.internalNoteText)).not.toBeInTheDocument();
  });
});
