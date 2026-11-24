// Navigation targets resolve to kept pages: every href in the Sidebar
// and MobileMenu menus must match a page the inventory keeps (keep or
// modify). A link to a dropped page fails here instead of 404ing in the
// crawl. Later beads add their entries together with their pages.
import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = join(here, '..');
const srcRoot = join(webRoot, 'src');

const inventory = JSON.parse(
  readFileSync(
    join(webRoot, 'tools', 'page-inventory', 'inventory.json'),
    'utf8'
  )
);

const hrefPattern = /href:\s*['"`](\/[^'"`]*?)['"`]/g;

// A page file becomes the route it answers: index.tsx files answer their
// directory, `[param]` segments match any one segment.
const pageRoute = (path) => {
  if (path === 'index.tsx') {
    return '/';
  }
  let route = `/${path.replace(/\/index\.tsx$/, '').replace(/\.tsx$/, '')}`;
  route = route.replace(/\[.*?\]/g, '{}');
  return route === '' ? '/' : route;
};

const keptRoutes = () =>
  inventory
    .filter((entry) => entry.class === 'keep' || entry.class === 'modify')
    .map((entry) => pageRoute(entry.path));

const menuHrefs = (file) => {
  const content = readFileSync(join(srcRoot, file), 'utf8');
  const hrefs = new Set();
  for (const match of content.matchAll(hrefPattern)) {
    hrefs.add(match[1]);
  }
  return [...hrefs].sort();
};

const routeMatches = (href, route) => {
  const a = href.split('/');
  const b = route.split('/');
  if (a.length !== b.length) {
    return false;
  }
  return a.every((segment, i) => b[i] === '{}' || segment === b[i]);
};

describe('navigation targets', () => {
  it('every menu href resolves to a kept page', () => {
    const routes = keptRoutes();
    assert.ok(routes.length > 0, 'the inventory keeps at least one page');
    for (const file of [
      'components/Layout/Sidebar/index.tsx',
      'components/Layout/MobileMenu/index.tsx',
    ]) {
      for (const href of menuHrefs(file)) {
        assert.ok(
          routes.some((route) => routeMatches(href, route)),
          `${file} links to ${href}, which no kept page serves`
        );
      }
    }
  });
});
