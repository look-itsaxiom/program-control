// fake-api.ts
//
// An in-memory implementation of the phase 2 REST contract (see
// phases/2-rest-api/STORIES.md), installed as globalThis.fetch so phase 3
// and phase 6 tests can render the real App against a fake server instead of
// a live Go process.
//
// The seed is the same world api/phase_tests/seed_test.go builds, task for
// task: one customer org owning the program, two vendor orgs it is shared
// with (A as editor, B as viewer), four users (Alice, Bob and Carol in an org
// each, Dave in none), and six tasks in a diamond (root before branchA and
// branchB, both before join) plus two tasks outside it. Task ownership is
// spread across all three organizations so the internal-note rule is
// exercised on more than one owner. Keeping both fakes on one world means a
// story means the same thing whether it is checked against Go or the browser.

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
  programName: 'Engine Overhaul',
  emptyProgramId: 'program-empty',
  tasks: {
    root: 'task-1',
    branchA: 'task-2',
    branchB: 'task-3',
    join: 'task-4',
    independent: 'task-5',
    blocked: 'task-6',
  },
  titles: {
    root: 'Draft engine spec',
    branchA: 'Machine the housing',
    branchB: 'Cast the manifold',
    join: 'Assemble and test',
    independent: 'Order replacement gaskets',
    blocked: 'Certify for export',
  },
  internalNoteText: 'Vendor B under-bid this contract; do not disclose the target margin.',
} as const;

