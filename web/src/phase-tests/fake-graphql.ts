// fake-graphql.ts
//
// An in-memory implementation of the phase 4 GraphQL contract (see
// phases/4-graphql/STORIES.md): it executes the SDL from that contract, with `graphql-js`,
// against an in-memory seed, and installs itself on `globalThis.fetch` for the single path
// `/graphql`, so phase 4 tests can render the real `App` (with
// `localStorage['programControl.transport'] === 'graphql'`) against a fake server instead of a
// live Go process.
//
// The seed mirrors fake-api.ts (same orgs, users, program, task ids, titles, statuses and
// dependency shape, which in turn mirror api/phase_tests/seed_test.go), so a test can log in as
// the same alice/bob/carol/dave and land on the same board regardless of which transport it
// exercises. It does not import fake-api's mutable state (that
// file keeps it module-private); it builds its own independent copy of the same scenario.

import { buildSchema, execute, parse, GraphQLError, type ExecutionResult } from 'graphql';
import { SEED, SEED_TASK_ORGS } from './fake-api';

// --- schema (verbatim from the phase 4 contract) ---------------------------

const schema = buildSchema(`
  type Query { program(id: ID!): Program  me: User! }
  type Mutation { setTaskStatus(taskId: ID!, status: TaskStatus!): Task!  addDependency(taskId: ID!, predecessorId: ID!): Task! }
  type Program { id: ID!  name: String!  tasks(status: TaskStatus): [Task!]!  readyTasks: [Task!]! }
  type Task { id: ID!  title: String!  status: TaskStatus!  estimateMinutes: Int  dueOn: String  internalNote: String  assignee: User  predecessors: [Task!]!  program: Program! }
  type User { id: ID!  displayName: String!  email: String! }
  enum TaskStatus { TODO DOING BLOCKED DONE }
`);

// --- in-memory seed ----------------------------------------------------------

interface SeedUser {
  id: string;
  displayName: string;
  email: string;
  orgId: string | null; // dave belongs nowhere
}

interface SeedTask {
  id: string;
  programId: string;
  orgId: string;
  title: string;
  status: string; // TaskStatus enum value, e.g. "TODO"
  estimateMinutes: number | null;
  dueOn: string | null;
  internalNote: string | null;
  assigneeId: string | null;
  predecessorIds: string[];
}

const seedUsers: SeedUser[] = [
  { id: SEED.users.alice, displayName: 'Alice Customer', email: 'alice@customer.example', orgId: SEED.orgs.customer },
  { id: SEED.users.bob, displayName: 'Bob VendorA', email: 'bob@vendora.example', orgId: SEED.orgs.vendorA },
  { id: SEED.users.carol, displayName: 'Carol VendorB', email: 'carol@vendorb.example', orgId: SEED.orgs.vendorB },
  { id: SEED.users.dave, displayName: 'Dave Stranger', email: 'dave@nowhere.example', orgId: null },
];

