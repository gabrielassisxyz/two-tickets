# two-tickets: agent briefing

> Read before every interaction. Living spec: short, imperative. On every gotcha or decision, append one line here.

> **What it is:** a movie and TV tracker for two people. It records what each person watched, down to the season, keeps ratings, reviews and lists, and recommends what to watch next, with the reason.
> **Calibration:** Tier 2 · Phase: work. High personal stakes, low external ones: two fixed users, nothing exposed publicly.
> **Review gate:** standard, the Tier 2 default. One independent external review of the whole branch diff, once, before the pull request goes up for merge.
> **Merging:** the owner merges pull requests. An agent opens them and stops there.

## Stack & Commands

- **Stack:** Go 1.26 at the root (`cmd/`, `internal/`) with SQLite, and a Next.js frontend under `web/` lifted from Seerr (MIT). Neither exists yet; the tracker holds the order in which they land.
- **Gate:** `bin/ci`. Today it runs gitleaks. The Go checks switch on when `go.mod` lands, and the web checks when `web/package.json` does.
- **Hooks:** run `bin/install-hooks` once after clone. It enables the gitleaks pre-commit hook and the commit-message attribution check.
- **Worktrees:** `bin/worktree new <type>/<kebab-desc>`, and all work happens there. The main tree stays on `main`.
- **Tracker:** `br` (beads). `br ready` lists open, unblocked work. The database is git-ignored, and `bin/worktree` links it into every worktree.

## Scope (current)

- **Current scope:** v0.1, which covers:
  - catalog browsing and search through TMDB;
  - watched state for movies, whole series and seasons;
  - a seven-step sentiment rating with reviews;
  - personal and shared lists;
  - a first "For you" list, each item explained by the titles that produced it;
  - two fixed accounts, with no sign-up.

  Don't expand beyond it without a present need; if a change drifts past it, STOP and flag it.

<!-- BEGIN universal-principles v3 -->
## Working principles

- **The human defines the WHAT; the agent decides the HOW.** Plan first for non-trivial
  tasks: show the plan and the to-do list, and wait for approval before building.
- **State assumptions; never pick between readings silently.** If several fit, present
  them. If a simpler approach exists, push back. If the task is impossible as stated, or
  information is missing, say so instead of guessing. This is a bias, not a ritual.
- **Surgical changes — touch only what you must.** Every changed line traces to the task.
  Don't "improve" adjacent code, reformat, or refactor what isn't broken; match existing style
  even if you'd do it differently. Flag unrelated dead code — don't delete it. Remove only the
  imports / variables / functions your own change orphaned.
- **Chesterton's Fence — find the problem before undoing the decision.** A config, a flag, a
  workaround that looks arbitrary is a **fence**: someone put it there, probably to fix
  something that is invisible to you *because the fence is working*. You arrive with no
  history, so absence of a visible reason is evidence of your ignorance, not of its
  uselessness. When your fresh measurement contradicts what the human vaguely remembers
  ("I changed this once, because of some problem"), **your measurement is the suspect first**
  — it may be measuring the case that *isn't* failing. Go find the original problem, then
  decide. *(A CIFS share was benchmarked with a big sequential `dd`, looked fast, and the
  local-disk download dir was "fixed" away — while the actual failure was random writes:
  par2, unrar, torrent piece-writes. Two wrong commits.)*
- **"Flaky" is not a diagnosis — test in the environment the thing actually runs in.** A
  component that fails *consistently* under automation is being **mis-invoked**, not being
  unreliable; "it works when I run it by hand" is not evidence that it works. The shell you
  test in has a TTY, a `$HOME`, an `ssh-agent`, an interactive stdin — the systemd unit, the
  CI job and the scripted harness have none of those, so a passing manual run can be testing
  a different program. Reproduce it *there* (start the unit, `env -u SSH_AUTH_SOCK`,
  `</dev/null`, `--dry-run` to print the real command line) before accepting "unstable" as a
  cause. **When a fix doesn't change the symptom, stop fixing and go look at what is actually
  being executed.** *(An interactive-mode flag with no TTY made one harness fail every review
  panel for weeks, written off as "flaky"; it was the wrong flag.)*
- **KISS and YAGNI — don't build for a need that doesn't exist.** No preventive
  abstractions, no single-use interfaces. Architecture is *extracted* once a pattern proves
  itself in real use, never designed up front. Most over-engineering is doing "right" or
  "fast" before a working thing justifies it. When a solo tool starts being framed as a
  public / multi-tenant / plugin-system / configurable-N-backends platform with no present
  need, STOP and ask — that is a standing duty, not a suggestion.
- **Development cost is not your cost — don't let it pick the design.** Choosing between
  technical options, weight quality, simplicity, robustness and long-term maintainability;
  don't weight how long the work takes. The estimate comes out in human units — days,
  weeks — because that is what the training data measured, and the cheaper option then
  wins on a cost the agent does not pay. This is **not** licence to over-build: KISS and
  YAGNI decide *whether* a thing is needed, and this decides *how well* it is built once
  it is. "That would take a week" is not an argument here; "nothing needs this yet" is.
- **No silent decisions (comprehension debt).** Never make a silent architectural or
  design call — state it and record the rationale, so the reasoning is recoverable later.
- **Real decisions go in the chat, on their own — never via a popup.** Surface the options,
  what each means, the trade-offs, and a recommendation; then decide together. Don't bury
  it mid-text or bundle it with other topics, and don't compress it into a quick-pick widget
  (e.g. AskUserQuestion) — the widget overlays the reasoning instead of showing it. Widgets
  are for trivial short-answer picks only.

## Git: branches, commits, PRs, comments

