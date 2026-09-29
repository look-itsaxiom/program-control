import { describe } from 'vitest';

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