/** The owning organization of each seeded task, mirroring seed_test.go. */
export const SEED_TASK_ORGS = {
  root: SEED.orgs.customer,
  branchA: SEED.orgs.vendorA,
  branchB: SEED.orgs.vendorB,
  join: SEED.orgs.customer,
  independent: SEED.orgs.vendorA,
  blocked: SEED.orgs.customer,
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
    [SEED.programId, { id: SEED.programId, orgId: SEED.orgs.customer, name: SEED.programName }],
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

  // Seeded tasks are created one minute apart, oldest first, so S2.2's
  // created_at tiebreak is a real ordering rather than an accident of
  // insertion order. seed_test.go spaces its rows the same way.
  const base = Date.UTC(2026, 8, 1, 9, 0, 0);
  let minute = 0;
  const t = (
    id: string,
    title: string,
    orgId: string,
    status: TaskStatus,
    opts: Partial<Pick<Task, 'assigneeId' | 'estimateMinutes' | 'dueOn' | 'internalNote'>> = {},
  ): Task => {
    const at = new Date(base + minute++ * 60_000).toISOString();
    return {
      id,
      programId: SEED.programId,
      orgId,
      title,
      status,
      estimateMinutes: opts.estimateMinutes ?? null,
      assigneeId: opts.assigneeId ?? null,
      dueOn: opts.dueOn ?? null,
      internalNote: opts.internalNote ?? null,
      createdAt: at,
      updatedAt: at,
    };
  };

  tasks = new Map(
    [
      t(SEED.tasks.root, SEED.titles.root, SEED_TASK_ORGS.root, 'todo', {
        internalNote: SEED.internalNoteText,
      }),
      t(SEED.tasks.branchA, SEED.titles.branchA, SEED_TASK_ORGS.branchA, 'todo'),
      t(SEED.tasks.branchB, SEED.titles.branchB, SEED_TASK_ORGS.branchB, 'todo'),
      t(SEED.tasks.join, SEED.titles.join, SEED_TASK_ORGS.join, 'todo'),
      t(SEED.tasks.independent, SEED.titles.independent, SEED_TASK_ORGS.independent, 'todo'),
      t(SEED.tasks.blocked, SEED.titles.blocked, SEED_TASK_ORGS.blocked, 'blocked'),
    ].map((task) => [task.id, task]),
  );

  // The diamond: root -> branchA, root -> branchB, branchA -> join,
  // branchB -> join. independent and blocked stand outside it entirely, with
  // no predecessors and no successors.
  dependencies = [
    { predecessorId: SEED.tasks.root, successorId: SEED.tasks.branchA },
    { predecessorId: SEED.tasks.root, successorId: SEED.tasks.branchB },
    { predecessorId: SEED.tasks.branchA, successorId: SEED.tasks.join },
    { predecessorId: SEED.tasks.branchB, successorId: SEED.tasks.join },
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

// canSeeTask applies S2.8 to a single task: a caller who cannot reach the
// task's program cannot tell the task exists either, so its routes answer 404
// exactly as they would for an id nobody has ever used.
function canSeeTask(callerOrgId: string | null, task: Task): boolean {
  const program = programs.get(task.programId);
  if (!program) return false;
  return canAccessProgram(callerOrgId, program);
}

// isCalendarDate accepts only a real YYYY-MM-DD date: the shape has to match
// and the parsed value has to round-trip, so "2026-02-30" is rejected rather
// than silently rolled forward the way Date's own parser would.
function isCalendarDate(value: unknown): boolean {
  if (typeof value !== 'string') return false;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!m) return false;
  const year = Number(m[1]);
  const month = Number(m[2]);
  const day = Number(m[3]);
  const parsed = new Date(Date.UTC(year, month - 1, day));
  return (
    parsed.getUTCFullYear() === year &&
    parsed.getUTCMonth() === month - 1 &&
    parsed.getUTCDate() === day
  );
}

// --- Route handling ---------------------------------------------------

interface HandlerResult {
  status: number;
  body: unknown;
  headers?: Record<string, string>;
}

// Every route this phase defines, with the methods that route accepts. A
// request whose path matches one of these but whose method is not in its list
// is S2.7's 405 with an Allow header, not a 404.
const ROUTES: { pattern: RegExp; methods: string[] }[] = [
  { pattern: /^\/programs\/[^/]+\/tasks\/ready$/, methods: ['GET'] },
  { pattern: /^\/programs\/[^/]+\/tasks$/, methods: ['GET', 'POST'] },
  { pattern: /^\/tasks\/[^/]+\/dependencies$/, methods: ['POST'] },
  { pattern: /^\/tasks\/[^/]+\/dependencies\/[^/]+$/, methods: ['DELETE'] },
  { pattern: /^\/tasks\/[^/]+$/, methods: ['GET', 'PATCH'] },
];

/** The Allow header value for a known path called with a method it does not accept, else null. */
function methodNotAllowed(method: string, pathWithoutQuery: string): string | null {
  for (const route of ROUTES) {
    if (route.pattern.test(pathWithoutQuery) && !route.methods.includes(method)) {
      return route.methods.join(', ');
    }
  }
  return null;
}

function handle(method: string, path: string, userId: string | null, body: unknown): HandlerResult {
  // dave is a known-but-orgless user: he authenticates but belongs nowhere.
  const isKnownUser = userId !== null && (membership.has(userId) || userId === SEED.users.dave);
  if (!isKnownUser) {
    return { status: 401, body: errorBody('unauthenticated', 'missing or unknown X-User-Id') };
  }
  const callerOrgId = callerOrgFor(userId);
  const pathWithoutQuery = path.split('?')[0] ?? path;

  const allow = methodNotAllowed(method, pathWithoutQuery);
  if (allow !== null) {
    return {
      status: 405,
      body: errorBody('method_not_allowed', `${method} is not allowed on ${pathWithoutQuery}`),
      headers: { Allow: allow },
    };
  }

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
    if (title.length < 1) {
      details.push({ field: 'title', message: 'must not be empty or whitespace-only' });
    }
    if (b.estimateMinutes !== undefined && b.estimateMinutes !== null) {
      const est = Number(b.estimateMinutes);
      if (!Number.isFinite(est) || est < 0) {
        details.push({ field: 'estimateMinutes', message: 'must not be negative' });
      }
    }
    if (b.dueOn !== undefined && b.dueOn !== null && !isCalendarDate(b.dueOn)) {
      details.push({ field: 'dueOn', message: 'must be a YYYY-MM-DD calendar date' });
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
    return {
      status: 201,
      body: taskJson(created, callerOrgId),
      headers: { Location: `/tasks/${id}` },
    };
  }

  // POST /tasks/{id}/dependencies
  if ((m = path.match(/^\/tasks\/([^/]+)\/dependencies$/)) && method === 'POST') {
    const successorId = m[1]!;
    const successor = tasks.get(successorId);
    if (!successor || !canSeeTask(callerOrgId, successor)) {
      return { status: 404, body: errorBody('not_found', 'task not found') };
    }
    const b = (body ?? {}) as Record<string, unknown>;
    const predecessorId = String(b.predecessorId ?? '');
    const predecessor = tasks.get(predecessorId);
    if (!predecessor || !canSeeTask(callerOrgId, predecessor)) {
      return { status: 404, body: errorBody('not_found', 'predecessor not found') };
    }
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
    if (!task || !canSeeTask(callerOrgId, task)) {
      return { status: 404, body: errorBody('not_found', 'task not found') };
    }
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
    if (!task || !canSeeTask(callerOrgId, task)) {
      return { status: 404, body: errorBody('not_found', 'task not found') };
    }
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
    headers: {
      'Content-Type': 'application/json',
      'X-Request-Id': requestId,
      ...(result.headers ?? {}),
    },
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
