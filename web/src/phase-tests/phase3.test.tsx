import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it } from 'vitest';
import App from '../App';
import { phaseGate } from './helpers';
import { installFakeApi, SEED, type FakeApiControls } from './fake-api';

const describe = phaseGate(3);

let api: FakeApiControls;

function goToProgram(programId: string, userId: string) {
  localStorage.setItem('programControl.userId', userId);
  window.history.pushState({}, '', `/programs/${programId}`);
}

// rowsInOrder / titleOfRow read the board through ARIA table roles only
// (role="row", accessible text), so they work whether the app renders a
// real <table> or a div-based ARIA table, and regardless of column order.
function rowsInOrder(): HTMLElement[] {
  return screen.getAllByRole('row').slice(1); // drop the header row
}

function titleOfRow(row: HTMLElement): string {
  const found = Object.values(SEED.titles).find((title) => within(row).queryByText(title));
  if (!found) throw new Error('row does not contain a known task title');
  return found;
}

function rowFor(title: string): HTMLElement {
  const row = screen.getAllByRole('row').find((r) => within(r).queryByText(title));
  if (!row) throw new Error(`no row found for "${title}"`);
  return row;
}

// setStatus drives the per-row status control, which the Contract allows to
// be a <select> or a group of buttons labelled with the task title.
async function setStatus(user: ReturnType<typeof userEvent.setup>, title: string, status: string) {
  const control = screen.getByLabelText(title);
  if (control.tagName === 'SELECT') {
    try {
      await user.selectOptions(control, [status]);
    } catch {
      const capitalized = status.charAt(0).toUpperCase() + status.slice(1);
      await user.selectOptions(control, [capitalized]);
    }
    return;
  }
  const button = within(control).getByRole('button', { name: new RegExp(`^${status}$`, 'i') });
  await user.click(button);
}

// readStatus reads the current value of a status control back out: a
// <select>'s .value, or, for a button group, whichever button marks itself
// as the current selection via aria-pressed or aria-current.
function readStatus(control: HTMLElement): string {
  if (control.tagName === 'SELECT') {
    return (control as HTMLSelectElement).value.toLowerCase();
  }
  const active = control.querySelector('[aria-pressed="true"], [aria-current="true"], [aria-current="page"]');
  if (active) return (active.textContent ?? '').trim().toLowerCase();
  throw new Error(
    'could not read the status control: expected a <select> or a button group exposing the active status via aria-pressed/aria-current',
  );
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
      expect(within(table).getByText(title)).toBeInTheDocument();
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

    expect(screen.queryByText(SEED.titles.design)).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(SEED.titles.design)).toBeInTheDocument());
  });

  it('S3.1 shows an empty state for a program with no tasks', async () => {
    goToProgram(SEED.emptyProgramId, SEED.users.alice);
    render(<App />);

    const table = await screen.findByRole('table');
    await waitFor(() => {
      expect(screen.queryByText(SEED.titles.design)).not.toBeInTheDocument();
    });
    expect(within(table).getAllByRole('row')).toHaveLength(1); // header row only
  });

  it('S3.2 narrows the board with the filter input', async () => {
    const user = userEvent.setup();
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await screen.findByText(SEED.titles.auth);

    await user.type(screen.getByLabelText('Filter tasks'), 'auth');

    await waitFor(() => {
      expect(screen.getByText(SEED.titles.auth)).toBeInTheDocument();
      expect(screen.queryByText(SEED.titles.sandbox)).not.toBeInTheDocument();
      expect(screen.queryByText(SEED.titles.docs)).not.toBeInTheDocument();
      expect(screen.queryByText(SEED.titles.design)).not.toBeInTheDocument();
    });
  });

  it('S3.2 sorts the board when a column header is clicked, reflected in aria-sort', async () => {
    const user = userEvent.setup();
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await screen.findByText(SEED.titles.design);

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
    await screen.findByText(SEED.titles.taskService);

    await setStatus(user, SEED.titles.taskService, 'doing');

    // Optimistic: visible immediately, well before the fake's delay elapses.
    expect(readStatus(screen.getByLabelText(SEED.titles.taskService))).toBe('doing');

    // Persists once the fake resolves.
    await waitFor(
      () => {
        expect(readStatus(screen.getByLabelText(SEED.titles.taskService))).toBe('doing');
      },
      { timeout: 1000 },
    );

    const patch = api.requests.find(
      (r) => r.method === 'PATCH' && r.path === `/tasks/${SEED.tasks.taskService}`,
    );
    expect(patch).toBeTruthy();
  });

  it('S3.3 rolls back a status change and announces the failure when the save fails', async () => {
    const user = userEvent.setup();
    api.latency(150);
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await screen.findByText(SEED.titles.taskService);

    const before = readStatus(screen.getByLabelText(SEED.titles.taskService));
    api.failNext('server_error', 500);

    await setStatus(user, SEED.titles.taskService, 'done');
    expect(readStatus(screen.getByLabelText(SEED.titles.taskService))).toBe('done');

    await waitFor(
      () => {
        expect(readStatus(screen.getByLabelText(SEED.titles.taskService))).toBe(before);
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
    await screen.findByText(SEED.titles.design);

    // Diamond: design(done) -> auth(todo), taskService(todo) -> integration(todo).
    // auth and taskService are ready (their only predecessor, design, is done).
    // integration is not ready (its predecessors aren't both done yet).
    // docs (doing) and sandbox (blocked, no predecessors) are not ready either.
    const readyTitles = [SEED.titles.auth, SEED.titles.taskService];
    const notReadyTitles = [
      SEED.titles.design,
      SEED.titles.integration,
      SEED.titles.docs,
      SEED.titles.sandbox,
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
    await screen.findByText(SEED.titles.design);

    // design has no predecessors today; auth already depends on design.
    // Making auth a predecessor of design would close a cycle.
    const select = screen.getByLabelText(`Add dependency to ${SEED.titles.design}`);
    try {
      await user.selectOptions(select, [SEED.titles.auth]);
    } catch {
      await user.selectOptions(select, [SEED.tasks.auth]);
    }

    await waitFor(() => {
      expect(within(rowFor(SEED.titles.design)).getByText(/cycle/i)).toBeInTheDocument();
    });

    expect(api.hasDependency(SEED.tasks.auth, SEED.tasks.design)).toBe(false);
  });

  it('S3.6 does not show internal note text to a caller outside the owning org', async () => {
    goToProgram(SEED.programId, SEED.users.bob);
    render(<App />);
    await screen.findByText(SEED.titles.design);

    expect(screen.queryByText(SEED.internalNoteText)).not.toBeInTheDocument();
  });
});
