// No `@server/` imports in web/src. The kept frontend talks to the api
// only through the `/api/v1` proxy route; a server-only import would tie a
// kept page to Seerr's server modules.
import { strict as assert } from 'node:assert';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const srcRoot = join(here, '..', 'src');

const listSourceFiles = () => {
  const found = [];
  const walk = (dir) => {
    for (const name of readdirSync(dir).sort()) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) {
        walk(full);
      } else if (/\.(ts|tsx)$/.test(name)) {
        found.push(full);
      }
    }
  };
  walk(srcRoot);
  return found;
};

describe('no server imports', () => {
  it("rg '@server/' web/src returns nothing", () => {
    const offenders = [];
    for (const full of listSourceFiles()) {
      if (readFileSync(full, 'utf8').includes('@server/')) {
        offenders.push(full);
      }
    }
    assert.deepEqual(offenders, []);
  });
});