- **Ask the repo for its default branch; never assume one.** Repos differ — `master` and `main`
  are both common, often in the same person's account — and a wrong guess sends a PR to a branch
  that does not exist, or, worse, has you "fixing" a URL that was right all along.
  `git symbolic-ref --short refs/remotes/origin/HEAD | sed 's|^origin/||'`, or
  `gh repo view --json defaultBranchRef -q .defaultBranchRef.name`.
  Never commit directly to it: branch, then PR.
- **A new repo starts on `main`.** That is the preferred name, and `init.defaultBranch` is
  set to it, so `git init` produces it without anyone choosing. It settles new repos only:
  an existing one keeps the branch it has, because renaming breaks open PRs, CI filters,
  deploy hooks and every permalink into the tree, and buys nothing. The rule above still
  governs everything already in existence — ask, never assume.
- **Branches** — Conventional Branch (conventionalbranch.org): `<type>/<kebab-description>`,
  types `feature/`, `bugfix/`, `hotfix/`, `chore/`, `release/`, `docs/`.
- **Commits** — Conventional Commits (conventionalcommits.org): `<type>(scope): <description>`,
  types `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`, `perf`, `style`.
  Breaking change → `!` after the type or a `BREAKING CHANGE:` footer.
- **Atomic commits** — one logical change per commit, each independently green and
  revertible. Never `git add .` blind; split unrelated changes.
- **Always work in your own worktree — mandatory, not conditional.** Parallel sessions
  are opened freely and nothing signals their existence to you, so a "check whether another
  session is here first" step can never be reliable — the honest answer is always "maybe".
  The only collision-proof arrangement is structural: keep the main working tree on the
  default branch as a clean reference and **never work in it** — before your first write
  (commit, branch, rebase, stash; read-only exploration is exempt), create your own worktree
  and do everything there. **When the repo ships a worktree tool, that tool is the only correct
  way to make one** — `bin/worktree new <type>/<kebab-desc>`, or whatever the repo calls it.
  A raw `git worktree add` materialises tracked files and nothing else, so every git-ignored
  path the repo links into a worktree is silently absent, and the tools that read those paths
  fail inside it without saying why: a tracker whose database is git-ignored reports itself
  uninitialised and offers to create a second one. Only where the repo ships no such tool is
  `git worktree add ../<repo>-<task> -b <your-branch> <origin>/<default-branch>` the right
  command. Do this **whether or not** you believe another agent is running — that belief is
  exactly what you cannot verify. Report which worktree/branch you used; remove it once merged.
  Only the human can see all the open sessions.
- **Pull requests** — describe **what + why**. *What*: a 1–3 line summary. *Why* (the bulk):
  decisions, trade-offs, rejected alternatives. The diff shows the what; the PR explains why.
- **Comments** — always **WHY, not WHAT**: explain intent, never restate the obvious
  mechanics. Keep existing comments; they carry intent.

## Code style (baseline)

- Functions: 4–40 lines, one thing each (SRP). Files: under ~500 lines, split by responsibility.
- Names specific and unique — avoid `data`, `handler`, `Manager`, `util`.
- Explicit types. Early returns over nested ifs; max ~2 levels of indentation.
- Inject dependencies; wrap third-party libs behind a thin interface this project owns.
- No duplication — but don't extract *too early*. Tolerate duplication while the pattern is
  still forming; extract the abstraction *from* proven, repeated code, never ahead of it.
- **Refactoring is not automatic.** After a large feature, list the candidates and ask
  before pruning; consolidate once the thing works and the seams are obvious, not before.
<!-- END universal-principles v3 -->

## Tests (TDD)

- Every feature is born with a test; every bugfix with a regression test.
- Tests run with ONE command, no manual setup, no secret credential. If it can't run headless, it's wrong.
- TMDB is never called from tests: recorded responses and a fake server stand in for it.
- Every new guard is broken on purpose once before it is trusted. The change records the break, the failure it produced and the passing run.
- Before saying "done", run the tests and `bin/ci`; report the result.

## Small releases

- Every commit on `main` passes `bin/ci` and is production-ready.
- Closed work is committed before switching tasks; flag it if it has not been.

## Security (habit, not a phase)

- When touching user input, network, filesystem, auth, or queries, flag the risk and propose the guard (SSRF, rate limiting, path traversal, injection, session handling).
- Dependency CVEs are caught by `govulncheck` and `npm audit` in `bin/ci` once those stacks land.
- The TMDB key and the account secrets come from the environment; only `.env.example` is committed. Seerr's client ships a hard-coded TMDB key: never copy it.

## Git & secrets

- Before any commit, show `git status` + `git diff --cached`; confirm no secret is staged. If you spot one, STOP and report it. The gitleaks pre-commit hook is the deterministic backstop.
- Nothing host-specific is committed: no names, no addresses, no paths from one machine. Deployment specifics live in the GitOps repository that deploys the app.

## Post-implementation checklist (run before "done")

1. Commits small and well-described.
2. Refactoring candidates listed (if the change was large).
3. Security risks flagged (if you touched a sensitive surface).
4. This spec updated if behavior, setup, or release flow changed, and any hurdle it gained is classified, not just appended.

## Common hurdles

| hurdle | class | gate |
|---|---|---|
| A raw `git worktree add` has no `.beads`, so `br` in that worktree reports no tracker and offers to create one | tripwire | `bin/worktree` links it; never use a raw `git worktree add` here |
| Hosted Actions minutes run out on this private repository; the check then dies within seconds with "the job was not started" | prose | none until the self-hosted runner lands |
| Seerr's own `package.json` builds and starts its Express server, so importing `web/` unmodified switches on the web section of `bin/ci` with scripts that do not fit | prose | the import lands together with web-only scripts, or `bin/ci` changes in the same commit |

**A hurdle promoted to a gate is deleted from this table, not duplicated.**
