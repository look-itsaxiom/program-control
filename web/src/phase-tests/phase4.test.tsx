import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it } from 'vitest';
import App from '../App';
import { phaseGate, rowFor, setStatus } from './helpers';
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

// Rows and status controls are reached through helpers.ts, shared with phases 3
// and 6: the board is read through ARIA roles only, so a div-based ARIA table
// passes, and the status control may be a <select> or a group of buttons, the
// same latitude the phase 3 contract gives it.

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

    expect(await screen.findByText(SEED.titles.root)).toBeInTheDocument();
    expect(operations.some((op) => /\bprogram\s*\(/.test(op.query))).toBe(true);
    // fake-graphql only answers /graphql; anything that fell through to a REST-style call would
    // show up here instead of as a recorded GraphQL operation.
    expect(delegatedCalls).toHaveLength(0);
  });

  it('S4.3: toggling status sends the mutation and updates the row without an extra program query', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.branchA);

    const before = programQueryCount();
    expect(before).toBeGreaterThan(0);

    await setStatus(user, SEED.titles.branchA, 'DOING');

    expect(operations.some((op) => /\bsetTaskStatus\s*\(/.test(op.query))).toBe(true);
    expect(programQueryCount()).toBe(before);
    expect(await within(rowFor(SEED.titles.branchA)).findByText(/doing/i)).toBeInTheDocument();
  });

  it('S4.5: cache normalization keeps a downstream task\'s Ready badge in sync without a refetch', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.join);

    // join depends on both branchA and branchB; neither is done yet, so it starts not ready.
    // Finishing both, one mutation at a time, should flip its Ready badge on live, purely from
    // the cache each mutation's response updates, with no separate query telling the UI "join is
    // now ready."
    expect(within(rowFor(SEED.titles.join)).queryByText(/ready/i)).not.toBeInTheDocument();

    await setStatus(user, SEED.titles.branchA, 'DONE');
    expect(within(rowFor(SEED.titles.join)).queryByText(/ready/i)).not.toBeInTheDocument();

    await setStatus(user, SEED.titles.branchB, 'DONE');

    expect(await within(rowFor(SEED.titles.join)).findByText(/ready/i)).toBeInTheDocument();
  });

  it('S4.6: a GraphQL error surfaces in the live region', async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText(SEED.titles.root);

    // root is already a transitive predecessor of join (root -> branchA -> join); adding join as
    // a predecessor of root would close that loop.
    const row = rowFor(SEED.titles.root);
    const dependencyEditor = within(row).getByLabelText(`Add dependency to ${SEED.titles.root}`);
    const option = within(dependencyEditor).getByRole('option', { name: SEED.titles.join });
    await user.selectOptions(dependencyEditor, option);

    const liveRegion = document.querySelector('[aria-live="polite"]');
    expect(liveRegion).not.toBeNull();
    expect(await within(liveRegion as HTMLElement).findByText(/cycle/i)).toBeInTheDocument();
  });
});
