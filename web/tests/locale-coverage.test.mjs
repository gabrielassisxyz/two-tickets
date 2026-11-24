// pt-BR coverage over the messages the kept pages actually use: every
// message key passed to `formatMessage` in a kept source file must be
// present and non-empty in pt-BR. `loadLocaleData` drops empty values so
// the English default renders instead of a blank label; this test proves
// there is nothing left to drop among the messages the kept pages use.
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
      } else if (/\.(ts|tsx)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name)) {
        found.push(full);
      }
    }
  };
  walk(srcRoot);
  return found;
};

const scopePattern = /defineMessages\(\s*['"]([^'"]+)['"]\s*,/g;
const usagePattern = /formatMessage\(\s*messages\.([A-Za-z0-9_]+)/g;

const usedMessages = () => {
  const keys = new Set();
  for (const full of listSourceFiles()) {
    const content = readFileSync(full, 'utf8');
    const scopes = [...content.matchAll(scopePattern)].map((match) => match[1]);
    if (scopes.length === 0) {
      continue;
    }
    const scope = scopes[0];
    for (const usage of content.matchAll(usagePattern)) {
      keys.add(`${scope}.${usage[1]}`);
    }
    // The shared catalogue is always scoped `i18n`, whatever file uses it.
    for (const usage of content.matchAll(
      /formatMessage\(\s*globalMessages\.([A-Za-z0-9_]+)/g
    )) {
      keys.add(`i18n.${usage[1]}`);
    }
  }
  return [...keys].sort();
};

describe('pt-BR coverage', () => {
  it('every message a kept page uses is present and non-empty in pt-BR', () => {
    const ptBR = JSON.parse(
      readFileSync(join(srcRoot, 'i18n', 'locale', 'pt_BR.json'), 'utf8')
    );
    const missing = [];
    const empty = [];
    for (const key of usedMessages()) {
      if (!(key in ptBR)) {
        missing.push(key);
      } else if (ptBR[key] === '') {
        empty.push(key);
      }
    }
    assert.deepEqual(missing, [], 'messages missing from pt-BR');
    assert.deepEqual(empty, [], 'messages empty in pt-BR');
  });
});
