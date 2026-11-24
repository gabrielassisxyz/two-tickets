// Where fixtures and goldens live, how derived fixtures are computed, and how
// the bundled upstream runner is obtained.
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { CASES } from '../cases.mjs';
import { FIXTURES } from '../fixtures.mjs';
import { bundleHarness } from './bundle.mjs';
import { fetchUpstream } from './upstream.mjs';

export const TOOL_DIR = resolve(dirname(fileURLToPath(import.meta.url)), '..');
export const REPO_ROOT = resolve(TOOL_DIR, '..', '..', '..');
export const TMDB_DIR = join(REPO_ROOT, 'testdata', 'tmdb');
export const GOLDEN_DIR = join(REPO_ROOT, 'testdata', 'golden');
export const INDEX_FILE = join(TMDB_DIR, 'index.json');

export const fixtureFile = (id) => join(TMDB_DIR, `${id}.json`);
export const goldenFile = (entry) => join(GOLDEN_DIR, entry.family, `${entry.name}.json`);

export const isDerived = (id) => Boolean(FIXTURES.find((fixture) => fixture.id === id)?.derive);

// Catches the registry and the cases drifting apart before anything runs.
export const validateRegistry = () => {
  const problems = [];
  const ids = new Set();
  for (const fixture of FIXTURES) {
    if (ids.has(fixture.id)) {
      problems.push(`fixture ${fixture.id} is registered twice`);
    }
    ids.add(fixture.id);
  }
  const names = new Set();
  const used = new Set();
  for (const entry of CASES) {
    if (names.has(entry.name)) {
      problems.push(`case ${entry.name} is defined twice`);
    }
    names.add(entry.name);
    if (!['discover', 'title', 'search', 'lookup'].includes(entry.family)) {
      problems.push(`case ${entry.name} has no known route family`);
    }
    for (const id of entry.fixtures) {
      used.add(id);
      if (!ids.has(id)) {
        problems.push(`case ${entry.name} uses unregistered fixture ${id}`);
      }
    }
  }
  for (const fixture of FIXTURES) {
    if (!used.has(fixture.id)) {
      problems.push(`fixture ${fixture.id} is used by no case`);
    }
    for (const base of [fixture.derive?.from, ...(fixture.derive?.with ?? [])]) {
      if (base && !ids.has(base)) {
        problems.push(`fixture ${fixture.id} derives from unregistered ${base}`);
      }
    }
  }
  return problems;
};

export const readRecordedFixture = (id) => {
  const file = fixtureFile(id);
  if (!existsSync(file)) {
    return null;
  }
  return JSON.parse(readFileSync(file, 'utf8'));
};

const deriveFixture = (fixture, byId) => {
  const { from, with: extras = [], request, response, body } = fixture.derive;
  const base = byId.get(from);
  const inputs = extras.map((id) => structuredClone(byId.get(id).response.body));
  const derived = {
    id: fixture.id,
    request: request ? request(structuredClone(base.request)) : structuredClone(base.request),
    response: response
      ? structuredClone(response)
      : { status: base.response.status, body: body(structuredClone(base.response.body), inputs) },
  };
  return derived;
};

// Recorded fixtures come from disk; derived ones are computed, in registry
// order, from fixtures already known.
export const loadAllFixtures = () => {
  const byId = new Map();
  const missing = [];
  for (const fixture of FIXTURES.filter((entry) => !entry.derive)) {
    const recordedFixture = readRecordedFixture(fixture.id);
    if (recordedFixture) {
      byId.set(fixture.id, recordedFixture);
    } else {
      missing.push(fixture.id);
    }
  }
  if (missing.length > 0) {
    throw new Error(
      `recorded fixtures missing (record them with record.mjs): ${missing.join(', ')}`
    );
  }
  for (const fixture of FIXTURES.filter((entry) => entry.derive)) {
    byId.set(fixture.id, deriveFixture(fixture, byId));
  }
  return byId;
};

const resolveUpstreamDir = (explicitDir) => {
  const dir = explicitDir || process.env.GOLDENS_UPSTREAM_DIR;
  if (dir) {
    return { dir: fetchUpstream(dir), cleanup: () => undefined };
  }
  const temporary = mkdtempSync(join(tmpdir(), 'seerr-upstream-'));
  return {
    dir: fetchUpstream(join(temporary, 'seerr')),
    cleanup: () => rmSync(temporary, { recursive: true, force: true }),
  };
};

// Fetches the pinned upstream commit, bundles it with the harness and returns
// the runner. The bundle and, unless a directory was given, the upstream
// checkout are disposable.
export const openUpstreamRunner = async (explicitDir) => {
  const upstream = resolveUpstreamDir(explicitDir);
  try {
    const outfile = join(TOOL_DIR, '.build', 'harness.cjs');
    await bundleHarness({ upstreamDir: upstream.dir, outfile });
    const require = createRequire(import.meta.url);
    delete require.cache[outfile];
    const harness = require(outfile);
    // The bundle carries upstream's source, hard-coded TMDB key included; once
    // loaded it is not kept in the tree.
    rmSync(outfile);
    return {
      run: harness.createRunner(join(upstream.dir, 'seerr-api.yml')),
      close: upstream.cleanup,
    };
  } catch (e) {
    upstream.cleanup();
    throw e;
  }
};
