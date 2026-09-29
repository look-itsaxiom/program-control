// fake-api.ts
//
// An in-memory implementation of the phase 2 REST contract (see
// phases/2-rest-api/STORIES.md), installed as globalThis.fetch so phase 3
// tests can render the real App against a fake server instead of a live
// Go process. Seeded to mirror the Go seedWorld: one customer org, two
// vendor orgs, four users, a program shared to both vendors, and a program
// of six tasks arranged in a diamond dependency shape plus a tail and an
// independent task, with one internal note and one task owned by a vendor
// (mixed owning orgs).

export type OrgRole = 'owner' | 'admin' | 'member' | 'viewer';
export type ShareRole = 'editor' | 'viewer';
export type TaskStatus = 'todo' | 'doing' | 'blocked' | 'done';

interface Task {
  id: string;
  programId: string;
  orgId: string;
  title: string;
  status: TaskStatus;
  estimateMinutes: number | null;
  assigneeId: string | null;
  dueOn: string | null;
  internalNote: string | null;
  createdAt: string;
  updatedAt: string;
}

interface Program {
  id: string;
  orgId: string;
  name: string;
}

interface Dependency {
  predecessorId: string;
  successorId: string;
}

export interface RequestLogEntry {
  method: string;
  path: string;
  userId: string | null;
  body: unknown;
}

export interface FakeApiControls {
  /** Delay every subsequent response by this many ms, until changed again. */
  latency(ms: number): void;
  /** Make the next request fail with this error code (and optional status). */
  failNext(code: string, status?: number): void;
  /** Every request seen so far, in order. */
  requests: RequestLogEntry[];
  /** Reset all state (data, latency, failNext, requests) back to the seed. */
  reset(): void;
  /** True if the given predecessor -> successor edge currently exists. */
  hasDependency(predecessorId: string, successorId: string): boolean;
}

// --- Seed identities --------------------------------------------------

export const SEED = {
  orgs: {
    customer: 'org-customer',
    vendorA: 'org-vendor-a',
    vendorB: 'org-vendor-b',
  },
  users: {
    alice: 'user-alice',
    bob: 'user-bob',
    carol: 'user-carol',
    dave: 'user-dave',
  },
  programId: 'program-1',
  emptyProgramId: 'program-empty',
  tasks: {
    design: 'task-1',
    auth: 'task-2',
    taskService: 'task-3',
    integration: 'task-4',
    docs: 'task-5',
    sandbox: 'task-6',
  },
  titles: {
    design: 'Design API contract',
    auth: 'Build auth service',
    taskService: 'Build task service',
    integration: 'Integration tests',
    docs: 'Write docs',
    sandbox: 'Vendor sandbox setup',
  },
  internalNoteText: 'Confirm scope with legal before external share',
} as const;

// --- Mutable state ------------------------------------------------------

let membership: Map<string, string>; // userId -> orgId
let programs: Map<string, Program>;
let shares: Map<string, Map<string, ShareRole>>; // programId -> orgId -> role
let tasks: Map<string, Task>;
let dependencies: Dependency[];
let requestSeq: number;

