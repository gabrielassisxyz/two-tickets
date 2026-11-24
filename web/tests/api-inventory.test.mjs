// Static and runtime `/api/v1` inventory against the route manifest.
//
// The manifest (testdata/contract/api-routes.txt) is generated from the Go
// literals by internal/httpapi/route_manifest_test.go. This test diffs two
// more sets against it:
//
// - statically: every `/api/v1` literal in web/src, with `${...}` rewritten
//   to `{}` and query strings dropped;
// - at runtime: the request paths the renderer tests recorded in
//   web/tests/.runtime-requests.jsonl while every kept page and component
//   rendered its fixtures (run `corepack pnpm test` in web/ first).
//
// Two explicit gaps, each naming the bead that clears it:
// - declaredWithoutCaller: manifest paths no kept page calls yet;
// - calledButNotDeclared: frontend calls the api does not serve yet.
// The test fails on any difference outside the lists, and it fails when a
// listed entry is no longer needed, so the bead that closes a gap deletes
// its entry.
//
// Files holding `/api/v1` only as a prefix (the proxy and its test) are
// excluded by name, as are test files: fixtures must not pollute the
// inventory. No path is built by string concatenation in web/src; every
// dynamic segment is a `${...}` inside one literal.
import { strict as assert } from 'node:assert';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = join(here, '..');
const srcRoot = join(webRoot, 'src');
const manifestFile = join(
  webRoot,
  '..',
  'testdata',
  'contract',
  'api-routes.txt'
);
const runtimeLog = join(here, '.runtime-requests.jsonl');

// Manifest paths no kept page calls yet. The Discover page uses a fixed
// slider manifest instead of the server-stored customization, and the custom
// keyword sliders went with the edit UI, so neither route has a caller.
const declaredWithoutCaller = [
  {
    path: '/api/v1/settings/discover',
    bead: 'tt-2gv (no bead re-adds server-driven sliders; entry persists while the server keeps the route)',
  },
  {
    path: '/api/v1/discover/keyword/{}/movies',
    bead: 'tt-2gv (custom keyword sliders were removed with Discover customization; entry persists while the server keeps the route)',
  },
];

// Frontend calls the api does not serve yet. The library api bead serves
// both watchlist feeds.
const calledButNotDeclared = [
  { path: '/api/v1/watchlist', bead: 'tt-72w' },
  { path: '/api/v1/watchlist/{}', bead: 'tt-72w' },
  { path: '/api/v1/discover/watchlist', bead: 'tt-72w' },
];

const excludedFiles = new Set(['utils/apiProxy.ts', 'utils/apiProxy.test.ts']);

const isExcluded = (rel) =>
  excludedFiles.has(rel) ||
  rel.endsWith('.test.ts') ||
  rel.endsWith('.test.tsx');

const listSourceFiles = () => {
  const found = [];
  const walk = (dir, prefix) => {
    for (const name of readdirSync(dir).sort()) {
      const full = join(dir, name);
      const rel = prefix === '' ? name : `${prefix}/${name}`;
      if (statSync(full).isDirectory()) {
        walk(full, rel);
      } else if (/\.(ts|tsx)$/.test(name) && !isExcluded(rel)) {
        found.push({ rel, full });
      }
    }
  };
  walk(srcRoot, '');
  return found;
};

// Every single-, double- or backtick-quoted literal containing /api/v1/,
// collected by a small tokenizer that understands comments, escapes and
// one level of `${...}` nesting, so a closing backtick is never mistaken
// for the next literal's opening one and an apostrophe inside a comment
// never starts a literal.
const extractLiterals = (content) => {
  const literals = [];
  let i = 0;
  const n = content.length;
  while (i < n) {
    const two = content.slice(i, i + 2);
    if (two === '//') {
      const end = content.indexOf('\n', i);
      i = end === -1 ? n : end + 1;
      continue;
    }
    if (two === '/*') {
      const end = content.indexOf('*/', i + 2);
      i = end === -1 ? n : end + 2;
      continue;
    }
    const ch = content[i];
    if (ch !== "'" && ch !== '"' && ch !== '`') {
      i += 1;
      continue;
    }
    const prev = i === 0 ? '\n' : content[i - 1];
    if (/[A-Za-z0-9_]/.test(prev)) {
      i += 1;
      continue;
    }
    const quote = ch;
    let j = i + 1;
    let depth = 0;
    let closed = false;
    while (j < n) {
      const c = content[j];
      if (quote !== '`') {
        if (c === '\\') {
          j += 2;
          continue;
        }
        if (c === quote) {
          closed = true;
          break;
        }
        if (c === '\n') {
          break;
        }
        j += 1;
        continue;
      }
      if (c === '\\') {
        j += 2;
        continue;
      }
      if (depth === 0 && c === '`') {
        closed = true;
        break;
      }
      if (c === '$' && content[j + 1] === '{') {
        depth += 1;
        j += 2;
        continue;
      }
      if (c === '{') {
        if (depth > 0) {
          depth += 1;
        }
        j += 1;
        continue;
      }
      if (c === '}') {
        if (depth > 0) {
          depth -= 1;
        }
        j += 1;
        continue;
      }
      j += 1;
    }
    if (closed) {
      const literal = content.slice(i, j + 1);
      if (literal.includes('/api/v1/')) {
        literals.push(literal);
      }
      i = j + 1;
    } else {
      i += 1;
    }
  }
  return literals;
};

