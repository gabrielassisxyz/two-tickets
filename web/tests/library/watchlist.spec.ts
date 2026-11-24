import {
  devices,
  expect,
  test,
  type Locator,
  type Page,
} from '@playwright/test';
import {
  api,
  base,
  listIdOf,
  refuse,
  restoreToast,
  revealCardActions,
  setOnWatchlist,
  signIn,
} from './session';

// Mobile viewport with touch input, on every run.
test.use({ ...devices['Pixel 7'] });

// A rejected add or remove restores the star's previous state and says so,
// on a card, on a slider and on both title pages. Each case sets the
// title's watchlist state through the api first, so no case depends on
// another having run.

const addPattern = '**/api/v1/watchlist';
const removePattern = '**/api/v1/watchlist/*';

const expectRestored = async (
  page: Page,
  control: Locator,
  refused: { count: number },
  pressed: 'true' | 'false'
) => {
  await expect(restoreToast(page).first()).toBeVisible();
  expect(refused.count, 'the request was sent and refused').toBe(1);
  await expect(control).toHaveAttribute('aria-pressed', pressed);
};

const searchCard = async (page: Page, tmdbId: number) => {
  await page.goto(`${base()}/search?query=cooper`);
  const card = page.locator(
    `[data-testid="title-card"][data-media-type="tv"][data-tmdb-id="${tmdbId}"]`
  );
  await expect(card).toHaveCount(1);
  return card;
};

test.beforeEach(async ({ page }) => {
  await signIn(page);
});

test('card: a rejected add restores the empty star', async ({ page }) => {
  await setOnWatchlist(page, 'tv', 4438, false);
  const card = await searchCard(page, 4438);
  await revealCardActions(card);
  const star = card.getByTestId('quick-watchlist');
  await expect(star).toHaveAttribute('aria-pressed', 'false');
  const refused = await refuse(page, addPattern, 'POST');
  await star.tap();
  await expectRestored(page, star, refused, 'false');
});

test('card: a rejected remove restores the filled star', async ({ page }) => {
  await setOnWatchlist(page, 'tv', 194999, true);
  const card = await searchCard(page, 194999);
  await revealCardActions(card);
  const star = card.getByTestId('quick-watchlist');
  await expect(star).toHaveAttribute('aria-pressed', 'true');
  const refused = await refuse(page, removePattern, 'DELETE');
  await star.tap();
  await expectRestored(page, star, refused, 'true');
});

test('slider: a rejected add on "Lista do casal" restores the empty star', async ({
  page,
}) => {
  const couple = await listIdOf(page, 'couple');
  await setOnWatchlist(page, 'movie', 11441, false);
  await api(page, 'PUT', `/api/v1/library/lists/${couple}/items`, {
    mediaType: 'movie',
    tmdbId: 11441,
  });
  await page.goto(`${base()}/`);
  const card = page
    .getByTestId('library-slider-couple')
    .locator(
      '[data-testid="title-card"][data-media-type="movie"][data-tmdb-id="11441"]'
    );
  await revealCardActions(card);
  const star = card.getByTestId('quick-watchlist');
  await expect(star).toHaveAttribute('aria-pressed', 'false');
  const refused = await refuse(page, addPattern, 'POST');
  await star.tap();
  await expectRestored(page, star, refused, 'false');
});

test('slider: a rejected remove on "Minha lista" keeps the card and its star', async ({
  page,
}) => {
  await setOnWatchlist(page, 'movie', 26564, true);
  await page.goto(`${base()}/`);
  const card = page
    .getByTestId('library-slider-watchlist')
    .locator(
      '[data-testid="title-card"][data-media-type="movie"][data-tmdb-id="26564"]'
    );
  await revealCardActions(card);
  const star = card.getByTestId('quick-watchlist');
  await expect(star).toHaveAttribute('aria-pressed', 'true');
  const refused = await refuse(page, removePattern, 'DELETE');
  await star.tap();
  await expectRestored(page, star, refused, 'true');
  await expect(card).toHaveCount(1);
});

test('movie page: a rejected add keeps "add"', async ({ page }) => {
  await setOnWatchlist(page, 'movie', 603, false);
  await page.goto(`${base()}/movie/603`);
  const add = page.getByTestId('title-watchlist-add');
  const refused = await refuse(page, addPattern, 'POST');
  await add.tap();
  await expect(restoreToast(page).first()).toBeVisible();
  expect(refused.count).toBe(1);
  await expect(add).toBeVisible();
  await expect(page.getByTestId('title-watchlist-remove')).toHaveCount(0);
});

test('movie page: a rejected remove keeps "remove"', async ({ page }) => {
  await setOnWatchlist(page, 'movie', 603, true);
  await page.goto(`${base()}/movie/603`);
  const remove = page.getByTestId('title-watchlist-remove');
  const refused = await refuse(page, removePattern, 'DELETE');
  await remove.tap();
  await expect(restoreToast(page).first()).toBeVisible();
  expect(refused.count).toBe(1);
  await expect(remove).toBeVisible();
  await expect(page.getByTestId('title-watchlist-add')).toHaveCount(0);
});

test('series page: a rejected add keeps "add"', async ({ page }) => {
  await setOnWatchlist(page, 'tv', 249042, false);
  await page.goto(`${base()}/tv/249042`);
  const add = page.getByTestId('title-watchlist-add');
  const refused = await refuse(page, addPattern, 'POST');
  await add.tap();
  await expect(restoreToast(page).first()).toBeVisible();
  expect(refused.count).toBe(1);
  await expect(add).toBeVisible();
  await expect(page.getByTestId('title-watchlist-remove')).toHaveCount(0);
});

test('series page: a rejected remove keeps "remove"', async ({ page }) => {
  await setOnWatchlist(page, 'tv', 249042, true);
  await page.goto(`${base()}/tv/249042`);
  const remove = page.getByTestId('title-watchlist-remove');
  const refused = await refuse(page, removePattern, 'DELETE');
  await remove.tap();
  await expect(restoreToast(page).first()).toBeVisible();
  expect(refused.count).toBe(1);
  await expect(remove).toBeVisible();
  await expect(page.getByTestId('title-watchlist-add')).toHaveCount(0);
});

test('a successful add and remove on a title page commit', async ({ page }) => {
  await setOnWatchlist(page, 'movie', 603, false);
  await page.goto(`${base()}/movie/603`);
  const added = page.waitForResponse(
    (r) =>
      r.url().endsWith('/api/v1/watchlist') && r.request().method() === 'POST'
  );
  await page.getByTestId('title-watchlist-add').tap();
  expect((await added).status()).toBe(201);
  const remove = page.getByTestId('title-watchlist-remove');
  await expect(remove).toBeEnabled();
  const removed = page.waitForResponse(
    (r) =>
      r.url().includes('/api/v1/watchlist/603') &&
      r.request().method() === 'DELETE'
  );
  await remove.tap();
  expect((await removed).status()).toBe(204);
  await expect(page.getByTestId('title-watchlist-add')).toBeVisible();
  await expect(restoreToast(page)).toHaveCount(0);
});
