import { screen, waitFor, within } from '@testing-library/react';
import type userEvent from '@testing-library/user-event';
import { describe } from 'vitest';

type User = ReturnType<typeof userEvent.setup>;

/**
 * Returns `describe.skip` when a `PHASE` env var is set and lower than `n`,
 * otherwise returns `describe`. Used to gate acceptance tests by phase so CI
 * can run only the phases Chase has reached (`PHASE=N` in ci.yml).
 */
export function phaseGate(n: number) {
  const phase = process.env.PHASE;
  if (phase !== undefined && Number(phase) < n) {
    return describe.skip;
  }
  return describe;
}

// The fake in-test server that implements the phase 2 REST contract for
// phase 3's acceptance tests lives in ./fake-api.ts (installFakeApi()).

// --- the board, read through ARIA roles only --------------------------------
//
// Every helper below reads the board through roles and accessible names, never
// through tag names or the DOM tree's shape, so a div-based ARIA table passes
// exactly as a real <table> does. Phases 3, 4 and 6 all share them: a status
// control the phase 3 contract allows to be a <select> should not have to be a
// <select> again in phase 4 just because that phase's test was written with
// one in mind.

/** Every body row of the board, in the order the board renders them. */
export function rowsInOrder(): HTMLElement[] {
  return screen.getAllByRole('row').slice(1); // drop the header row
}

// A task title is not unique in the document, and no phase test may assume it is. Phase 4's S4.6
// reaches a dependency by the accessible name of its `option`, which the phase 3 contract lets a
// board put in every row's dependency picker, so "Assemble and test" is on screen once as a row
// and five more times as an option. A plain getByText(title) therefore finds six elements and
// throws, and a row lookup that merely asks "does this row contain the text" picks whichever row
// offers the title first rather than the row that *is* the task.
//
// titleNodes reads only the places the board shows a task, skipping options, which is what every
// caller actually meant.

/** Elements showing `title` as board content, ignoring picker options carrying the same name. */
export function titleNodes(title: string, scope?: HTMLElement): HTMLElement[] {
  const area = scope ? within(scope) : screen;
  const options = new Set<Element>(area.queryAllByRole('option', { hidden: true }));
  return area.queryAllByText(title).filter((el) => !options.has(el));
}

/** Waits for the board to show `title` and returns the element showing it. */
export function findTitle(title: string, scope?: HTMLElement): Promise<HTMLElement> {
  return waitFor(() => {
    const [first] = titleNodes(title, scope);
    if (!first) throw new Error(`the board is not showing "${title}" yet`);
    return first;
  });
}

/** The row whose own title is `title`. Throws, naming the title, if there is none. */
export function rowFor(title: string): HTMLElement {
  const row = screen.getAllByRole('row').find((r) => titleNodes(title, r).length > 0);
  if (!row) throw new Error(`no row found for "${title}"`);
  return row;
}

/**
 * Sets `status` on the status control labelled with `title`.
 *
 * The contract allows that control to be either a `<select>` of statuses or a
 * group of buttons named for each status, so this handles both: it selects the
 * option (trying the capitalized label too, since the contract does not fix the
 * option text's case), or clicks the button whose accessible name is the status.
 */
export async function setStatus(user: User, title: string, status: string): Promise<void> {
  const control = screen.getByLabelText(title);
  if (control.tagName === 'SELECT') {
    try {
      await user.selectOptions(control, [status]);
    } catch {
      const capitalized = status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();
      await user.selectOptions(control, [capitalized]);
    }
    return;
  }
  const button = within(control).getByRole('button', { name: new RegExp(`^${status}$`, 'i') });
  await user.click(button);
}

/**
 * Reads the current value of a status control back out, lowercased: a
 * `<select>`'s value, or, for a button group, whichever button marks itself as
 * the current selection through `aria-pressed` or `aria-current`.
 */
export function readStatus(control: HTMLElement): string {
  if (control.tagName === 'SELECT') {
    return (control as HTMLSelectElement).value.toLowerCase();
  }
  const active = control.querySelector(
    '[aria-pressed="true"], [aria-current="true"], [aria-current="page"]',
  );
  if (active) return (active.textContent ?? '').trim().toLowerCase();
  throw new Error(
    'could not read the status control: expected a <select> or a button group exposing the active status via aria-pressed/aria-current',
  );
}

/** Reads the status shown on the row for `title`. */
export function statusOf(title: string): string {
  return readStatus(screen.getByLabelText(title));
}