function seedNow() {
  membership = new Map([
    [SEED.users.alice, SEED.orgs.customer],
    [SEED.users.bob, SEED.orgs.vendorA],
    [SEED.users.carol, SEED.orgs.vendorB],
    // dave is a stranger: no membership row at all.
  ]);
  programs = new Map([
    [SEED.programId, { id: SEED.programId, orgId: SEED.orgs.customer, name: 'Q4 Platform Rollout' }],
    [SEED.emptyProgramId, { id: SEED.emptyProgramId, orgId: SEED.orgs.customer, name: 'Empty Pilot' }],
  ]);
  shares = new Map([
    [
      SEED.programId,
      new Map([
        [SEED.orgs.vendorA, 'editor'],
        [SEED.orgs.vendorB, 'viewer'],
      ]),
    ],
    [SEED.emptyProgramId, new Map()],
  ]);

  const now = new Date().toISOString();
  const t = (
    id: string,
    title: string,
    orgId: string,
    status: TaskStatus,
    opts: Partial<Pick<Task, 'assigneeId' | 'estimateMinutes' | 'dueOn' | 'internalNote'>> = {},
  ): Task => ({
    id,
    programId: SEED.programId,
    orgId,
    title,
    status,
    estimateMinutes: opts.estimateMinutes ?? null,
    assigneeId: opts.assigneeId ?? null,
    dueOn: opts.dueOn ?? null,
    internalNote: opts.internalNote ?? null,
    createdAt: now,
    updatedAt: now,
  });

  tasks = new Map(
    [
      t(SEED.tasks.design, SEED.titles.design, SEED.orgs.customer, 'done', {
        assigneeId: SEED.users.alice,
        estimateMinutes: 120,
        dueOn: '2026-10-01',
        internalNote: SEED.internalNoteText,
      }),
      t(SEED.tasks.auth, SEED.titles.auth, SEED.orgs.customer, 'todo', {
        assigneeId: SEED.users.alice,
        estimateMinutes: 300,
        dueOn: '2026-10-05',
      }),
      t(SEED.tasks.taskService, SEED.titles.taskService, SEED.orgs.customer, 'todo', {
        estimateMinutes: 240,
        dueOn: '2026-10-06',
      }),
      t(SEED.tasks.integration, SEED.titles.integration, SEED.orgs.customer, 'todo', {
        assigneeId: SEED.users.carol,
        estimateMinutes: 180,
        dueOn: '2026-10-10',
      }),
      t(SEED.tasks.docs, SEED.titles.docs, SEED.orgs.customer, 'doing', {
        assigneeId: SEED.users.bob,
        estimateMinutes: 90,
        dueOn: '2026-10-12',
      }),
      t(SEED.tasks.sandbox, SEED.titles.sandbox, SEED.orgs.vendorA, 'blocked', {
        assigneeId: SEED.users.bob,
        estimateMinutes: 60,
      }),
    ].map((task) => [task.id, task]),
  );

  // Diamond: design -> auth, design -> taskService, auth -> integration,
  // taskService -> integration. Tail: integration -> docs. sandbox is
  // independent (no predecessors, and no successors).
  dependencies = [
    { predecessorId: SEED.tasks.design, successorId: SEED.tasks.auth },
    { predecessorId: SEED.tasks.design, successorId: SEED.tasks.taskService },
    { predecessorId: SEED.tasks.auth, successorId: SEED.tasks.integration },
    { predecessorId: SEED.tasks.taskService, successorId: SEED.tasks.integration },
    { predecessorId: SEED.tasks.integration, successorId: SEED.tasks.docs },
  ];

  requestSeq = 0;
}

let latencyMs = 0;
let queuedFailure: { code: string; status: number } | null = null;
let requestLog: RequestLogEntry[] = [];

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// --- JSON helpers ---------------------------------------------------------

function errorBody(code: string, message: string) {
  return { error: { code, message } };
}

function taskJson(task: Task, callerOrgId: string | null) {
  const { internalNote, ...rest } = task;
  if (callerOrgId !== null && callerOrgId === task.orgId) {
    return { ...rest, internalNote };
  }
  return rest;
}

function predecessorsOf(taskId: string): string[] {
  return dependencies.filter((d) => d.successorId === taskId).map((d) => d.predecessorId);
}

function successorsOf(taskId: string): string[] {
  return dependencies.filter((d) => d.predecessorId === taskId).map((d) => d.successorId);
}

function isReady(task: Task): boolean {
  if (task.status !== 'todo') return false;
  return predecessorsOf(task.id).every((predId) => tasks.get(predId)?.status === 'done');
}

// reachableFrom returns every task id reachable by walking forward
// (predecessor -> successor) edges starting at startId, used to detect
// whether adding predecessor -> successor would close a cycle: it would if
// predecessor is already reachable forward from successor.
function reachableFrom(startId: string): Set<string> {
  const seen = new Set<string>();
  const stack = [startId];
  while (stack.length > 0) {
    const current = stack.pop()!;
    for (const next of successorsOf(current)) {
      if (!seen.has(next)) {
        seen.add(next);
        stack.push(next);
      }
    }
  }
  return seen;
}

function callerOrgFor(userId: string | null): string | null {
  if (userId === null) return null;
  return membership.get(userId) ?? null;
}

function canAccessProgram(callerOrgId: string | null, program: Program): boolean {
  if (callerOrgId === null) return false;
  if (callerOrgId === program.orgId) return true;
  return shares.get(program.id)?.has(callerOrgId) ?? false;
}

// --- Route handling ---------------------------------------------------

interface HandlerResult {
  status: number;
  body: unknown;
}

