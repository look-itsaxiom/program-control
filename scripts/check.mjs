#!/usr/bin/env node
// check.mjs [N] [--verbose]
//
// Runs one phase's (or, with no argument, phases 0-7's) acceptance tests -
// the Go tests in api/phase_tests and the Vitest tests in web/src/phase-tests
// - and prints a pass/fail scoreboard. Exits 0 only when every test that ran
// for the requested phase(s) passed.
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { getDatabaseUrl } from './lib/env.mjs';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.dirname(scriptsDir);
const apiDir = path.join(projectRoot, 'api');
const isWindows = process.platform === 'win32';

function parseArgs(argv) {
  let phase = null;
  let verbose = false;
  for (const arg of argv) {
    if (arg === '--verbose' || arg === '-v') {
      verbose = true;
    } else if (/^\d+$/.test(arg)) {
      phase = Number(arg);
    } else {
      console.error(`Usage: node scripts/check.mjs [phase] [--verbose]`);
      process.exit(1);
    }
  }
  return { phase, verbose };
}

function ensureDb() {
  const result = spawnSync(process.execPath, [path.join(scriptsDir, 'db.mjs'), 'ensure'], {
    stdio: 'inherit',
    shell: false,
  });
  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

// spawnPnpm runs a pnpm subcommand, resolving pnpm.cmd via a shell on Windows
// (pnpm ships as a shim with no native Windows executable) and running it
// directly everywhere else.
function spawnPnpm(args, opts = {}) {
  if (isWindows) {
    return spawnSync('pnpm.cmd', args, { shell: true, encoding: 'utf8', ...opts });
  }
  return spawnSync('pnpm', args, { shell: false, encoding: 'utf8', ...opts });
}

// --- Go phase tests -------------------------------------------------------

// extractGoFailureMessage returns the first substantive output line from a
// test's captured output (skipping the === RUN / --- FAIL/PASS/SKIP banner
// lines), which is normally the t.Fatalf/t.Errorf call site and message.
function extractGoFailureMessage(output) {
  for (const rawLine of output.split('\n')) {
    const line = rawLine.trim();
    if (!line) continue;
    if (line.startsWith('=== RUN') || line.startsWith('=== PAUSE') || line.startsWith('=== CONT')) {
      continue;
    }
    if (line.startsWith('--- FAIL') || line.startsWith('--- PASS') || line.startsWith('--- SKIP')) {
      continue;
    }
    return line;
  }
  return '';
}

function runGoPhase(n, verbose) {
  const databaseUrl = getDatabaseUrl();
  const args = ['test', './phase_tests/', '-p', '1', '-count=1', '-run', `^TestPhase${n}_`, '-json'];
  const result = spawnSync('go', args, {
    cwd: apiDir,
    env: { ...process.env, DATABASE_URL: databaseUrl },
    encoding: 'utf8',
    shell: false,
  });
  if (result.error) {
    throw new Error(`Failed to run go: ${result.error.message}`);
  }

  const stdout = result.stdout || '';
  const events = [];
  for (const line of stdout.split('\n')) {
    if (!line.trim()) continue;
    try {
      events.push(JSON.parse(line));
    } catch {
      // Non-JSON line (shouldn't normally happen with -json); ignore.
    }
  }

  if (verbose) {
    for (const ev of events) {
      if (ev.Action === 'output' && ev.Output) process.stdout.write(ev.Output);
    }
    if (result.stderr) process.stderr.write(result.stderr);
  }

  const byTest = new Map();
  for (const ev of events) {
    if (!ev.Test) continue; // package-level event, not a per-test one
    let entry = byTest.get(ev.Test);
    if (!entry) {
      entry = { name: ev.Test, status: null, output: '' };
      byTest.set(ev.Test, entry);
    }
    if (ev.Action === 'output' && ev.Output) {
      entry.output += ev.Output;
    } else if (ev.Action === 'pass' || ev.Action === 'fail' || ev.Action === 'skip') {
      entry.status = ev.Action;
    }
  }

  // Go subtests (t.Run) report as their own "Name/subname" events. Count only
  // the top-level test toward the scoreboard total; a subtest failure already
  // fails its parent's own pass/fail event, but the parent's own captured
  // output rarely has the assertion detail (that's on the subtest's output),
  // so borrow the first failing subtest's message for display.
  const topLevel = new Map();
  const firstFailingSubtest = new Map(); // parent name -> its entry
  for (const entry of byTest.values()) {
    const slash = entry.name.indexOf('/');
    if (slash === -1) {
      topLevel.set(entry.name, entry);
    } else if (entry.status === 'fail') {
      const parentName = entry.name.slice(0, slash);
      if (!firstFailingSubtest.has(parentName)) firstFailingSubtest.set(parentName, entry);
    }
  }

  const tests = [...topLevel.values()].map((entry) => {
    if (entry.status !== 'fail') return { name: entry.name, status: entry.status, message: '' };
    let message = extractGoFailureMessage(entry.output);
    if (!message && firstFailingSubtest.has(entry.name)) {
      message = extractGoFailureMessage(firstFailingSubtest.get(entry.name).output);
    }
    return { name: entry.name, status: entry.status, message };
  });

  if (tests.length === 0 && result.status !== 0) {
    // The package failed to build/run before any test could report - surface
    // it as a failure rather than silently reporting "no tests yet".
    tests.push({
      name: `go test ./phase_tests/ -run '^TestPhase${n}_'`,
      status: 'fail',
      message: extractGoFailureMessage(`${stdout}\n${result.stderr || ''}`) || 'go test failed to run',
    });
    if (!verbose && result.stderr) process.stderr.write(result.stderr);
  }

  return tests;
}

// --- Web (Vitest) phase tests ----------------------------------------------

function statusFromVitest(status) {
  if (status === 'passed') return 'pass';
  if (status === 'failed') return 'fail';
  return 'skip'; // skipped, pending, todo
}

// phaseFullNameMatches checks whether a Vitest assertion's fullName belongs to
// phase n. It requires a word boundary after the number so "phase 1" never
// matches "phase 10" (this project only has single-digit phases, 0 through
// 7, but the boundary check costs nothing and avoids that class of bug).
function phaseFullNameMatches(fullName, n) {
  return new RegExp(`\\bphase ${n}(?!\\d)`).test(fullName);
}

function runVitestPhase(n, verbose, tmpDir) {
  const outFile = path.join(tmpDir, `vitest-phase-${n}.json`);
  const args = ['--filter', 'web', 'exec', 'vitest', 'run'];
  if (verbose) {
    args.push('--reporter=verbose', '--reporter=json', `--outputFile.json=${outFile}`);
  } else {
    args.push('--reporter=json', `--outputFile=${outFile}`);
  }
  args.push('-t', `phase ${n}`);

  const result = spawnPnpm(args, {
    cwd: projectRoot,
    stdio: verbose ? 'inherit' : 'pipe',
  });
  if (result.error) {
    throw new Error(`Failed to run pnpm: ${result.error.message}`);
  }

  let report;
  try {
    report = JSON.parse(readFileSync(outFile, 'utf8'));
  } catch {
    // Vitest found nothing to run at all (e.g. no test files match), or
    // crashed before writing a report.
    if (result.status !== 0) {
      return [
        {
          name: `vitest -t "phase ${n}"`,
          status: 'fail',
          message: 'vitest failed to produce a report; run with --verbose for details',
        },
      ];
    }
    return [];
  }

  const tests = [];
  for (const fileResult of report.testResults || []) {
    for (const assertion of fileResult.assertionResults || []) {
      if (!assertion.fullName || !phaseFullNameMatches(assertion.fullName, n)) continue;
      const status = statusFromVitest(assertion.status);
      tests.push({
        name: assertion.fullName,
        status,
        message: status === 'fail' ? (assertion.failureMessages?.[0] || '').split('\n')[0] : '',
      });
    }
  }
  return tests;
}

// --- Scoreboard --------------------------------------------------------

function summarize(tests) {
  const passed = tests.filter((t) => t.status === 'pass').length;
  const failed = tests.filter((t) => t.status === 'fail').length;
  const skipped = tests.filter((t) => t.status === 'skip').length;
  return { passed, failed, skipped, total: passed + failed };
}

function printTestLines(tests) {
  for (const t of tests) {
    if (t.status === 'pass') {
      console.log(`  ✓ ${t.name}`);
    } else if (t.status === 'fail') {
      console.log(`  ✗ ${t.name}`);
      if (t.message) console.log(`      ${t.message}`);
    } else {
      console.log(`  - ${t.name} (skipped)`);
    }
  }
}

function checkPhase(n, { verbose } = {}, tmpDir) {
  const tests = [...runGoPhase(n, verbose), ...runVitestPhase(n, verbose, tmpDir)];
  const { passed, failed, skipped, total } = summarize(tests);
  return { phase: n, tests, passed, failed, skipped, total };
}

function printPhase(result) {
  const { phase, tests, passed, total } = result;
  if (tests.length === 0) {
    console.log(`Phase ${phase}: no tests yet`);
    return;
  }
  console.log(`Phase ${phase}: ${passed}/${total} passing`);
  printTestLines(tests);
}

function main() {
  const { phase, verbose } = parseArgs(process.argv.slice(2));

  ensureDb();

  const tmpDir = mkdtempSync(path.join(tmpdir(), 'program-control-check-'));
  let exitCode = 0;
  try {
    if (phase !== null) {
      const result = checkPhase(phase, { verbose }, tmpDir);
      printPhase(result);
      exitCode = result.failed > 0 ? 1 : 0;
    } else {
      const results = [];
      for (const n of [0, 1, 2, 3, 4, 5, 6, 7]) {
        results.push(checkPhase(n, { verbose }, tmpDir));
      }

      console.log('Phase  Passing  Status');
      let anyFailed = false;
      for (const result of results) {
        const { phase: n, passed, total, failed, tests } = result;
        if (failed > 0) anyFailed = true;
        const noTestsYet = tests.length === 0;
        const status = noTestsYet ? 'no tests yet' : failed > 0 ? 'FAILING' : 'passing';
        const score = noTestsYet ? '-' : `${passed}/${total}`;
        console.log(`${String(n).padEnd(6)} ${score.padEnd(8)} ${status}`);
      }

      const failingResults = results.filter((r) => r.failed > 0);
      if (failingResults.length > 0) {
        console.log('');
        for (const result of failingResults) {
          console.log(`Phase ${result.phase} failures:`);
          printTestLines(result.tests.filter((t) => t.status === 'fail'));
        }
      }

      exitCode = anyFailed ? 1 : 0;
    }
  } catch (err) {
    console.error(err instanceof Error ? err.message : String(err));
    exitCode = 1;
  } finally {
    rmSync(tmpDir, { recursive: true, force: true });
  }
  process.exit(exitCode);
}

main();
