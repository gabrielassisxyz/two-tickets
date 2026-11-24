# two-tickets

A movie and TV tracker for two people. It records what each person watched, down to the season, keeps their ratings, reviews and lists, and recommends what to watch next with the reason: "because you liked X, Y, Z".

**Status:** v0.1 is released and in use by its two people.

## What it does

- Browse and search the catalog, which comes from TMDB.
- Mark movies, whole series and single seasons as watched, per person.
- Rate on a seven-step scale, from hated to loved, and write a review.
- Keep personal lists and lists shared by the two people.
- A "For you" list, each item explained by the rated titles that produced it.
- Two fixed accounts, set in the environment; there is no sign-up.

## How it runs

Two processes, each shipped as its own image:

- `api`: the Go backend (`cmd/two-tickets`). It keeps everything in SQLite
  under `DATA_DIR` and calls TMDB for the catalog. On start, when the schema
  is behind, it takes a verified copy of `state.db` into `pre-migration/`
  and migrates; it listens only once the schema is current.
- `web`: the Next.js frontend in `web/`, lifted from
  [Seerr](https://github.com/seerr-team/seerr) (MIT). Its licence text is
  kept in `web/LICENSE.seerr`; see `web/UPSTREAM.md` for the pinned upstream
  commit. It renders pages and forwards `/api/v1` to the api at
  `API_INTERNAL_URL`, so the browser never talks to the api directly.

`.env.example` lists every variable both processes read. The api needs at
least `DATA_DIR`, a TMDB key, the two accounts and `ALLOWED_ORIGINS`.

This product uses the [TMDB](https://www.themoviedb.org/) API but is not
endorsed or certified by TMDB.

## Development

- `bin/install-hooks` once after clone.
- `bin/ci` is the gate CI runs.
- `bin/worktree new <type>/<kebab-desc>` before changing anything.

## Releases

Pushing a `v*` tag (for example `v0.1.0`) builds two images and pushes them to
GHCR as `ghcr.io/<owner>/two-tickets-api:<version>` and
`ghcr.io/<owner>/two-tickets-web:<version>`, where `<version>` is the tag
without its leading `v`. Tags are immutable: the workflow refuses a version
that already exists, so a bad release is superseded by a new tag, never by
overwriting. The packages are public, so the cluster pulls them with no
credential.

`two-tickets backup` copies the live `state.db` to `/data/backup/state.db`
(overridable with `--target`): it deletes the previous artifact, writes an
online copy with `VACUUM INTO`, checks it with `integrity_check` and
`foreign_key_check`, and renames it into place. It exits non-zero on any
failure and leaves nothing behind. A restore installs the artifact as
`state.db` and clears `sessions`, so both users log in again.
