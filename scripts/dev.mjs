#!/usr/bin/env node
// Runs the API and the web app together, prefixing each line of output and
// killing both when you Ctrl+C.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { getEnv } from './lib/env.mjs';

const isWindows = process.platform === 'win32';

function prefixLines(prefix, chunk, stream) {
  const text = chunk.toString();
  const lines = text.split(/\r?\n/);
  // Drop the trailing empty segment produced by a final newline.
  if (lines[lines.length - 1] === '') lines.pop();
  for (const line of lines) {
    stream.write(`[${prefix}] ${line}\n`);
  }
}

const children = [];

function launch(prefix, cmd, args, opts) {
  // On POSIX, detach into its own process group so we can signal the whole
  // tree (e.g. `go run`'s compiled binary child) via a negative pid. On
  // Windows, `go run` and `pnpm` similarly spawn a real child process that
  // survives a plain kill of the wrapper; we tree-kill by pid with taskkill
  // instead (see shutdown()).
  const child = spawn(cmd, args, { shell: false, detached: !isWindows, ...opts });
  child.stdout.on('data', (chunk) => prefixLines(prefix, chunk, process.stdout));
  child.stderr.on('data', (chunk) => prefixLines(prefix, chunk, process.stderr));
  child.on('error', (err) => {
    console.error(`[${prefix}] failed to start: ${err.message}`);
  });
  children.push({ prefix, child });
  return child;
}

let started = 0;

if (existsSync('api')) {
  launch('api', 'go', ['run', './cmd/api'], { cwd: 'api', env: { ...process.env, ...getEnv() } });
  started++;
} else {
  console.log('[api] api/ does not exist yet, skipping.');
}

if (existsSync('web')) {
  launch('web', 'pnpm', ['--filter', 'web', 'dev'], { env: process.env });
  started++;
} else {
  console.log('[web] web/ does not exist yet, skipping.');
}

if (started === 0) {
  console.log('Nothing to run yet: neither api/ nor web/ exist.');
  process.exit(0);
}

let shuttingDown = false;
function killTree(child) {
  if (child.killed || child.exitCode !== null) return;
  if (isWindows) {
    // `go run` and `pnpm --filter web dev` each spawn a real child process
    // (the compiled binary, the underlying vite process). Killing only the
    // wrapper leaves that grandchild running and holding its port, so kill
    // the whole process tree by pid instead.
    spawnSync('taskkill', ['/pid', String(child.pid), '/T', '/F']);
  } else {
    try {
      process.kill(-child.pid, 'SIGTERM');
    } catch {
      child.kill('SIGTERM');
    }
  }
}

function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const { child } of children) {
    killTree(child);
  }
  process.exit(0);
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

for (const { prefix, child } of children) {
  child.on('exit', (code) => {
    console.log(`[${prefix}] exited with code ${code}`);
  });
}
