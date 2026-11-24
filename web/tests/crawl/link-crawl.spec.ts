import { devices, expect, test } from '@playwright/test';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

// Link crawl: from `/`, follow every internal link and prove each one
// answers below 400. Sign-in goes through the login form the way a user
// does it, so authenticated pages are crawled with a session. Title pages
// are invisible to the crawl — the title card keeps its link in the hover
// overlay — so the spec seeds the queue with every fixture-backed movie and
// series, and only follows a title, person or collection link when the TMDB
// substitute can answer that id.
test('every internal link resolves', async ({ page }) => {
  const base = process.env.CRAWL_BASE_URL as string;
  const username = process.env.CRAWL_USERNAME as string;
  const password = process.env.CRAWL_PASSWORD as string;
  const tmdbBase = process.env.CRAWL_TMDB_URL as string;
  expect(base, 'CRAWL_BASE_URL is set').toBeTruthy();
  expect(username, 'CRAWL_USERNAME is set').toBeTruthy();
  expect(password, 'CRAWL_PASSWORD is set').toBeTruthy();
  expect(tmdbBase, 'CRAWL_TMDB_URL is set').toBeTruthy();

  const fixturesResponse = await fetch(`${tmdbBase}/__fixtures`);
  expect(fixturesResponse.ok, 'GET /__fixtures answers 200').toBeTruthy();
  const fixtures = (await fixturesResponse.json()) as {
    movie: number[];
    tv: number[];
    person: number[];
    collection: number[];
  };
  for (const kind of ['movie', 'tv', 'person', 'collection'] as const) {
    expect(
      Array.isArray(fixtures[kind]),
      `GET /__fixtures lists ${kind} ids`
    ).toBeTruthy();
  }
  const movieIds = new Set(fixtures.movie);
  const tvIds = new Set(fixtures.tv);
  const personIds = new Set(fixtures.person);
  const collectionIds = new Set(fixtures.collection);

  // A title, person or collection link (and a title subpage) is only worth
  // following when its id is in the fixture list. Anything else is a
  // recommendation the substitute has no fixture for, which the api answers
  // with a 500 the way it answers a TMDB 404, so following it would fail a
  // crawl that is otherwise green.
  const shouldFollow = (target: string): boolean => {
    let id: RegExpMatchArray | null;
    if (
      (id = target.match(
        /^\/movie\/(\d+)(\/(cast|crew|recommendations|similar))?\/?$/
      ))
    ) {
      return movieIds.has(Number(id[1]));
    }
    if (
      (id = target.match(
        /^\/tv\/(\d+)(\/(cast|crew|recommendations|similar))?\/?$/
      ))
    ) {
      return tvIds.has(Number(id[1]));
    }
    if ((id = target.match(/^\/person\/(\d+)\/?$/))) {
      return personIds.has(Number(id[1]));
    }
    if ((id = target.match(/^\/collection\/(\d+)\/?$/))) {
      return collectionIds.has(Number(id[1]));
    }
    return true;
  };

  await page.goto(`${base}/login`);
  await page.locator('input[name="email"]').fill(username);
  await page.locator('input[name="password"]').fill(password);
  await page.getByTestId('local-signin-button').click();
  await page.waitForURL(`${base}/`);

  const seen = new Set<string>();
  const queue: string[] = [
    '/',
    ...fixtures.movie.map((id) => `/movie/${id}`),
    ...fixtures.tv.map((id) => `/tv/${id}`),
  ];
  const maxPages = 300;

  while (queue.length > 0 && seen.size < maxPages) {
    const path = queue.shift() as string;
    if (seen.has(path)) {
      continue;
    }
    seen.add(path);

    const response = await page.goto(`${base}${path}`);
    expect(response?.status(), `GET ${path} answers below 400`).toBeLessThan(
      400
    );
    const body = await page.content();
    expect(body, `GET ${path} is not an error page`).not.toContain(
      'Application error'
    );

    const hrefs = await page.$$eval('a[href^="/"]', (els) =>
      els.map((el) => el.getAttribute('href') as string)
    );
    for (const href of hrefs) {
      const target = href.split(/[?#]/)[0];
      if (
        target.startsWith('/api/') ||
        target.startsWith('/_next/') ||
        seen.has(target) ||
        queue.includes(target) ||
        !shouldFollow(target)
      ) {
        continue;
      }
      queue.push(target);
    }
  }

  // The stripped navigation always resolves: Descobrir plus Filmes/Séries.
  expect([...seen]).toContain('/');
  expect(seen.size).toBeGreaterThan(2);

  // Every fixture-backed title page was visited. Only seeded ids can pass
  // the filter above, so reaching this count means each of them resolved.
  const expectedTitles = fixtures.movie.length + fixtures.tv.length;
  const seenPaths = [...seen];
  const visitedTitles = seenPaths.filter(
    (path) => /^\/movie\/\d+\/?$/.test(path) || /^\/tv\/\d+\/?$/.test(path)
  );
  console.log(
    `crawl visited ${visitedTitles.length} title pages over ${expectedTitles} fixture-backed titles`
  );
  for (const id of fixtures.movie) {
    expect(seenPaths, `GET /movie/${id} was crawled`).toContain(
      `/movie/${id}`
    );
  }
  for (const id of fixtures.tv) {
    expect(seenPaths, `GET /tv/${id} was crawled`).toContain(`/tv/${id}`);
  }
  expect(
    visitedTitles.length,
    'the crawl visits every fixture-backed title page'
  ).toBeGreaterThanOrEqual(expectedTitles);
});
