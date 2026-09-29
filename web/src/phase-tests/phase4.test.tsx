import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it } from 'vitest';
import App from '../App';
import { phaseGate } from './helpers';
import {
  installFakeGraphql,
  uninstallFakeGraphql,
  resetGraphqlSeed,
  operations,
  delegatedCalls,
  programQueryCount,
  SEED,
} from './fake-graphql';

// This file holds the acceptance tests for phase 4 (GraphQL) on the client: see
// phases/4-graphql/STORIES.md, S4.5 and S4.6. It renders the real App against fake-graphql.ts
// instead of a live server, with localStorage['programControl.transport'] set to 'graphql' (the
// phase 4 contract's switch; the default stays REST so phase 3 tests keep passing untouched).

const describe = phaseGate(4);

function goToSeededProgramAsGraphql(userId: string) {
  window.localStorage.setItem('programControl.userId', userId);
  window.localStorage.setItem('programControl.transport', 'graphql');
  window.history.pushState({}, '', `/programs/${SEED.programId}`);
}

/** Selects `status` on the row's status control, whichever labelled option carries that text. */
async function setStatus(
  user: ReturnType<typeof userEvent.setup>,
  row: HTMLElement,
  title: string,
  status: string,
) {
  const control = within(row).getByLabelText(title);
  const option = within(control).getByRole('option', { name: new RegExp(status, 'i') });
  await user.selectOptions(control, option);
}

describe('phase 4 apollo', () => {
  beforeEach(() => {
    resetGraphqlSeed();
    installFakeGraphql();
    goToSeededProgramAsGraphql(SEED.users.alice);
  });

  afterEach(() => {
    uninstallFakeGraphql();
    window.localStorage.clear();
  });

  it('S4.5: renders the board through GraphQL, not REST, when the transport is graphql', async () => {
    render(<App />);

    expect(await screen.findByText(SEED.titles.design)).toBeInTheDocument();
    expect(operations.some((op) => /\bprogram\s*\(/.test(op.query))).toBe(true);
    // fake-graphql only answers /graphql; anything that fell through to a REST-style call would
    // show up here instead of as a recorded GraphQL operation.
    expect(delegatedCalls).toHaveLength(0);
  });

  it('S4.3: toggling status sends the mutation and updates the row without an extra program query', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.auth);

    const before = programQueryCount();
    expect(before).toBeGreaterThan(0);

    const row = screen.getByText(SEED.titles.auth).closest('tr')!;
    await setStatus(user, row, SEED.titles.auth, 'DOING');

    expect(operations.some((op) => /\bsetTaskStatus\s*\(/.test(op.query))).toBe(true);
    expect(programQueryCount()).toBe(before);
    expect(await within(row).findByText(/doing/i)).toBeInTheDocument();
  });

  it('S4.5: cache normalization keeps a downstream task\'s Ready badge in sync without a refetch', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.integration);

    // Integration depends on both auth and taskService; neither is done yet, so it starts not
    // ready. Finishing both, one mutation at a time, should flip its Ready badge on live, purely
    // from the cache each mutation's response updates, with no separate query telling the UI
    // "integration is now ready."
    const integrationRow = screen.getByText(SEED.titles.integration).closest('tr')!;
    expect(within(integrationRow).queryByText(/ready/i)).not.toBeInTheDocument();

    const authRow = screen.getByText(SEED.titles.auth).closest('tr')!;
    await setStatus(user, authRow, SEED.titles.auth, 'DONE');
    expect(within(integrationRow).queryByText(/ready/i)).not.toBeInTheDocument();

    const taskServiceRow = screen.getByText(SEED.titles.taskService).closest('tr')!;
    await setStatus(user, taskServiceRow, SEED.titles.taskService, 'DONE');

    expect(await within(integrationRow).findByText(/ready/i)).toBeInTheDocument();
  });

  it('S4.6: a GraphQL error surfaces in the live region', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.design);

    // design is already a transitive predecessor of docs (design -> auth -> integration ->
    // docs); adding docs as a predecessor of design would close that loop.
    const row = screen.getByText(SEED.titles.design).closest('tr')!;
    const dependencyEditor = within(row).getByLabelText(`Add dependency to ${SEED.titles.design}`);
    const option = within(dependencyEditor).getByRole('option', { name: SEED.titles.docs });
    await user.selectOptions(dependencyEditor, option);

    const liveRegion = document.querySelector('[aria-live="polite"]');
    expect(liveRegion).not.toBeNull();
    expect(await within(liveRegion as HTMLElement).findByText(/cycle/i)).toBeInTheDocument();
  });
});
