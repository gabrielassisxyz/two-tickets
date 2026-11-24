# two-tickets

A movie and TV tracker for two people. It records what each person watched, down to the season, keeps their ratings, reviews and lists, and recommends what to watch next with the reason: "because you liked X, Y, Z".

**Status:** in development. Nothing runs yet.

## Planned shape

- A Go backend with SQLite.
- A web frontend lifted from [Seerr](https://github.com/seerr-team/seerr) (MIT). Its licence text is kept in `web/LICENSE.seerr`; see `web/UPSTREAM.md` for the pinned upstream commit.
- Catalog data from [TMDB](https://www.themoviedb.org/). This product uses the TMDB API but is not endorsed or certified by TMDB.

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
