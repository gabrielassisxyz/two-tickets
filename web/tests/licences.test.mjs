// Licence inventory: every retained asset and every declared dependency
// carries a licence, and the check fails on an unclassified one.
//
// web/tests/licences.json is the inventory. This test proves it complete
// (every file under web/public and web/src/assets, every dependency in
// package.json) and clean (every licence on the allowlist; share-alike
// and trademark entries need their justification note).
import { strict as assert } from 'node:assert';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = join(here, '..');

const inventory = JSON.parse(readFileSync(join(here, 'licences.json'), 'utf8'));

const permissive = new Set([
  'MIT',
  'MIT-0',
  'ISC',
  'Apache-2.0',
  'BSD-2-Clause',
  'BSD-3-Clause',
  'OFL-1.1',
  'CC0-1.0',
  'Unlicense',
]);

const justified = new Set(['CC-BY-SA-4.0', 'trademark']);

const listFiles = (dir, prefix) => {
  const found = [];
  for (const name of readdirSync(dir).sort()) {
    const full = join(dir, name);
    const rel = prefix === '' ? name : `${prefix}/${name}`;
    if (statSync(full).isDirectory()) {
      found.push(...listFiles(full, rel));
    } else {
      found.push(rel);
    }
  }
  return found;
};

describe('licence inventory', () => {
  it('lists every retained asset', () => {
    const onDisk = [
      ...listFiles(join(webRoot, 'public'), 'public'),
      ...listFiles(join(webRoot, 'src', 'assets'), 'src/assets'),
    ].sort();
    const listed = inventory.assets.map((entry) => entry.path).sort();
    assert.deepEqual(listed, onDisk);
  });

  it('lists every declared dependency with its installed licence', () => {
    const pkg = JSON.parse(readFileSync(join(webRoot, 'package.json'), 'utf8'));
    const declared = [
      ...Object.keys(pkg.dependencies || {}),
      ...Object.keys(pkg.devDependencies || {}),
    ].sort();
    const byName = new Map(
      inventory.dependencies.map((entry) => [entry.name, entry.licence])
    );
    assert.deepEqual([...byName.keys()].sort(), declared);
    for (const name of declared) {
      let installed;
      try {
        installed = JSON.parse(
          readFileSync(
            join(webRoot, 'node_modules', name, 'package.json'),
            'utf8'
          )
        );
      } catch {
        assert.fail(
          `dependency ${name} is not installed; run pnpm install first`
        );
      }
      const licence =
        typeof installed.license === 'object'
          ? installed.license.type
          : installed.license;
      assert.equal(
        byName.get(name),
        licence || 'UNKNOWN',
        `licence drift for ${name}`
      );
    }
  });

  it('fails on an unclassified licence', () => {
    for (const entry of [...inventory.assets, ...inventory.dependencies]) {
      const label = entry.path || entry.name;
      if (permissive.has(entry.licence)) {
        continue;
      }
      assert.ok(
        justified.has(entry.licence),
        `unclassified licence for ${label}: ${entry.licence}`
      );
      assert.ok(
        typeof entry.note === 'string' && entry.note.length > 0,
        `missing justification for ${label} (${entry.licence})`
      );
    }
  });
});
