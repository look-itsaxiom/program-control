import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// React Testing Library registers its own cleanup only when `afterEach` is a global, and this
// project runs Vitest without `globals: true`. Without this hook every render stays mounted in
// the document, so the second test in a file sees the first test's rows and every `getByText`
// finds two of everything.
//
// Unmounting also settles the fake API in src/phase-tests/fake-api.ts: its latency and
// `failNext` controls are module-level, so a request still in flight when a test ends would
// otherwise wake up inside the next test and eat the failure that test had queued. installFakeApi
// stamps every request with the install it started under and ignores stale ones, and cleanup()
// here takes away the components that would have rendered their answers.
afterEach(() => {
  cleanup();
});