function handle(method: string, path: string, userId: string | null, body: unknown): HandlerResult {
  // dave is a known-but-orgless user: he authenticates but belongs nowhere.
  const isKnownUser = userId !== null && (membership.has(userId) || userId === SEED.users.dave);
  if (!isKnownUser) {
    return { status: 401, body: errorBody('unauthenticated', 'missing or unknown X-User-Id') };
  }
  const callerOrgId = callerOrgFor(userId);

  let m: RegExpMatchArray | null;

  // GET /programs/{id}/tasks/ready
  if ((m = path.match(/^\/programs\/([^/]+)\/tasks\/ready$/)) && method === 'GET') {
    const program = programs.get(m[1]!);
    if (!program || !canAccessProgram(callerOrgId, program)) {
      return { status: 404, body: errorBody('not_found', 'program not found') };
    }
    const ready = [...tasks.values()].filter((t) => t.programId === program.id && isReady(t));
    return { status: 200, body: { tasks: ready.map((t) => taskJson(t, callerOrgId)) } };
  }

  // GET /programs/{id}/tasks[?status=]
  if ((m = path.match(/^\/programs\/([^/]+)\/tasks(?:\?(.*))?$/)) && method === 'GET') {
    const program = programs.get(m[1]!);
    if (!program || !canAccessProgram(callerOrgId, program)) {
      return { status: 404, body: errorBody('not_found', 'program not found') };
    }
    const params = new URLSearchParams(m[2] ?? '');
    const status = params.get('status');
    let list = [...tasks.values()].filter((t) => t.programId === program.id);
    if (status) list = list.filter((t) => t.status === status);
    list.sort((a, b) => {
      if (a.dueOn === b.dueOn) return a.createdAt.localeCompare(b.createdAt);
      if (a.dueOn === null) return 1;
      if (b.dueOn === null) return -1;
      return a.dueOn.localeCompare(b.dueOn);
    });
    return { status: 200, body: { tasks: list.map((t) => taskJson(t, callerOrgId)) } };
  }

  // POST /programs/{id}/tasks
  if ((m = path.match(/^\/programs\/([^/]+)\/tasks$/)) && method === 'POST') {
    const program = programs.get(m[1]!);
    if (!program || !canAccessProgram(callerOrgId, program)) {
      return { status: 404, body: errorBody('not_found', 'program not found') };
    }
    const b = (body ?? {}) as Record<string, unknown>;
    const details: { field: string; message: string }[] = [];
    const title = typeof b.title === 'string' ? b.title.trim() : '';
    if (title.length < 1 || title.length > 200) {
      details.push({ field: 'title', message: 'must be between 1 and 200 characters' });
    }
    if (b.estimateMinutes !== undefined && b.estimateMinutes !== null) {
      const est = Number(b.estimateMinutes);
      if (!Number.isFinite(est) || est < 0) {
        details.push({ field: 'estimateMinutes', message: 'must not be negative' });
      }
    }
    if (details.length > 0) {
      return { status: 400, body: { error: { code: 'validation', message: 'invalid task', details } } };
    }
    const duplicate = [...tasks.values()].some((t) => t.programId === program.id && t.title === title);
    if (duplicate) {
      return { status: 409, body: errorBody('duplicate_title', `a task titled "${title}" already exists`) };
    }
    const id = `task-${++requestSeq}-new`;
    const now = new Date().toISOString();
    const created: Task = {
      id,
      programId: program.id,
      orgId: callerOrgId as string,
      title,
      status: 'todo',
      estimateMinutes: (b.estimateMinutes as number | undefined) ?? null,
      assigneeId: (b.assigneeId as string | undefined) ?? null,
      dueOn: (b.dueOn as string | undefined) ?? null,
      internalNote: (b.internalNote as string | undefined) ?? null,
      createdAt: now,
      updatedAt: now,
    };
    tasks.set(id, created);
    return { status: 201, body: taskJson(created, callerOrgId) };
  }

  // POST /tasks/{id}/dependencies
  if ((m = path.match(/^\/tasks\/([^/]+)\/dependencies$/)) && method === 'POST') {
    const successorId = m[1]!;
    const successor = tasks.get(successorId);
    if (!successor) return { status: 404, body: errorBody('not_found', 'task not found') };
    const b = (body ?? {}) as Record<string, unknown>;
    const predecessorId = String(b.predecessorId ?? '');
    const predecessor = tasks.get(predecessorId);
    if (!predecessor) return { status: 404, body: errorBody('not_found', 'predecessor not found') };
    if (predecessorId === successorId) {
      return {
        status: 409,
        body: errorBody('cycle', `${predecessor.title} -> ${predecessor.title} is a cycle`),
      };
    }
    const exists = dependencies.some(
      (d) => d.predecessorId === predecessorId && d.successorId === successorId,
    );
    if (exists) {
      return { status: 409, body: errorBody('duplicate', 'dependency already exists') };
    }
    // Adding predecessorId -> successorId closes a cycle if predecessorId is
    // already reachable by walking forward from successorId.
    const reachable = reachableFrom(successorId);
    if (reachable.has(predecessorId)) {
      return {
        status: 409,
        body: errorBody(
          'cycle',
          `${successor.title} -> ... -> ${predecessor.title} -> ${successor.title} would be a cycle`,
        ),
      };
    }
    dependencies.push({ predecessorId, successorId });
    return { status: 201, body: { predecessorId, successorId } };
  }

  // DELETE /tasks/{id}/dependencies/{predecessorId}
  if ((m = path.match(/^\/tasks\/([^/]+)\/dependencies\/([^/]+)$/)) && method === 'DELETE') {
    const successorId = m[1]!;
    const predecessorId = m[2]!;
    dependencies = dependencies.filter(
      (d) => !(d.predecessorId === predecessorId && d.successorId === successorId),
    );
    return { status: 204, body: null };
  }

  // PATCH /tasks/{id}
  if ((m = path.match(/^\/tasks\/([^/]+)$/)) && method === 'PATCH') {
    const task = tasks.get(m[1]!);
    if (!task) return { status: 404, body: errorBody('not_found', 'task not found') };
    const b = (body ?? {}) as Record<string, unknown>;
    const status = b.status as TaskStatus;
    const allowed: TaskStatus[] = ['todo', 'doing', 'blocked', 'done'];
    if (!allowed.includes(status)) {
      return {
        status: 400,
        body: {
          error: {
            code: 'validation',
            message: 'invalid status',
            details: [{ field: 'status', message: 'must be one of todo, doing, blocked, done' }],
          },
        },
      };
    }
    task.status = status;
    task.updatedAt = new Date().toISOString();
    return { status: 200, body: taskJson(task, callerOrgId) };
  }

  // GET /tasks/{id}
  if ((m = path.match(/^\/tasks\/([^/]+)$/)) && method === 'GET') {
    const task = tasks.get(m[1]!);
    if (!task) return { status: 404, body: errorBody('not_found', 'task not found') };
    return { status: 200, body: taskJson(task, callerOrgId) };
  }

  return { status: 404, body: errorBody('not_found', 'unknown route') };
}

