#!/usr/bin/env node
// Runs the API and the web app together, prefixing each line of output and
// killing both when you Ctrl+C.
import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { getEnv } from './lib/env.mjs';

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
  const child = spawn(cmd, args, { shell: false, ...opts });
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
function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const { child } of children) {
    if (!child.killed) child.kill('SIGTERM');
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
