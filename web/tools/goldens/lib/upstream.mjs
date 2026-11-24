// Fetches the pinned Seerr commit into a directory outside the repository.
// The upstream server code is never committed here; it is fetched for each run.
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';

export const SEERR_REPOSITORY = 'https://github.com/seerr-team/seerr.git';
export const SEERR_COMMIT = '3f3fee5e6166a45599caa02f217e356a1b0719d3';

const git = (dir, args) =>
  execFileSync('git', ['-C', dir, ...args], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  }).trim();

const headOf = (dir) => {
  try {
    return git(dir, ['rev-parse', 'HEAD']);
  } catch {
    return '';
  }
};

// A directory already holding the pinned commit, unmodified, is reused as is;
// anything else at that path is refused rather than overwritten, since an
// edited upstream file would silently change every golden.
export const fetchUpstream = (dir) => {
  if (existsSync(join(dir, '.git'))) {
    const head = headOf(dir);
    if (head !== SEERR_COMMIT) {
      throw new Error(
        `${dir} holds commit ${head || '(none)'}, not the pinned ${SEERR_COMMIT}`
      );
    }
    if (git(dir, ['status', '--porcelain', '--untracked-files=no']) !== '') {
      throw new Error(`${dir} has local changes to the pinned commit`);
    }
    return dir;
  }
  mkdirSync(dir, { recursive: true });
  git(dir, ['init', '--quiet']);
  git(dir, ['remote', 'add', 'origin', SEERR_REPOSITORY]);
  try {
    git(dir, ['fetch', '--quiet', '--depth', '1', 'origin', SEERR_COMMIT]);
  } catch (e) {
    throw new Error(
      `cannot fetch Seerr commit ${SEERR_COMMIT} from ${SEERR_REPOSITORY}: ${e.stderr || e.message}`
    );
  }
  git(dir, ['checkout', '--quiet', 'FETCH_HEAD']);
  const head = headOf(dir);
  if (head !== SEERR_COMMIT) {
    throw new Error(`fetched ${head}, expected ${SEERR_COMMIT}`);
  }
  return dir;
};