// The same world fake-api.ts and api/phase_tests/seed_test.go build: a diamond
// (root -> branchA, root -> branchB, both -> join), plus independent and
// blocked outside it. Ownership is spread across all three organizations, and
// root carries the one internal note.
const seedTaskDefs: SeedTask[] = [
  {
    id: SEED.tasks.root, programId: SEED.programId, orgId: SEED_TASK_ORGS.root, title: SEED.titles.root,
    status: 'TODO', estimateMinutes: null, dueOn: null, internalNote: SEED.internalNoteText,
    assigneeId: null, predecessorIds: [],
  },
  {
    id: SEED.tasks.branchA, programId: SEED.programId, orgId: SEED_TASK_ORGS.branchA, title: SEED.titles.branchA,
    status: 'TODO', estimateMinutes: null, dueOn: null, internalNote: null,
    assigneeId: null, predecessorIds: [SEED.tasks.root],
  },
  {
    id: SEED.tasks.branchB, programId: SEED.programId, orgId: SEED_TASK_ORGS.branchB, title: SEED.titles.branchB,
    status: 'TODO', estimateMinutes: null, dueOn: null, internalNote: null,
    assigneeId: null, predecessorIds: [SEED.tasks.root],
  },
  {
    id: SEED.tasks.join, programId: SEED.programId, orgId: SEED_TASK_ORGS.join, title: SEED.titles.join,
    status: 'TODO', estimateMinutes: null, dueOn: null, internalNote: null,
    assigneeId: null, predecessorIds: [SEED.tasks.branchA, SEED.tasks.branchB],
  },
  {
    id: SEED.tasks.independent, programId: SEED.programId, orgId: SEED_TASK_ORGS.independent, title: SEED.titles.independent,
    status: 'TODO', estimateMinutes: null, dueOn: null, internalNote: null,
    assigneeId: null, predecessorIds: [],
  },
  {
    id: SEED.tasks.blocked, programId: SEED.programId, orgId: SEED_TASK_ORGS.blocked, title: SEED.titles.blocked,
    status: 'BLOCKED', estimateMinutes: null, dueOn: null, internalNote: null,
    assigneeId: null, predecessorIds: [],
  },
];

let tasks: SeedTask[] = [];
let taskEvents: { taskId: string; toStatus: string }[] = [];

// --- operation log, for test assertions -------------------------------------

export interface RecordedOperation {
  operationName: string | null;
  query: string;
  variables: unknown;
}

/** Every GraphQL operation the fake has executed, in order. Reset by resetGraphqlSeed(). */
export const operations: RecordedOperation[] = [];

/** Every fetch call the fake delegated onward (i.e. was not for /graphql). */
export const delegatedCalls: string[] = [];

/** Resets the in-memory store back to the seed and clears the operation/delegation logs. */
export function resetGraphqlSeed(): void {
  tasks = seedTaskDefs.map((t) => ({ ...t, predecessorIds: [...t.predecessorIds] }));
  taskEvents = [];
  operations.length = 0;
  delegatedCalls.length = 0;
}
resetGraphqlSeed();

function operationNameOf(query: string): string | null {
  return /(?:query|mutation)\s+(\w+)/.exec(query)?.[1] ?? null;
}

