#!/usr/bin/env node
// db.mjs <up|down|migrate|status|reset|ensure>
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { getDatabaseUrl } from './lib/env.mjs';

const command = process.argv[2];
const valid = ['up', 'down', 'migrate', 'status', 'reset', 'ensure'];

if (!valid.includes(command)) {
  console.error(`Usage: node scripts/db.mjs <${valid.join('|')}>`);
  process.exit(1);
}

function run(cmd, args, opts = {}) {
  const result = spawnSync(cmd, args, { stdio: 'inherit', shell: false, ...opts });
  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }
  return result.status ?? 1;
}

function composeUp() {
  const status = run('docker', ['compose', 'up', '-d', '--wait']);
  if (status !== 0) process.exit(status);
}

function composeDown() {
  const status = run('docker', ['compose', 'down']);
  if (status !== 0) process.exit(status);
}

function pgReady() {
  const result = spawnSync(
    'docker',
    ['compose', 'exec', '-T', 'db', 'pg_isready', '-U', 'postgres', '-d', 'program_control'],
    { stdio: 'ignore' },
  );
  return result.status === 0;
}

function requireApi() {
  if (!existsSync('api')) {
    console.log(
      "api/ does not exist yet, skipping. Once Task 2 adds the api module, 'node scripts/db.mjs' will drive its migrator.",
    );
    return false;
  }
  return true;
}

function runMigrate(args) {
  if (!requireApi()) return;
  const databaseUrl = getDatabaseUrl();
  const status = run('go', ['run', './cmd/migrate', ...args], {
    cwd: 'api',
    env: { ...process.env, DATABASE_URL: databaseUrl },
  });
  if (status !== 0) process.exit(status);
}

switch (command) {
  case 'up':
    composeUp();
    break;
  case 'down':
    composeDown();
    break;
  case 'migrate':
    runMigrate(['up']);
    break;
  case 'status':
    runMigrate(['status']);
    break;
  case 'reset':
    runMigrate(['reset']);
    composeUp();
    break;
  case 'ensure':
    if (!pgReady()) {
      composeUp();
    } else {
      console.log('Postgres is already up.');
    }
    break;
}
