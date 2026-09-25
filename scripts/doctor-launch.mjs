#!/usr/bin/env node
// Runs the right doctor script for this OS and propagates its exit code.
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));

let result;
if (process.platform === 'win32') {
  result = spawnSync(
    'powershell',
    ['-ExecutionPolicy', 'Bypass', '-File', path.join(scriptsDir, 'doctor.ps1')],
    { stdio: 'inherit' },
  );
} else {
  result = spawnSync('sh', [path.join(scriptsDir, 'doctor.sh')], { stdio: 'inherit' });
}

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);
