import { devices, expect, test } from '@playwright/test';
import { api, base, signIn } from '../library/session';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

// Cold start against the fixture substitute: only two seeds have recorded
// recommendations, so two ratings still answer the trending list, flagged
// popular. Personal output is pinned by the api tests and the renderer
// fixtures instead.
test('for-you shows the cold-start carousel, logs one exposure per card, and drops watched titles', async ({
  page,
}) => {
  await signIn(page);

  // Seed the run through the real library routes, never by writing SQL.
  await api(page, 'PUT', '/api/v1/library/rating', {
    mediaType: 'movie',
    tmdbId: 603,
    scope: 'movie',
    level: 'loved',
  });
  await api(page, 'PUT', '/api/v1/library/rating', {
    mediaType: 'tv',
    tmdbId: 249042,
    scope: 'show',
    level: 'liked',
  });

  const exposures: { runId: number; ranks: number[] }[] = [];
  page.on('request', (request) => {
    if (
      request.url().endsWith('/api/v1/recommendations/exposures') &&
      request.method() === 'POST'
    ) {
      exposures.push(request.postDataJSON());
    }
  });

  await page.goto(`${base()}/`);
  const slider = page.getByTestId('for-you-slider');
  await expect(slider).toBeVisible();
  await expect(page.getByTestId('for-you-cold').first()).toContainText(
    'Em alta'
  );
  // Cold start names no seed.
  await expect(slider.getByText(/Porque voc/)).toHaveCount(0);

  const cards = slider.locator('[data-testid^="recs-exposure-"]');
  const count = await cards.count();
  expect(count).toBeGreaterThan(0);
  const runIds = new Set<string>();
  for (let i = 0; i < count; i++) {
    await cards.nth(i).scrollIntoViewIfNeeded();
    const runId = await cards.nth(i).getAttribute('data-run-id');
    runIds.add(runId as string);
  }
  await expect.poll(() => exposures.length, { timeout: 10000 }).toBe(count);
  // One run, one post per rank, each body shaped as the api documents.
  expect(runIds.size).toBe(1);
  const [runId] = [...runIds].map(Number);
  const ranks = exposures.map((exposure) => exposure.ranks).flat();
  expect(new Set(ranks).size).toBe(count);
  for (const exposure of exposures) {
    expect(exposure.runId).toBe(runId);
    expect(exposure.ranks).toHaveLength(1);
  }

  // Scrolling again records nothing: one exposure per card per run.
  await cards.nth(0).scrollIntoViewIfNeeded();
  await page.waitForTimeout(1000);
  expect(exposures).toHaveLength(count);

  // The paged page shows the same cold run.
  await page.goto(`${base()}/discover/for-you`);
  await expect(page.getByTestId('for-you-cold')).toContainText('Em alta');
  await expect(page.getByTestId('for-you-grid')).toBeVisible();

  // Watching a recommended title recomputes the run without it.
  const firstCard = page
    .getByTestId('for-you-grid')
    .locator('[data-testid="title-card"]')
    .first();
  const watchedId = await firstCard.getAttribute('data-tmdb-id');
  const watchedMedia = await firstCard.getAttribute('data-media-type');
  expect(watchedId).toBeTruthy();
  await api(page, 'PUT', '/api/v1/library/watch', {
    mediaType: watchedMedia,
    tmdbId: Number(watchedId),
    scope: watchedMedia === 'movie' ? 'movie' : 'show',
  });
  await page.reload();
  await expect(page.getByTestId('for-you-grid')).toBeVisible();
  await expect(
    page
      .getByTestId('for-you-grid')
      .locator(`[data-testid="title-card"][data-tmdb-id="${watchedId}"]`)
  ).toHaveCount(0);
});
