#!/usr/bin/env node
// One command from a fresh clone to a running dev environment.
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));

function run(cmd, args, opts = {}) {
  console.log(`\n> ${cmd} ${args.join(' ')}`);
  const result = spawnSync(cmd, args, { stdio: 'inherit', shell: false, ...opts });
  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }
  if ((result.status ?? 1) !== 0) {
    process.exit(result.status);
  }
}

// 1. Doctor, fail fast.
run('node', [path.join(scriptsDir, 'doctor-launch.mjs')]);

// 2. Install web/workspace dependencies.
run('pnpm', ['install']);

// 3. Bring up Postgres.
run('docker', ['compose', 'up', '-d', '--wait']);

// 4. Download Go module dependencies, if the api module exists yet.
if (existsSync('api')) {
  run('go', ['mod', 'download'], { cwd: 'api' });
} else {
  console.log('\napi/ does not exist yet, skipping go mod download.');
}

// 5. Apply migrations (skips gracefully if api/ is absent).
run('node', [path.join(scriptsDir, 'db.mjs'), 'migrate']);

console.log(
  '\nNext: open phases/0-scaffold/STORIES.md, run pnpm check 0, and run `claude` in this folder to meet your mentor.',
);
