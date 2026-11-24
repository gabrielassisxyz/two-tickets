# Goldens harness

The catalog routes of `api` must answer with the response shapes Seerr's own
server returns, or the lifted frontend renders blank pages. This harness pins
those shapes. It runs the mappers and route logic of the pinned Seerr commit
(`3f3fee5e6166a45599caa02f217e356a1b0719d3`, see `web/UPSTREAM.md`) over
recorded TMDB responses and writes what upstream answers as golden files, so
the Go port is checked against upstream behaviour rather than against a
reading of it.

| path | what |
| --- | --- |
| `testdata/tmdb/` | TMDB fixtures: one request and its response per file |
| `testdata/tmdb/index.json` | every fixture, its category, whether it was recorded or derived, and the route families and cases that use it |
| `testdata/golden/<family>/<case>.json` | one golden per case: the `/api/v1` request, the fixtures behind it, the status and the body |

Route families: `discover` (discover pages, sliders, trending, backdrops),
`title` (movie, series, season, person and collection pages), `search`, and
`lookup` (genres, languages, regions, studios, networks, keywords, watch
providers, certifications).

## Commands

Node 22 (or 24.22.1 and later), pnpm through corepack. From this directory:

```sh
corepack pnpm install --frozen-lockfile
node generate.mjs            # regenerate every golden from testdata/tmdb/
node generate.mjs --check    # write nothing; exit 1 if anything committed differs
node --test lib/golden.test.mjs
```

Generation makes no TMDB call and opens no socket. The only network access
is `git fetch` of the pinned Seerr commit into a temporary directory, which
fails the run if the commit cannot be fetched. `--upstream <dir>` (or
`GOLDENS_UPSTREAM_DIR`) reuses a checkout of that commit, and with one the
run works with no network at all. `bin/ci` runs `--check`.

## How a golden is produced

`lib/bundle.mjs` bundles upstream's `server/routes/index.ts` with esbuild.
The catalog modules (the TMDB client, `server/models/`, the discover, movie,
tv, person, collection and search routes, `server/lib/search.ts`) come from
the fetched commit unchanged. Everything a catalog response does not depend
on is replaced by a module under `stubs/`: the database (one signed-in user
whose locale is pt-BR, no title requested or watchlisted), the settings
(upstream's defaults with discover region `BR`), the logger, the cache (every
lookup misses), and the route families this project does not port. An
upstream import that is neither listed in `UPSTREAM_MODULES` nor stubbed
fails the build.

`lib/harness-entry.mjs` mounts the router as upstream's `server/index.ts`
does, request validation against `seerr-api.yml` included, and injects each
request in process. Upstream's TMDB client talks to an axios adapter
(`lib/fixture-adapter.mjs`) that answers from the case's fixtures by exact
path and query parameters, `api_key` aside. A request no fixture matches,
or a listed fixture that is never asked for, fails the run. The clock is
frozen at the recording date, because the upcoming sliders put today's date
in their requests.

## What a golden compares

A golden keeps the body upstream produced, except under `popularity`,
`voteCount` and `voteAverage`. TMDB changes those daily, so their values
are replaced by a type marker such as `"<number>"`. `compareToGolden` in
`lib/golden.mjs` is the comparison a port's tests apply. It checks field
names, presence (absent and `null` are different) and JSON types everywhere,
and values everywhere except under those three keys. Snake_case objects
nested in camelCase ones, such as movie `releases` and series
`contentRatings`, are compared exactly like everything else.

## Fixtures

`fixtures.mjs` registers every fixture. A recorded fixture is a real TMDB
response. A derived fixture is computed from recorded ones by the edit its
entry names, for what TMDB will not return on demand: adult entries, empty
records, result kinds an endpoint rarely mixes, and 429, 5xx and timeout
failures. Derived fixtures are committed, and `--check` fails when one no
longer matches its derivation.

Fixtures are recorded once, by hand, with a developer TMDB key:

```sh
TMDB_API_KEY=... node record.mjs
```

`record.mjs` records only fixtures that are not on disk yet. To re-record
one, delete its file first. The key goes out as the `api_key` query
parameter and is never written: a fixture holds the request path and its
parameters without `api_key`, the status and the body, and no header.

## Adding a case

Add it to `cases.mjs`, listing its fixtures in the order upstream asks for
them, and register any new fixture in `fixtures.mjs`. Record what is
missing, run `node generate.mjs`, and commit the fixtures, goldens and index
together.
