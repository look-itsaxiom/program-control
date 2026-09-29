// This file holds the acceptance test for phase 6's UI piece (S6.3, live board), a stretch
// phase. It exercises only the contract in phases/6-permissions-and-realtime/STORIES.md: the app
// opens an EventSource for the current program and updates the affected row when a `task.updated`
// event arrives, without issuing a new request for the board's data.
//
// The test supplies its own minimal EventSource stand-in (assigned to globalThis.EventSource)
// rather than running a real HTTP stream, and drives it directly: this is what the phase 6 brief
// asks for, and it keeps the test decoupled from whatever transport (fetch polyfill, native
// EventSource) the real browser build would otherwise need.
import { render, screen, within, waitFor } from '@testing-library/react';
import { beforeEach, expect, it } from 'vitest';
import App from '../App';
import { phaseGate } from './helpers';
import { installFakeApi, SEED, type FakeApiControls } from './fake-api';

const describe = phaseGate(6);

let api: FakeApiControls;

function goToProgram(programId: string, userId: string) {
  localStorage.setItem('programControl.userId', userId);
  window.history.pushState({}, '', `/programs/${programId}`);
}

function rowFor(title: string): HTMLElement {
  const row = screen.getAllByRole('row').find((r) => within(r).queryByText(title));
  if (!row) throw new Error(`no row found for "${title}"`);
  return row;
}

// readStatus reads the current value of a row's status control: a <select>'s .value, or, for a
// button group, whichever button marks itself as the current selection via aria-pressed or
// aria-current. Mirrors phase3.test.tsx's helper of the same name (each phase's test file is
// self-contained).
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

// FakeEventSource is a minimal, fully test-controlled stand-in for the browser's EventSource. The
// contract only requires that the app open one against the program's stream URL and react to a
// `task.updated` event; this fake lets the test fire that event directly instead of running a
// real SSE connection.
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  url: string;
  private listeners = new Map<string, Set<(ev: MessageEvent) => void>>();
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  readyState = 1; // OPEN

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, cb: (ev: MessageEvent) => void) {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(cb);
  }

  removeEventListener(type: string, cb: (ev: MessageEvent) => void) {
    this.listeners.get(type)?.delete(cb);
  }

  close() {
    this.readyState = 2; // CLOSED
  }

  // emit drives a named SSE event the way a real EventSource would deliver one: to every listener
  // registered for that event type, and to a plain `type === 'message'` handler assigned via
  // onmessage (only relevant if the app never names the event, which the contract does not allow,
  // but keeping this here makes the fake a faithful enough stand-in either way).
  emit(type: string, data: unknown) {
    const event = { type, data: JSON.stringify(data) } as MessageEvent;
    this.listeners.get(type)?.forEach((cb) => cb(event));
    if (type === 'message') this.onmessage?.(event);
  }
}

beforeEach(() => {
  localStorage.clear();
  api = installFakeApi();
  FakeEventSource.instances = [];
  (globalThis as unknown as { EventSource: typeof FakeEventSource }).EventSource = FakeEventSource;
});

describe('phase 6 realtime', () => {
  it('S6.3 updates a row when the stream emits task.updated, without issuing a new request', async () => {
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await screen.findByText(SEED.titles.design);

    const source = FakeEventSource.instances.at(-1);
    if (!source) {
      throw new Error(
        'App did not construct an EventSource for the program stream (expected `new EventSource(...)` once the board is showing)',
      );
    }
    expect(source.url).toContain(SEED.programId);

    const requestCountBeforeEvent = api.requests.length;
    const initialStatus = readStatus(screen.getByLabelText(SEED.titles.design));
    const newStatus = initialStatus === 'doing' ? 'blocked' : 'doing';

    source.emit('task.updated', {
      id: SEED.tasks.design,
      programId: SEED.programId,
      orgId: SEED.orgs.customer,
      title: SEED.titles.design,
      status: newStatus,
      estimateMinutes: null,
      assigneeId: null,
      dueOn: null,
      internalNote: null,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });

    await waitFor(() => {
      expect(readStatus(screen.getByLabelText(SEED.titles.design))).toBe(newStatus);
    });

    const row = rowFor(SEED.titles.design);
    expect(within(row)).toBeTruthy(); // the row still exists and is addressable after the update

    // The whole point of a push-based update is that the client did not have to ask again.
    expect(api.requests.length).toBe(requestCountBeforeEvent);
  });

  it('S6.3 ignores a task.updated event for a task the current program is not showing', async () => {
    goToProgram(SEED.programId, SEED.users.alice);
    render(<App />);
    await screen.findByText(SEED.titles.design);

    const source = FakeEventSource.instances.at(-1);
    if (!source) throw new Error('App did not construct an EventSource for the program stream');

    const beforeStatus = readStatus(screen.getByLabelText(SEED.titles.design));

    source.emit('task.updated', {
      id: 'task-not-in-this-program',
      programId: 'some-other-program',
      orgId: SEED.orgs.customer,
      title: 'Unrelated task from elsewhere',
      status: 'done',
      estimateMinutes: null,
      assigneeId: null,
      dueOn: null,
      internalNote: null,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });

    // Give any (incorrect) re-render a moment to happen, then confirm nothing about the visible
    // board changed because of an event naming a task it never showed.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(readStatus(screen.getByLabelText(SEED.titles.design))).toBe(beforeStatus);
    expect(screen.queryByText('Unrelated task from elsewhere')).not.toBeInTheDocument();
  });
});