const normalizeStatic = (literal) => {
  let path = literal.slice(1, -1);
  const marker = path.indexOf('/api/v1/');
  path = path.slice(marker);
  // Drop the query string: the first `?` at interpolation depth zero. A
  // `?` inside `${...}` (a ternary) is part of the path expression.
  let depth = 0;
  let cut = -1;
  for (let i = 0; i < path.length; i++) {
    if (path[i] === '$' && path[i + 1] === '{') {
      depth += 1;
      i += 1;
    } else if (path[i] === '{' && depth > 0) {
      depth += 1;
    } else if (path[i] === '}' && depth > 0) {
      depth -= 1;
    } else if (path[i] === '?' && depth === 0) {
      cut = i;
      break;
    }
  }
  if (cut >= 0) {
    path = path.slice(0, cut);
  }
  let previous = '';
  while (previous !== path) {
    previous = path;
    path = path.replace(/\$\{(?:[^{}]|\{[^{}]*\})*\}/g, '{}');
  }
  return path;
};

const staticPaths = () => {
  const paths = new Set();
  for (const { rel, full } of listSourceFiles()) {
    const content = readFileSync(full, 'utf8');
    for (const literal of extractLiterals(content)) {
      const normalized = normalizeStatic(literal);
      if (normalized === '/api/v1/' || normalized.endsWith('/')) {
        continue;
      }
      if (!normalized.includes('/api/v1/')) {
        continue;
      }
      paths.add(normalized);
    }
  }
  return [...paths].sort();
};

const manifestPaths = () =>
  readFileSync(manifestFile, 'utf8')
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
    .sort();

const runtimePaths = () => {
  let raw;
  try {
    raw = readFileSync(runtimeLog, 'utf8');
  } catch {
    assert.fail(
      `missing ${runtimeLog}: run the renderer tests first (corepack pnpm test in web/)`
    );
  }
  const paths = new Set();
  for (const line of raw.split('\n')) {
    if (line.trim() === '') {
      continue;
    }
    const { url } = JSON.parse(line);
    const path = String(url).split('?')[0];
    if (path === '/api/v1/' || path.endsWith('/')) {
      continue;
    }
    paths.add(path);
  }
  return [...paths].sort();
};

// A static `{}` (a `${...}` interpolation) matches any one concrete
// segment, but a manifest `{}` never stands in for a static concrete
// segment: the static side must name the route it calls. Runtime paths are
// concrete, so a manifest or gap `{}` matches any one of their segments.
const matchesStatic = (candidate, template) => {
  const a = candidate.split('/');
  const b = template.split('/');
  if (a.length !== b.length) {
    return false;
  }
  return a.every((segment, i) => segment === b[i] || segment === '{}');
};

const matchesRuntime = (candidate, template) => {
  const a = candidate.split('/');
  const b = template.split('/');
  if (a.length !== b.length) {
    return false;
  }
  return a.every((segment, i) => segment === b[i] || b[i] === '{}');
};

const checkAgainstManifest = (name, matches, paths) => {
  const manifest = manifestPaths();
  const declared = declaredWithoutCaller.map((entry) => entry.path);
  const called = calledButNotDeclared.map((entry) => entry.path);

  for (const path of paths) {
    const known =
      manifest.some((entry) => matches(path, entry)) ||
      called.some((entry) => matches(path, entry));
    if (!known) {
      assert.fail(
        `${name} path ${path} is neither in the manifest nor a called-but-not-declared gap`
      );
    }
  }

  for (const entry of manifest) {
    const covered = paths.some((path) => matches(path, entry));
    if (declared.includes(entry)) {
      assert.ok(
        !covered,
        `${name} now covers ${entry}: the declared-without-caller entry is no longer needed, delete it`
      );
      continue;
    }
    assert.ok(covered, `${name} is missing manifest route ${entry}`);
  }

  for (const entry of called) {
    assert.ok(
      paths.some((path) => matches(path, entry)),
      `${name} no longer calls ${entry}: the called-but-not-declared entry is no longer needed, delete it`
    );
  }
};

describe('api inventory', () => {
  it('static literals equal the manifest plus the gaps', () => {
    checkAgainstManifest('static', matchesStatic, staticPaths());
  });

  it('runtime requests equal the manifest plus the gaps', () => {
    checkAgainstManifest('runtime', matchesRuntime, runtimePaths());
  });
});
