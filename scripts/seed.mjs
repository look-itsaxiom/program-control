#!/usr/bin/env node
// seed.mjs
//
// Runs `go run ./cmd/seed` in api/ with DATABASE_URL set, per phase 7's S7.1 contract
// (phases/7-ship/STORIES.md). cmd/seed itself is written when S7.1 is implemented; until then
// this script fails with Go's own "no such directory" error, which is expected.
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { getDatabaseUrl } from './lib/env.mjs';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
const apiDir = path.join(path.dirname(scriptsDir), 'api');
const isWindows = process.platform === 'win32';

const result = spawnSync(isWindows ? 'go.exe' : 'go', ['run', './cmd/seed'], {
  cwd: apiDir,
  env: { ...process.env, DATABASE_URL: getDatabaseUrl() },
  stdio: 'inherit',
  shell: false,
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);
