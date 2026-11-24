# two-tickets

A movie and TV tracker for two people. It records what each person watched, down to the season, keeps their ratings, reviews and lists, and recommends what to watch next with the reason: "because you liked X, Y, Z".

**Status:** in development. Nothing runs yet.

## Planned shape

- A Go backend with SQLite.
- A web frontend lifted from [Seerr](https://github.com/seerr-team/seerr) (MIT). Its licence notice will travel with the code taken from it.
- Catalog data from [TMDB](https://www.themoviedb.org/). This product uses the TMDB API but is not endorsed or certified by TMDB.

## Development

- `bin/install-hooks` once after clone.
- `bin/ci` is the gate CI runs.
- `bin/worktree new <type>/<kebab-desc>` before changing anything.
