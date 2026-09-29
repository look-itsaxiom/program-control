#!/usr/bin/env node
// check-citations.mjs
//
// Verifies every course-lesson citation in phases/**/*.md against the real
// lesson tree. A citation is any backticked token shaped like a lesson
// directory (`113-live-coding-drills`), a section inside one
// (`01-how-a-screen-is-graded`), or the two joined
// (`105-integrations-in-go/02-webhook-receiver`).
//
// A bare section token is resolved against the lesson named on the same line,
// falling back to the last lesson named earlier in the same file, which is how
// the primers and hints actually write them.
//
// Usage: node scripts/check-citations.mjs [pathToLessonsDir]
// Exits non-zero when any citation names a path that does not exist.

import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';

const repoRoot = resolve(import.meta.dirname, '..');
const lessonsDir = resolve(
  process.argv[2] ??
    process.env.LESSONS_DIR ??
    join(repoRoot, '..', 'ReactRefresher', 'src', 'content', 'lessons'),
);

if (!existsSync(lessonsDir)) {
  console.error(`lessons directory not found: ${lessonsDir}`);
  console.error('pass it as an argument or set LESSONS_DIR');
  process.exit(2);
}

const lessons = new Set(
  readdirSync(lessonsDir).filter((name) => statSync(join(lessonsDir, name)).isDirectory()),
);

/** True when `section` exists inside `lesson`, as either a concept file or an exercise folder. */
function sectionExists(lesson, section) {
  return (
    existsSync(join(lessonsDir, lesson, `${section}.md`)) ||
    existsSync(join(lessonsDir, lesson, section))
  );
}

function markdownFilesUnder(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...markdownFilesUnder(full));
    else if (entry.name.endsWith('.md')) out.push(full);
  }
  return out;
}

const TOKEN = /`([^`]+)`/g;
const CITATION = /^(\d{2,3}-[a-z0-9-]+)(?:\/(\d{2}-[a-z0-9-]+))?$/;

const files = markdownFilesUnder(join(repoRoot, 'phases')).sort();
const misses = [];
let checked = 0;

for (const file of files) {
  const lines = readFileSync(file, 'utf8').split('\n');
  let lastLesson = null;

  for (const [index, line] of lines.entries()) {
    const tokens = [...line.matchAll(TOKEN)].map((m) => m[1]);
    // A lesson named anywhere on this line wins over one from an earlier line.
    const lessonOnThisLine = tokens.find((t) => lessons.has(t)) ?? null;

    for (const token of tokens) {
      const match = CITATION.exec(token);
      if (!match) continue;
      const [, head, tail] = match;

      // `lesson/section`
      if (tail !== undefined) {
        checked++;
        if (!lessons.has(head)) {
          misses.push({ file, line: index + 1, token, why: `no lesson directory "${head}"` });
        } else if (!sectionExists(head, tail)) {
          misses.push({ file, line: index + 1, token, why: `"${head}" has no section "${tail}"` });
        }
        lastLesson = lessons.has(head) ? head : lastLesson;
        continue;
      }

      // A bare lesson directory.
      if (lessons.has(head)) {
        checked++;
        lastLesson = head;
        continue;
      }

      // A bare section, resolved against the lesson in scope.
      const scope = lessonOnThisLine ?? lastLesson;
      if (scope === null) continue; // not a citation we can attribute; ignore.
      checked++;
      if (!sectionExists(scope, head)) {
        misses.push({ file, line: index + 1, token, why: `"${scope}" has no section "${head}"` });
      }
    }
  }
}

for (const miss of misses) {
  const where = relative(repoRoot, miss.file).split(sep).join('/');
  console.log(`${where}:${miss.line}: \`${miss.token}\` -> ${miss.why}`);
}
console.log(
  `\nchecked ${checked} citations across ${files.length} files under phases/; ${misses.length} missing`,
);
process.exit(misses.length === 0 ? 0 : 1);
