import { expect, type Page } from '@playwright/test';

// The stack deploy/web-crawl.sh starts: the production web image in front
// of the real api, whose TMDB is the fixture substitute. These specs import
// nothing but @playwright/test, which the script installs inside the
// Playwright image, so nothing resolves from the host's web/node_modules.
export const base = () => {
  const value = process.env.CRAWL_BASE_URL as string;
  expect(value, 'CRAWL_BASE_URL is set').toBeTruthy();
  return value;
};

export const signIn = async (page: Page) => {
  const username = process.env.CRAWL_USERNAME as string;
  const password = process.env.CRAWL_PASSWORD as string;
  expect(username, 'CRAWL_USERNAME is set').toBeTruthy();
  expect(password, 'CRAWL_PASSWORD is set').toBeTruthy();
  await page.goto(`${base()}/login`);
  await page.locator('input[name="email"]').fill(username);
  await page.locator('input[name="password"]').fill(password);
  await page.getByTestId('local-signin-button').tap();
  await page.waitForURL(`${base()}/`);
};

// Calls the api through web's proxy with the browser's own session. Every
// mutation carries the Origin the api's origin check allows.
export const api = async (
  page: Page,
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  path: string,
  data?: unknown
) => {
  const response = await page.request.fetch(`${base()}${path}`, {
    method,
    data,
    headers: { Origin: base() },
  });
  expect(response.status(), `${method} ${path} answers 2xx`).toBeLessThan(300);
  return response.status() === 204 ? undefined : response.json();
};

export const listIdOf = async (
  page: Page,
  kind: 'watchlist' | 'couple'
): Promise<number> => {
  const { results } = (await api(page, 'GET', '/api/v1/library/lists')) as {
    results: { id: number; kind: string; owner: string }[];
  };
  const list = results.find(
    (entry) =>
      entry.kind === kind && (kind === 'couple' || entry.owner === 'me')
  );
  expect(list, `the ${kind} list exists`).toBeTruthy();
  return (list as { id: number }).id;
};

export const setOnWatchlist = async (
  page: Page,
  mediaType: 'movie' | 'tv',
  tmdbId: number,
  onWatchlist: boolean
) => {
  if (onWatchlist) {
    await api(page, 'POST', '/api/v1/watchlist', {
      mediaType,
      tmdbId,
      title: `${mediaType} ${tmdbId}`,
    });
  } else {
    await api(
      page,
      'DELETE',
      `/api/v1/watchlist/${tmdbId}?mediaType=${mediaType}`
    );
  }
};

// The restore toast the optimistic hook shows, in pt-BR as the app renders.
export const restoreToast = (page: Page) =>
  page.getByText('Não foi possível salvar. O valor anterior foi restaurado.');

// Answers every request matching `pattern` and `method` with a 500 and
// counts them, so a test proves the request was made and then refused.
export const refuse = async (page: Page, pattern: string, method: string) => {
  const refused = { count: 0 };
  await page.route(pattern, async (route) => {
    if (route.request().method() !== method) {
      await route.fallback();
      return;
    }
    refused.count += 1;
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: '{"message":"refused by the test"}',
    });
  });
  return refused;
};