/** How many `program(...)` queries the fake has executed since the last reset. */
export function programQueryCount(): number {
  return operations.filter((op) => /\bprogram\s*\(/.test(op.query)).length;
}

// --- resolvers -----------------------------------------------------------------

interface Ctx {
  userId: string | null;
}

function findUser(id: string | null): SeedUser | undefined {
  if (id === null) return undefined;
  return seedUsers.find((u) => u.id === id);
}

function requireAuth(ctx: Ctx): SeedUser {
  const user = findUser(ctx.userId);
  if (!user) {
    throw new GraphQLError('unauthenticated', { extensions: { code: 'UNAUTHENTICATED' } });
  }
  return user;
}

function makeUserView(u: SeedUser) {
  return { id: u.id, displayName: u.displayName, email: u.email };
}

function makeTaskView(t: SeedTask) {
  return {
    id: t.id,
    title: t.title,
    status: t.status,
    estimateMinutes: t.estimateMinutes,
    dueOn: t.dueOn,
    internalNote: (_args: unknown, ctx: Ctx) => {
      const caller = findUser(ctx.userId);
      return caller && caller.orgId === t.orgId ? t.internalNote : null;
    },
    assignee: () => {
      const u = t.assigneeId ? findUser(t.assigneeId) : undefined;
      return u ? makeUserView(u) : null;
    },
    predecessors: () =>
      t.predecessorIds
        .map((id) => tasks.find((x) => x.id === id))
        .filter((x): x is SeedTask => Boolean(x))
        .map(makeTaskView),
    program: () => makeProgramView(t.programId),
  };
}

function makeProgramView(programId: string) {
  return {
    id: programId,
    name: SEED.programName,
    tasks: (args: { status?: string }) => {
      let list = tasks.filter((t) => t.programId === programId);
      if (args?.status) list = list.filter((t) => t.status === args.status);
      return list.map(makeTaskView);
    },
    readyTasks: () => {
      const byId = new Map(tasks.map((t) => [t.id, t]));
      return tasks
        .filter((t) => t.programId === programId)
        .filter((t) => t.status === 'TODO' && t.predecessorIds.every((pid) => byId.get(pid)?.status === 'DONE'))
        .map(makeTaskView);
    },
  };
}

// isAncestor reports whether `candidate` is already a transitive predecessor of `of`, by walking
// `of`'s predecessor chain.
function isAncestor(candidate: string, of: string, seen = new Set<string>()): boolean {
  if (seen.has(of)) return false;
  seen.add(of);
  const t = tasks.find((x) => x.id === of);
  if (!t) return false;
  for (const pid of t.predecessorIds) {
    if (pid === candidate || isAncestor(candidate, pid, seen)) return true;
  }
  return false;
}

const root = {
  me: (_args: unknown, ctx: Ctx) => makeUserView(requireAuth(ctx)),

  program: (args: { id: string }, ctx: Ctx) => {
    requireAuth(ctx);
    if (!tasks.some((t) => t.programId === args.id) && args.id !== SEED.programId && args.id !== SEED.emptyProgramId) {
      throw new GraphQLError('program not found', { extensions: { code: 'NOT_FOUND' } });
    }
    return makeProgramView(args.id);
  },

  setTaskStatus: (args: { taskId: string; status: string }, ctx: Ctx) => {
    requireAuth(ctx);
    const t = tasks.find((x) => x.id === args.taskId);
    if (!t) throw new GraphQLError('task not found', { extensions: { code: 'NOT_FOUND' } });
    t.status = args.status;
    taskEvents.push({ taskId: t.id, toStatus: args.status });
    return makeTaskView(t);
  },

  addDependency: (args: { taskId: string; predecessorId: string }, ctx: Ctx) => {
    requireAuth(ctx);
    const t = tasks.find((x) => x.id === args.taskId);
    if (!t) throw new GraphQLError('task not found', { extensions: { code: 'NOT_FOUND' } });
    if (args.taskId === args.predecessorId || isAncestor(args.taskId, args.predecessorId)) {
      throw new GraphQLError(`adding this dependency would create a cycle`, { extensions: { code: 'CYCLE' } });
    }
    t.predecessorIds.push(args.predecessorId);
    return makeTaskView(t);
  },
};

// --- fetch install/uninstall ------------------------------------------------

let previousFetch: typeof fetch | undefined;

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

async function fakeGraphqlFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const url = requestUrl(input);
  if (!url.includes('/graphql')) {
    delegatedCalls.push(url);
    if (previousFetch) return previousFetch(input, init);
    throw new Error(`fake-graphql: no fetch installed for ${url}`);
  }

  const bodyText = typeof init?.body === 'string' ? init.body : '{}';
  const { query, variables, operationName } = JSON.parse(bodyText) as {
    query: string;
    variables?: unknown;
    operationName?: string;
  };

  const headers = new Headers(init?.headers);
  const userId = headers.get('X-User-Id');
  operations.push({ operationName: operationName ?? operationNameOf(query), query, variables });

  const result: ExecutionResult = await execute({
    schema,
    document: parse(query),
    rootValue: root,
    contextValue: { userId },
    variableValues: variables as { [key: string]: unknown } | undefined,
  });

  return new Response(JSON.stringify(result), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Installs the fake as globalThis.fetch, capturing whatever fetch was active for delegation. */
export function installFakeGraphql(): void {
  previousFetch = globalThis.fetch;
  globalThis.fetch = fakeGraphqlFetch as typeof fetch;
}

/** Restores whatever fetch was active before installFakeGraphql() was called. */
export function uninstallFakeGraphql(): void {
  if (previousFetch) globalThis.fetch = previousFetch;
  previousFetch = undefined;
}

export { SEED };
