// Tiny .env loader: no dependency, just enough for this project's needs.
import { readFileSync } from 'node:fs';

export function loadEnv(path = '.env') {
  const vars = {};
  let text;
  try {
    text = readFileSync(path, 'utf8');
  } catch {
    return vars;
  }
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq === -1) continue;
    const key = trimmed.slice(0, eq).trim();
    let value = trimmed.slice(eq + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    vars[key] = value;
  }
  return vars;
}

const DEFAULT_DATABASE_URL =
  'postgres://postgres:postgres@localhost:5433/program_control?sslmode=disable';

export function getDatabaseUrl() {
  const fromEnv = loadEnv();
  return process.env.DATABASE_URL || fromEnv.DATABASE_URL || DEFAULT_DATABASE_URL;
}

export function getEnv() {
  return { ...loadEnv(), ...process.env };
}
