// Records the TMDB fixtures that are not on disk yet, by running each case
// against the live TMDB API and saving what upstream asks for. Run it once,
// by hand, with a developer key in TMDB_API_KEY; CI never runs it.
//
//   TMDB_API_KEY=... node record.mjs [--upstream <dir>]
//
// Only cases made entirely of recorded fixtures are run. Within a case, each
// request no existing fixture answers is fetched live and saved under the next
// unrecorded fixture id in the case's list, so a case lists its fixtures in
// the order upstream first asks for them. The key is sent as the api_key query
// parameter and is never written: a fixture holds the request path and its
// parameters without api_key, the response status and the response body, and
// no header.
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { parseArgs } from 'node:util';
import { CASES } from './cases.mjs';
import { serializeJson } from './lib/golden.mjs';
import { fixtureFile, isDerived, openUpstreamRunner, readRecordedFixture } from './lib/store.mjs';

const TMDB_API = 'https://api.themoviedb.org/3';

const apiKey = process.env.TMDB_API_KEY;
if (!apiKey) {
  console.error('record.mjs: TMDB_API_KEY is not set; nothing was recorded');
  process.exit(2);
}

const redact = (text) => String(text).split(apiKey).join('<redacted>');

const fetchLive = async (request) => {
  const url = new URL(`${TMDB_API}${request.path}`);
  for (const [key, value] of Object.entries(request.params)) {
    url.searchParams.set(key, value);
  }
  url.searchParams.set('api_key', apiKey);
  let response;
  try {
    response = await fetch(url, { signal: AbortSignal.timeout(30000) });
  } catch (e) {
    throw new Error(`TMDB ${request.path}: ${redact(e.message)}`);
  }
  // A throttled or failing TMDB is not a response worth keeping.
  if (response.status === 429 || response.status >= 500) {
    throw new Error(`TMDB ${request.path} answered ${response.status}; try again later`);
  }
  return { status: response.status, body: await response.json() };
};

const saveFixture = (fixture) => {
  const file = fixtureFile(fixture.id);
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, serializeJson(fixture));
};

const recordCase = async (run, entry, missingIds) => {
  const pending = [...missingIds];
  const known = entry.fixtures.map(readRecordedFixture).filter(Boolean);
  const onUnmatched = async (request) => {
    const id = pending.shift();
    if (!id) {
      return null;
    }
    const fixture = { id, request, response: await fetchLive(request) };
    saveFixture(fixture);
    console.log(`recorded ${id}: ${request.path} -> ${fixture.response.status}`);
    return fixture;
  };
  const outcome = await run(entry.url, known, { onUnmatched });
  if (outcome.unmatched.length > 0) {
    throw new Error(
      `case ${entry.name} asked for more than its fixtures: ${JSON.stringify(outcome.unmatched)}`
    );
  }
  if (pending.length > 0) {
    throw new Error(`case ${entry.name} never asked for: ${pending.join(', ')}`);
  }
};

const main = async () => {
  const { values } = parseArgs({ options: { upstream: { type: 'string' } } });
  const runner = await openUpstreamRunner(values.upstream);
  try {
    for (const entry of CASES) {
      if (entry.fixtures.some(isDerived)) {
        continue;
      }
      const missingIds = entry.fixtures.filter((id) => !readRecordedFixture(id));
      if (missingIds.length > 0) {
        await recordCase(runner.run, entry, missingIds);
      }
    }
  } finally {
    runner.close();
  }
};

main().catch((e) => {
  console.error(`record.mjs: ${redact(e.message)}`);
  process.exit(1);
});
