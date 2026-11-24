import { devices, expect, test } from '@playwright/test';
import { api, base, listIdOf, signIn } from './session';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

test('log in, search, rate from a card, and add a title to the couple list', async ({
  page,
}) => {
  await signIn(page);

  // Search the way a user does: type into the header field.
  const search = page.locator('input[type="search"]');
  await search.tap();
  await search.fill('cooper');
  await page.waitForURL(/\/search\?query=cooper/);
  const card = page.locator(
    '[data-testid="title-card"][data-media-type="movie"][data-tmdb-id="19840"]'
  );
  await expect(card.getByTestId('quick-actions')).toBeVisible();

  // Rate in seven steps from the card: one tap opens the picker in place,
  // one tap on a label rates.
  await card.getByTestId('quick-rate').tap();
  const picker = card.getByTestId('rating-picker');
  await expect(picker.getByRole('radio')).toHaveCount(7);
  const rated = page.waitForRequest(
    (r) => r.url().endsWith('/api/v1/library/rating') && r.method() === 'PUT'
  );
  await picker.getByTestId('rating-level-liked').tap();
  const rating = await rated;
  expect(rating.postDataJSON()).toEqual({
    mediaType: 'movie',
    tmdbId: 19840,
    scope: 'movie',
    level: 'liked',
  });
  expect((await rating.response())?.status()).toBe(200);
  await expect(card.getByTestId('quick-rate')).toHaveAttribute(
    'aria-label',
    'Gostei'
  );

  // Watched flips at once and sends exactly one PUT.
  const watches: string[] = [];
  page.on('request', (r) => {
    if (r.url().endsWith('/api/v1/library/watch')) {
      watches.push(r.method());
    }
  });
  const watched = card.getByTestId('quick-watched');
  await watched.tap();
  await expect(watched).toHaveAttribute('aria-pressed', 'true');
  await expect(watched).toHaveAttribute('aria-busy', 'false');
  expect(watches).toEqual(['PUT']);

  // Add a title to the couple list from its page.
  await page.goto(`${base()}/movie/603`);
  const couple = page.getByTestId('list-membership-couple');
  await expect(couple).toHaveAttribute('aria-pressed', 'false');
  const added = page.waitForResponse(
    (r) =>
      /\/api\/v1\/library\/lists\/\d+\/items$/.test(r.url()) &&
      r.request().method() === 'PUT'
  );
  await couple.tap();
  expect((await added).status()).toBeLessThan(300);
  await expect(couple).toHaveAttribute('aria-pressed', 'true');

  const coupleId = await listIdOf(page, 'couple');
  const items = (await api(
    page,
    'GET',
    `/api/v1/library/lists/${coupleId}/items`
  )) as { results: { mediaType: string; tmdbId: number }[] };
  expect(items.results).toContainEqual(
    expect.objectContaining({ mediaType: 'movie', tmdbId: 603 })
  );

  // Both pages show it: the couple list on Lists, the rating in History.
  await page.goto(`${base()}/lists`);
  await expect(
    page
      .getByTestId('library-list-couple')
      .locator('[data-testid="title-card"][data-tmdb-id="603"]')
  ).toHaveCount(1);
  await page.goto(`${base()}/history`);
  const row = page.locator(
    '[data-testid="history-row"][data-media-type="movie"][data-tmdb-id="19840"]'
  );
  await expect(row.getByTestId('history-rating')).toHaveText('Gostei');
  await expect(row.getByTestId('history-watched')).toContainText(
    'Assistido em'
  );
});