// --- Header helpers ---------------------------------------------------

function headerValue(init: RequestInit | undefined, name: string): string | null {
  const headers = init?.headers;
  if (!headers) return null;
  if (headers instanceof Headers) return headers.get(name);
  if (Array.isArray(headers)) {
    const found = headers.find(([k]) => k.toLowerCase() === name.toLowerCase());
    return found ? found[1] : null;
  }
  const record = headers as Record<string, string>;
  const key = Object.keys(record).find((k) => k.toLowerCase() === name.toLowerCase());
  return key ? (record[key] ?? null) : null;
}

function normalizePath(input: RequestInfo | URL): string {
  const raw = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
  // Strip an origin if present (tests may pass absolute URLs).
  const withoutOrigin = raw.replace(/^https?:\/\/[^/]+/, '');
  // The real app calls relative /api/... URLs, proxied in dev by stripping
  // the /api prefix; the fake mirrors that so it works whether callers pass
  // /api/programs/... or /programs/... directly.
  return withoutOrigin.startsWith('/api/') ? withoutOrigin.slice(4) : withoutOrigin;
}

async function fakeFetchImpl(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const method = (init?.method ?? 'GET').toUpperCase();
  const path = normalizePath(input);
  const userId = headerValue(init, 'X-User-Id');
  const requestId = headerValue(init, 'X-Request-Id') ?? `fake-req-${++requestSeq}`;
  let parsedBody: unknown = undefined;
  if (init?.body) {
    try {
      parsedBody = JSON.parse(init.body as string);
    } catch {
      parsedBody = undefined;
    }
  }

  requestLog.push({ method, path, userId, body: parsedBody });

  if (latencyMs > 0) {
    await sleep(latencyMs);
  }

  if (queuedFailure) {
    const failure = queuedFailure;
    queuedFailure = null;
    return new Response(JSON.stringify(errorBody(failure.code, `forced failure: ${failure.code}`)), {
      status: failure.status,
      headers: { 'Content-Type': 'application/json', 'X-Request-Id': requestId },
    });
  }

  const result = handle(method, path, userId, parsedBody);
  return new Response(result.body === null ? null : JSON.stringify(result.body), {
    status: result.status,
    headers: { 'Content-Type': 'application/json', 'X-Request-Id': requestId },
  });
}

export function installFakeApi(): FakeApiControls {
  seedNow();
  latencyMs = 0;
  queuedFailure = null;
  requestLog = [];

  globalThis.fetch = fakeFetchImpl as typeof fetch;

  return {
    latency(ms: number) {
      latencyMs = ms;
    },
    failNext(code: string, status = 500) {
      queuedFailure = { code, status };
    },
    get requests() {
      return requestLog;
    },
    reset() {
      seedNow();
      latencyMs = 0;
      queuedFailure = null;
      requestLog = [];
    },
    hasDependency(predecessorId: string, successorId: string) {
      return dependencies.some((d) => d.predecessorId === predecessorId && d.successorId === successorId);
    },
  };
}
