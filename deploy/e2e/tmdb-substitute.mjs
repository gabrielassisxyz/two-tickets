// A stand-in for TMDB, for the Playwright stack in deploy/web-crawl.sh. The
// api runs with TMDB_BASE_URL pointing here and gets the recorded fixtures
// under testdata/tmdb/ back, so Discover, search and the title pages render
// real catalog data with no call to TMDB and no key.
//
// A request is matched on its exact path and parameters first (api_key
// aside), then on its path alone, and otherwise answers TMDB's own 404 body.
// Where several fixtures fit, a recorded 200 wins over a derived or failing
// one, so a fixture made to test adult filtering or an error path is only
// served when nothing better matches.
import { readFileSync } from 'node:fs';
import http from 'node:http';
import { join } from 'node:path';

const root = process.env.TMDB_FIXTURES_DIR ?? '/e2e/tmdb';
const port = Number(process.env.TMDB_SUBSTITUTE_PORT ?? '8080');
// The api's base URL ends in TMDB's version segment, as the real one does.
const versionPrefix = '/3';

const notFound = JSON.stringify({
  success: false,
  status_code: 34,
  status_message: 'The resource you requested could not be found.',
});

const paramsKey = (params) =>
  JSON.stringify(
    Object.entries(params)
      .filter(([name]) => name !== 'api_key')
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
  );

const rank = (fixture) =>
  (fixture.status === 200 ? 0 : 2) + (fixture.source === 'recorded' ? 0 : 1);

const loadFixtures = () => {
  const index = JSON.parse(readFileSync(join(root, 'index.json'), 'utf8'));
  const fixtures = [];
  for (const entry of index.fixtures) {
    const file = join(root, entry.file.replace(/^testdata\/tmdb\//, ''));
    const recorded = JSON.parse(readFileSync(file, 'utf8'));
    if (!recorded.request || !recorded.response) {
      continue;
    }
    fixtures.push({
      path: recorded.request.path,
      params: paramsKey(recorded.request.params ?? {}),
      status: recorded.response.status,
      body: JSON.stringify(recorded.response.body),
      source: entry.source,
      file: entry.file,
    });
  }
  return fixtures.sort((a, b) => rank(a) - rank(b) || a.file.localeCompare(b.file));
};

const fixtures = loadFixtures();

const match = (path, params) =>
  fixtures.find((f) => f.path === path && f.params === params) ??
  fixtures.find((f) => f.path === path && f.status === 200);

http
  .createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://substitute');
    const path = url.pathname.startsWith(`${versionPrefix}/`)
      ? url.pathname.slice(versionPrefix.length)
      : url.pathname;
    const found = match(path, paramsKey(Object.fromEntries(url.searchParams)));
    res.setHeader('content-type', 'application/json;charset=utf-8');
    res.statusCode = found ? found.status : 404;
    res.end(found ? found.body : notFound);
    console.log(
      JSON.stringify({ path, status: res.statusCode, fixture: found?.file })
    );
  })
  .listen(port, () => {
    console.log(JSON.stringify({ listening: port, fixtures: fixtures.length }));
  });
