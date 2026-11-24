import { devices, expect, test } from '@playwright/test';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

// Link crawl: from `/`, follow every internal link and prove each one
// answers below 400. Sign-in goes through the login form the way a user
// does it, so authenticated pages are crawled with a session.
test('every internal link resolves', async ({ page }) => {
  const base = process.env.CRAWL_BASE_URL as string;
  const username = process.env.CRAWL_USERNAME as string;
  const password = process.env.CRAWL_PASSWORD as string;
  expect(base, 'CRAWL_BASE_URL is set').toBeTruthy();
  expect(username, 'CRAWL_USERNAME is set').toBeTruthy();
  expect(password, 'CRAWL_PASSWORD is set').toBeTruthy();

  await page.goto(`${base}/login`);
  await page.locator('input[name="email"]').fill(username);
  await page.locator('input[name="password"]').fill(password);
  await page.getByTestId('local-signin-button').click();
  await page.waitForURL(`${base}/`);

  const seen = new Set<string>();
  const queue: string[] = ['/'];
  const maxPages = 100;

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
        queue.includes(target)
      ) {
        continue;
      }
      queue.push(target);
    }
  }

  // The stripped navigation always resolves: Descobrir plus Filmes/Séries.
  expect([...seen]).toContain('/');
  expect(seen.size).toBeGreaterThan(2);
});
