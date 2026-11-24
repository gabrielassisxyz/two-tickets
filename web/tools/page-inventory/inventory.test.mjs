// Every file under web/src/pages is classified in inventory.json, and
// inventory.json lists no file that does not exist. Plain Node, no
// dependencies. Run with: node --test web/tools/page-inventory/inventory.test.mjs
import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';
import { classifyPage, listPageFiles } from './generate.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const inventory = JSON.parse(readFileSync(join(here, 'inventory.json'), 'utf8'));
const byPath = new Map(inventory.map((entry) => [entry.path, entry]));

describe('page inventory', () => {
  it('classifies every file under web/src/pages', () => {
    const files = listPageFiles();
    assert.ok(files.length > 0, 'expected page files under web/src/pages');
    for (const path of files) {
      const entry = byPath.get(path);
      assert.ok(entry !== undefined, `page file missing from inventory.json: ${path}`);
      const expected = classifyPage(path);
      assert.ok(expected !== null, `page file has no classifying rule: ${path}`);
      assert.equal(entry.class, expected.class, `class drift for ${path}: inventory says ${entry.class}`);
      assert.ok(
        typeof entry.reason === 'string' && entry.reason.length > 0,
        `empty reason for ${path}`,
      );
    }
  });

  it('lists no file that does not exist, and only known classes', () => {
    const files = new Set(listPageFiles());
    for (const entry of inventory) {
      assert.ok(files.has(entry.path), `inventory lists a file that does not exist: ${entry.path}`);
      assert.ok(
        ['keep', 'modify', 'drop'].includes(entry.class),
        `unknown class for ${entry.path}: ${entry.class}`,
      );
    }
  });
});
