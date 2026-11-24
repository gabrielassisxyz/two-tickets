import { devices, expect, test } from '@playwright/test';
import { api, base, listIdOf, revealCardActions, signIn } from './session';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

test('log in, search, mark watched from a card, rate on the title page, and add a title to the couple list', async ({
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

  // The card's actions appear only once it is tapped, as upstream's hover
  // buttons did, and the card carries no rating.
  await expect(card.getByTestId('quick-actions')).toBeHidden();
  await revealCardActions(card);
  await expect(card.getByTestId('quick-rate')).toHaveCount(0);

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

  // Rate in seven steps on the title's page: one tap opens the picker, one
  // tap on a label rates.
  await page.goto(`${base()}/movie/603`);
  await page.getByTestId('title-rate').tap();
  const picker = page.getByTestId('rating-picker');
  await expect(picker.getByRole('radio')).toHaveCount(7);
  const rated = page.waitForRequest(
    (r) => r.url().endsWith('/api/v1/library/rating') && r.method() === 'PUT'
  );
  await picker.getByTestId('rating-level-liked').tap();
  const rating = await rated;
  expect(rating.postDataJSON()).toEqual({
    mediaType: 'movie',
    tmdbId: 603,
    scope: 'movie',
    level: 'liked',
  });
  expect((await rating.response())?.status()).toBe(200);
  await expect(page.getByTestId('title-rate')).toHaveAttribute(
    'aria-label',
    'Gostei'
  );

  // Add the same title to the couple list from its page.
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

  // Both pages show it: the couple list on Lists; the rating and the watch
  // in History.
  await page.goto(`${base()}/lists`);
  await expect(
    page
      .getByTestId('library-list-couple')
      .locator('[data-testid="title-card"][data-tmdb-id="603"]')
  ).toHaveCount(1);
  await page.goto(`${base()}/history`);
  const rated603 = page.locator(
    '[data-testid="history-row"][data-media-type="movie"][data-tmdb-id="603"]'
  );
  await expect(rated603.getByTestId('history-rating')).toHaveText('Gostei');
  const watched19840 = page.locator(
    '[data-testid="history-row"][data-media-type="movie"][data-tmdb-id="19840"]'
  );
  await expect(watched19840.getByTestId('history-watched')).toContainText(
    'Assistido em'
  );
});
