// Classifies every file under web/src/pages as keep, modify or drop, and
// writes the result to inventory.json. Plain Node, no dependencies.
//
// A page with no matching rule is unclassified: generation fails instead of
// guessing, and the test fails until the rules name the page's class.
import { readdirSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const pagesDir = join(here, '..', '..', 'src', 'pages');
const outFile = join(here, 'inventory.json');

export const KEEP = 'keep';
export const MODIFY = 'modify';
export const DROP = 'drop';

// Ordered rules: the first match wins. Each reason fits on one line.
const rules = [
  {
    match: (p) =>
      [
        '404.tsx',
        '_document.tsx',
        '_error.tsx',
        'index.tsx',
        'search.tsx',
      ].includes(p),
    cls: KEEP,
    reason: 'standalone catalogue page, kept as is',
  },
  {
    match: (p) => p.startsWith('discover/'),
    cls: KEEP,
    reason: 'discover catalogue browsing, kept as is',
  },
  {
    match: (p) => p.startsWith('person/'),
    cls: KEEP,
    reason: 'person title page, kept as is',
  },
  {
    match: (p) =>
      /^(movie\/\[movieId\]|tv\/\[tvId\])\/(cast|crew|recommendations|similar)\.tsx$/.test(
        p
      ),
    cls: KEEP,
    reason: 'title subpage, kept as is',
  },
  // Not an upstream page: the browser's /api/v1 calls reach the api process
  // through this route, so it stays.
  {
    match: (p) => p === 'api/v1/[...path].ts',
    cls: KEEP,
    reason: 'runtime proxy to the api process, added here',
  },
  // Not upstream pages: the library's own Lists and History pages.
  {
    match: (p) => p === 'lists.tsx' || p === 'history.tsx',
    cls: KEEP,
    reason: 'library page, added here',
  },
  // Not an upstream page: the credits page, added here.
  {
    match: (p) => p === 'about.tsx',
    cls: KEEP,
    reason: 'About page with credits, added here',
  },
  {
    match: (p) => p === '_app.tsx',
    cls: MODIFY,
    reason: 'app shell, modified for the two fixed accounts',
  },
  {
    match: (p) =>
      p === 'movie/[movieId]/index.tsx' || p === 'tv/[tvId]/index.tsx',
    cls: MODIFY,
    reason: 'title page, modified to call the local api',
  },
  {
    match: (p) => p.startsWith('collection/'),
    cls: MODIFY,
    reason: 'collection page, modified for local lists',
  },
  // A page under a kept section still drops when its route is not ported:
  // the Plex sign-in callback has no counterpart with two local accounts.
  {
    match: (p) => p.startsWith('login/plex/'),
    cls: DROP,
    reason: 'Plex sign-in callback, a route that is not ported',
  },
  {
    match: (p) => p.startsWith('login/'),
    cls: MODIFY,
    reason: 'login flow, modified for the two fixed accounts',
  },
  {
    match: (p) =>
      [
        'blocklist/',
        'issues/',
        'requests/',
        'resetpassword/',
        'settings/',
        'users/',
        'profile/',
      ].some((prefix) => p.startsWith(prefix)),
    cls: DROP,
    reason: 'request and user management, not ported',
  },
  {
    match: (p) => p === 'setup.tsx',
    cls: DROP,
    reason: 'first-run setup wizard, not ported',
  },
];

// A file no rule names belongs with its parent's class: the top-level
// section decides. A file outside every known section is unclassified.
const parentClass = {
  discover: KEEP,
  person: KEEP,
  movie: MODIFY,
  tv: MODIFY,
  collection: MODIFY,
  login: MODIFY,
  blocklist: DROP,
  issues: DROP,
  requests: DROP,
  resetpassword: DROP,
  settings: DROP,
  users: DROP,
  profile: DROP,
  '404.tsx': KEEP,
  '_document.tsx': KEEP,
  '_error.tsx': KEEP,
  'index.tsx': KEEP,
  'search.tsx': KEEP,
  'lists.tsx': KEEP,
  'history.tsx': KEEP,
  '_app.tsx': MODIFY,
  'setup.tsx': DROP,
};

export function classifyPage(posixPath) {
  for (const rule of rules) {
    if (rule.match(posixPath)) {
      return { path: posixPath, class: rule.cls, reason: rule.reason };
    }
  }
  const top = posixPath.split('/')[0];
  if (parentClass[top] !== undefined) {
    return {
      path: posixPath,
      class: parentClass[top],
      reason: `under the ${top} section, follows its parent class`,
    };
  }
  return null;
}

// Every file under web/src/pages, as a posix-style path relative to it.
export function listPageFiles() {
  const found = [];
  const walk = (dir, prefix) => {
    for (const name of readdirSync(dir).sort()) {
      const full = join(dir, name);
      const rel = prefix === '' ? name : `${prefix}/${name}`;
      if (statSync(full).isDirectory()) {
        walk(full, rel);
      } else {
        found.push(rel);
      }
    }
  };
  walk(pagesDir, '');
  return found;
}

const isMain = process.argv[1] === fileURLToPath(import.meta.url);
if (isMain) {
  const entries = [];
  let failed = false;
  for (const path of listPageFiles()) {
    const entry = classifyPage(path);
    if (entry === null) {
      console.error(`unclassified page (no rule names it): ${path}`);
      failed = true;
    } else {
      entries.push(entry);
    }
  }
  if (failed) {
    console.error(
      'refused to write inventory.json: classify every page above first'
    );
    process.exit(1);
  }
  entries.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
  writeFileSync(outFile, `${JSON.stringify(entries, null, 2)}\n`);
  console.log(`wrote ${entries.length} entries to inventory.json`);
}
