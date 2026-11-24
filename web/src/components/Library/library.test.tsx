// @vitest-environment jsdom
// Component tests of the library UI against fake /api/v1/library/* and
// watchlist answers, including rejected ones (400, 403, 500). Every request
// is recorded with its method and body, so each test asserts on what was
// actually sent, not on what the component meant to send.
import type { PublicSettingsResponse } from '@app/compat/interfaces/api/settingsInterfaces';
import Discover from '@app/components/Discover';
import HistoryPage from '@app/components/Library/HistoryPage';
import LibraryListSlider from '@app/components/Library/LibraryListSlider';
import ListsPage from '@app/components/Library/ListsPage';
import SeasonRows from '@app/components/Library/SeasonRows';
import TitlePanel from '@app/components/Library/TitlePanel';
import MovieDetails from '@app/components/MovieDetails';
import TitleCard from '@app/components/TitleCard';
import TvDetails from '@app/components/TvDetails';
import { SettingsProvider } from '@app/context/SettingsContext';
import {
  cleanup,
  fireEvent,
  render,
  waitFor,
  within,
} from '@testing-library/react';
import axios from 'axios';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { IntlProvider } from 'react-intl';
import { SWRConfig } from 'swr';
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest';

const here = dirname(fileURLToPath(import.meta.url));

vi.mock('next/router', () => ({
  useRouter: () => ({
    pathname: '/',
    query: { movieId: '603', tvId: '1399' },
    replace: vi.fn(),
    push: vi.fn(),
  }),
}));
vi.mock('@app/assets/services/tmdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/imdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/tvdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/rt.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/trakt.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/simkl.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/letterboxd.svg', () => ({ default: () => null }));
vi.mock('@app/assets/spinner.svg', () => ({ default: () => null }));

const addToast = vi.fn();
vi.mock('@app/hooks/useToasts', () => ({
  default: () => ({ addToast, removeToast: vi.fn() }),
  useToasts: () => ({ addToast, removeToast: vi.fn() }),
}));

vi.mock('axios');
const mockedAxios = vi.mocked(axios, true);

const settingsFixture: PublicSettingsResponse = JSON.parse(
  readFileSync(
    join(
      here,
      '..',
      '..',
      '..',
      '..',
      'testdata',
      'contract',
      'settings-public.json'
    ),
    'utf8'
  )
);

interface SentRequest {
  method: string;
  path: string;
  query: string;
  data: unknown;
}

let sent: SentRequest[] = [];
// "METHOD /path" -> the status the fake api answers with instead of 2xx.
let rejections = new Map<string, number>();
// "METHOD /path" requests the test holds open to look at the in-flight UI.
let held = new Map<string, () => void>();

const person = (
  displayName: string,
  overrides: Record<string, unknown> = {}
) => ({
  displayName,
  status: 'unwatched',
  watched: false,
  watchedAt: null,
  rating: null,
  regularSeasonsWatched: 0,
  seasons: [],
  ...overrides,
});

let titleState: Record<string, unknown> = {};

const movieResult = (id: number, title: string) => ({
  id,
  mediaType: 'movie',
  title,
  originalTitle: title,
  overview: '',
  posterPath: null,
  releaseDate: '1999-03-31',
  voteAverage: 8,
  voteCount: 1,
  popularity: 1,
  genreIds: [],
  originalLanguage: 'en',
  adult: false,
  video: false,
});

const listItem = (id: number, title: string) => ({
  mediaType: 'movie',
  tmdbId: id,
  available: true,
  result: movieResult(id, title),
  addedAt: '2026-11-01T00:00:00.000Z',
  addedByMe: true,
  note: null,
});

const lists = [
  {
    id: 1,
    kind: 'watchlist',
    name: 'Watchlist',
    owner: 'me',
    itemCount: 1,
    createdAt: '',
    canRename: false,
    canChangeItems: true,
  },
  {
    id: 3,
    kind: 'custom',
    name: 'Terror',
    owner: 'me',
    itemCount: 1,
    createdAt: '',
    canRename: true,
    canChangeItems: true,
  },
  {
    id: 2,
    kind: 'couple',
    name: 'Couple',
    owner: 'shared',
    itemCount: 1,
    createdAt: '',
    canRename: false,
    canChangeItems: true,
  },
  {
    id: 4,
    kind: 'watchlist',
    name: 'Watchlist',
    owner: 'partner',
    itemCount: 1,
    createdAt: '',
    canRename: false,
    canChangeItems: false,
  },
  {
    id: 5,
    kind: 'custom',
    name: 'Do Bob',
    owner: 'partner',
    itemCount: 1,
    createdAt: '',
    canRename: false,
    canChangeItems: false,
  },
];

const historyRow = (n: number, overrides: Record<string, unknown> = {}) => ({
  mediaType: 'movie',
  tmdbId: 1000 + n,
  available: true,
  result: movieResult(1000 + n, `History ${n}`),
  scope: 'movie',
  seasonNumber: null,
  activityAt: '2026-11-01T00:00:00.000Z',
  watched: false,
  watchedAt: null,
  rating: null,
  ...overrides,
});

const historyAnswer = (query: URLSearchParams) => {
  if (query.get('level') === 'liked') {
    return {
      results: [
        historyRow(99, {
          watched: true,
          watchedAt: '2026-10-01T00:00:00.000Z',
          rating: { level: 'liked', review: 'Muito bom.', updatedAt: '' },
        }),
      ],
      nextCursor: null,
    };
  }
  if (query.get('cursor') === 'cursor-2') {
    return { results: [historyRow(20)], nextCursor: null };
  }
  return {
    results: Array.from({ length: 20 }, (_, n) =>
      historyRow(
        n,
        n === 0
          ? {
              watched: true,
              watchedAt: '2026-10-01T00:00:00.000Z',
              rating: { level: 'loved', review: null, updatedAt: '' },
            }
          : {}
      )
    ),
    nextCursor: 'cursor-2',
  };
};

const detailsFixture = (kind: 'movie' | 'tv', onUserWatchlist: boolean) => ({
  id: kind === 'movie' ? 603 : 1399,
  title: 'Fixture Movie',
  name: 'Fixture Series',
  overview: '',
  posterPath: null,
  backdropPath: null,
  genres: [],
  credits: { cast: [], crew: [] },
  seasons:
    kind === 'tv'
      ? [
          {
            id: 9,
            seasonNumber: 0,
            name: 'Specials',
            episodeCount: 2,
            overview: '',
            airDate: '2020-01-01',
          },
          {
            id: 10,
            seasonNumber: 1,
            name: 'Season 1',
            episodeCount: 8,
            overview: '',
            airDate: '2020-01-01',
          },
        ]
      : undefined,
  relatedVideos: [],
  mediaInfo: null,
  onUserWatchlist,
});

let onUserWatchlist = false;

const answerGet = (path: string, query: URLSearchParams) => {
  if (path === '/api/v1/auth/me') {
    return {
      id: 1,
      displayName: 'Alice',
      email: 'alice@example.com',
      permissions: 0,
      userType: 2,
    };
  }
  if (path.startsWith('/api/v1/library/title/')) {
    return titleState;
  }
  if (path === '/api/v1/library/lists') {
    return { results: lists };
  }
  const items = path.match(/^\/api\/v1\/library\/lists\/(\d+)\/items$/);
  if (items) {
    return {
      results: [listItem(600 + Number(items[1]), `List ${items[1]} title`)],
      nextCursor: null,
    };
  }
  if (path === '/api/v1/library/history') {
    return historyAnswer(query);
  }
  if (path === '/api/v1/settings/discover') {
    // 102 is the retired couple-list slider an older api still lists; the
    // page must not render it.
    return [101, 102].map((type, order) => ({
      id: type,
      type,
      order,
      isBuiltIn: true,
      enabled: true,
      title: null,
      data: null,
    }));
  }
  if (path === '/api/v1/movie/603') {
    return detailsFixture('movie', onUserWatchlist);
  }
  if (path === '/api/v1/tv/1399') {
    return detailsFixture('tv', onUserWatchlist);
  }
  return { page: 1, results: [], totalPages: 0, totalResults: 0 };
};

const rejection = (status: number) =>
  Object.assign(new Error(`status ${status}`), {
    isAxiosError: true,
    response: { status, data: { message: 'refused' } },
  });

const answerMutation = (method: string, url: string, data: unknown) => {
  const [path, query = ''] = url.split('?');
  sent.push({ method, path, query, data });
  const key = `${method} ${path}`;
  const status = rejections.get(key);
  const answer = () =>
    status === undefined
      ? Promise.resolve({ status: 200, data: {} })
      : Promise.reject(rejection(status));
  if (held.has(key) || held.has('*')) {
    return new Promise((resolve, reject) => {
      held.set(key, () => answer().then(resolve, reject));
    });
  }
  return answer();
};

const release = (key: string) => held.get(key)?.();

beforeAll(() => {
  class StubObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  vi.stubGlobal('IntersectionObserver', StubObserver);
  vi.stubGlobal('ResizeObserver', StubObserver);
  mockedAxios.get.mockImplementation(async (url: string) => {
    const [path, query = ''] = url.split('?');
    sent.push({ method: 'GET', path, query, data: undefined });
    return { data: answerGet(path, new URLSearchParams(query)) };
  });
  mockedAxios.put.mockImplementation((url: string, data?: unknown) =>
    answerMutation('PUT', url, data)
  );
  mockedAxios.post.mockImplementation((url: string, data?: unknown) =>
    answerMutation('POST', url, data)
  );
  mockedAxios.patch.mockImplementation((url: string, data?: unknown) =>
    answerMutation('PATCH', url, data)
  );
  mockedAxios.delete.mockImplementation(
    (url: string, config?: { data?: unknown }) =>
      answerMutation('DELETE', url, config?.data)
  );
});

beforeEach(() => {
  sent = [];
  rejections = new Map();
  held = new Map();
  onUserWatchlist = false;
  titleState = {
    mediaType: 'movie',
    tmdbId: 603,
    me: person('Alice'),
    partner: person('Bob'),
    lists: [],
  };
  addToast.mockReset();
});

afterEach(cleanup);

const renderWithApi = (node: React.ReactNode) =>
  render(
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
        fetcher: (url: string) => mockedAxios.get(url).then((r) => r.data),
      }}
    >
      <IntlProvider
        locale="en"
        defaultLocale="en"
        messages={{}}
        onError={() => undefined}
      >
        <SettingsProvider currentSettings={settingsFixture}>
          {node}
        </SettingsProvider>
      </IntlProvider>
    </SWRConfig>
  );

const mutations = (method: string, path: string) =>
  sent.filter((request) => request.method === method && request.path === path);

const pressed = (el: HTMLElement) => el.getAttribute('aria-pressed');

describe('title card quick actions', () => {
  it('shows watch later and watched only on hover or tap, and no rating', () => {
    const screen = renderWithApi(
      <TitleCard id={603} title="Fixture" mediaType="movie" image="/p.jpg" />
    );
    const actions = screen.getByTestId('quick-actions');
    expect(actions.getAttribute('data-visible')).toBe('false');
    expect(actions.className).toContain('invisible');

    // The poster itself, not the title link the hover reveals.
    const poster = screen.container.querySelector(
      'div[role="link"]'
    ) as HTMLElement;
    fireEvent.pointerEnter(poster, { pointerType: 'mouse' });
    expect(actions.getAttribute('data-visible')).toBe('true');
    expect(actions.className).not.toContain('invisible');
    expect(
      within(actions)
        .getAllByRole('button')
        .map((b) => b.getAttribute('data-testid'))
    ).toEqual(['quick-watchlist', 'quick-watched']);
    expect(screen.queryByTestId('quick-rate')).toBeNull();
    expect(screen.queryByTestId('rating-picker')).toBeNull();

    fireEvent.pointerLeave(poster, { pointerType: 'mouse' });
    expect(actions.getAttribute('data-visible')).toBe('false');

    // A tap reveals them; the mouse events a browser emulates for that tap
    // must not hide them again, and a tap elsewhere does.
    fireEvent.click(poster);
    expect(actions.getAttribute('data-visible')).toBe('true');
    fireEvent.pointerLeave(poster, { pointerType: 'touch' });
    fireEvent.mouseLeave(poster);
    expect(actions.getAttribute('data-visible')).toBe('true');
    fireEvent.pointerDown(poster, { pointerType: 'touch' });
    expect(actions.getAttribute('data-visible')).toBe('true');
    fireEvent.pointerDown(document.body, { pointerType: 'touch' });
    expect(actions.getAttribute('data-visible')).toBe('false');
  });

  it('flips watched at once and sends one PUT /library/watch', async () => {
    held.set('PUT /api/v1/library/watch', () => undefined);
    const screen = renderWithApi(
      <TitleCard id={603} title="Fixture" mediaType="movie" />
    );
    const watched = screen.getByTestId('quick-watched');
    expect(pressed(watched)).toBe('false');
    fireEvent.click(watched);
    expect(pressed(watched)).toBe('true');
    expect(mutations('PUT', '/api/v1/library/watch')).toHaveLength(1);
    expect(mutations('PUT', '/api/v1/library/watch')[0].data).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
    });
    release('PUT /api/v1/library/watch');
    await waitFor(() =>
      expect(watched.getAttribute('aria-busy')).toBe('false')
    );
    expect(pressed(watched)).toBe('true');
    expect(mutations('PUT', '/api/v1/library/watch')).toHaveLength(1);
    expect(addToast).not.toHaveBeenCalled();
  });

  it('reverts watched and shows a toast when the PUT is rejected', async () => {
    rejections.set('PUT /api/v1/library/watch', 500);
    const screen = renderWithApi(
      <TitleCard id={603} title="Fixture" mediaType="movie" />
    );
    const watched = screen.getByTestId('quick-watched');
    fireEvent.click(watched);
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(pressed(watched)).toBe('false');
    expect(addToast.mock.calls[0][1]).toMatchObject({ appearance: 'error' });
  });
});

describe('rating on the title page', () => {
  it('opens the seven-label picker and rates with one tap', async () => {
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    fireEvent.click(screen.getByTestId('title-rate'));
    const picker = screen.getByTestId('rating-picker');
    expect(within(picker).getAllByRole('radio')).toHaveLength(7);
    fireEvent.click(within(picker).getByTestId('rating-level-liked'));
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/rating')).toHaveLength(1)
    );
    expect(mutations('PUT', '/api/v1/library/rating')[0].data).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
      level: 'liked',
    });
    await waitFor(() =>
      expect(screen.getByTestId('title-rate').getAttribute('aria-busy')).toBe(
        'false'
      )
    );
    fireEvent.click(screen.getByTestId('title-rate'));
    fireEvent.click(screen.getByTestId('rating-clear'));
    await waitFor(() =>
      expect(mutations('DELETE', '/api/v1/library/rating')).toHaveLength(1)
    );
    expect(mutations('DELETE', '/api/v1/library/rating')[0].data).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
    });
  });

  it('restores the rating after a 400', async () => {
    rejections.set('PUT /api/v1/library/rating', 400);
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    fireEvent.click(screen.getByTestId('title-rate'));
    fireEvent.click(screen.getByTestId('rating-level-loved'));
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId('title-rate').getAttribute('aria-label')).toBe(
      'Rate'
    );
  });
});

describe('the watchlist star restores the previous state after a rejected request', () => {
  const rejectAdd = () => rejections.set('POST /api/v1/watchlist', 500);
  const rejectDelete = () =>
    rejections.set('DELETE /api/v1/watchlist/603', 500);

  it('on a card, for an add and for a delete', async () => {
    rejectAdd();
    const add = renderWithApi(
      <TitleCard id={603} title="Fixture" mediaType="movie" />
    );
    fireEvent.click(add.getByTestId('quick-watchlist'));
    expect(pressed(add.getByTestId('quick-watchlist'))).toBe('true');
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(pressed(add.getByTestId('quick-watchlist'))).toBe('false');
    add.unmount();

    rejectDelete();
    const remove = renderWithApi(
      <TitleCard
        id={603}
        title="Fixture"
        mediaType="movie"
        isAddedToWatchlist
      />
    );
    fireEvent.click(remove.getByTestId('quick-watchlist'));
    expect(pressed(remove.getByTestId('quick-watchlist'))).toBe('false');
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(2));
    expect(pressed(remove.getByTestId('quick-watchlist'))).toBe('true');
    expect(mutations('DELETE', '/api/v1/watchlist/603')[0].query).toBe(
      'mediaType=movie'
    );
  });

  it('on the "Minha lista" slider, for a delete, keeping the card on failure', async () => {
    rejections.set('DELETE /api/v1/watchlist/601', 500);
    const mine = renderWithApi(
      <LibraryListSlider
        kind="watchlist"
        title="Minha lista"
        sliderKey="mine"
      />
    );
    const myStar = await mine.findByTestId('quick-watchlist');
    expect(pressed(myStar)).toBe('true');
    fireEvent.click(myStar);
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(pressed(mine.getByTestId('quick-watchlist'))).toBe('true');
    expect(mine.getAllByTestId('title-card')).toHaveLength(1);
  });

  it('on the movie page, for an add and for a delete', async () => {
    rejectAdd();
    const add = renderWithApi(<MovieDetails />);
    fireEvent.click(await add.findByTestId('title-watchlist-add'));
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(add.getByTestId('title-watchlist-add')).toBeTruthy();
    expect(add.queryByTestId('title-watchlist-remove')).toBeNull();
    add.unmount();

    onUserWatchlist = true;
    rejectDelete();
    const remove = renderWithApi(<MovieDetails />);
    fireEvent.click(await remove.findByTestId('title-watchlist-remove'));
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(2));
    expect(remove.getByTestId('title-watchlist-remove')).toBeTruthy();
    expect(remove.queryByTestId('title-watchlist-add')).toBeNull();
  });

  it('on the series page, for an add and for a delete', async () => {
    rejections.set('POST /api/v1/watchlist', 500);
    const add = renderWithApi(<TvDetails />);
    fireEvent.click(await add.findByTestId('title-watchlist-add'));
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(add.getByTestId('title-watchlist-add')).toBeTruthy();
    expect(mutations('POST', '/api/v1/watchlist')[0].data).toMatchObject({
      tmdbId: 1399,
      mediaType: 'tv',
    });
    add.unmount();

    onUserWatchlist = true;
    rejections.set('DELETE /api/v1/watchlist/1399', 500);
    const remove = renderWithApi(<TvDetails />);
    fireEvent.click(await remove.findByTestId('title-watchlist-remove'));
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(2));
    expect(remove.getByTestId('title-watchlist-remove')).toBeTruthy();
  });

  it('commits a successful add and remove on the movie page', async () => {
    const screen = renderWithApi(<MovieDetails />);
    fireEvent.click(await screen.findByTestId('title-watchlist-add'));
    await screen.findByTestId('title-watchlist-remove');
    await waitFor(() =>
      expect(
        screen.getByTestId('title-watchlist-remove').getAttribute('aria-busy')
      ).toBe('false')
    );
    fireEvent.click(screen.getByTestId('title-watchlist-remove'));
    await screen.findByTestId('title-watchlist-add');
    expect(mutations('POST', '/api/v1/watchlist')).toHaveLength(1);
    expect(mutations('DELETE', '/api/v1/watchlist/603')).toHaveLength(1);
    expect(addToast).not.toHaveBeenCalled();
  });
});

describe('series page', () => {
  it('has a watched toggle and a rating per season row, season 0 included, and a whole-series control', async () => {
    titleState = { ...titleState, mediaType: 'tv', tmdbId: 1399 };
    const seasons = detailsFixture('tv', false).seasons ?? [];
    const screen = renderWithApi(
      <>
        <TitlePanel mediaType="tv" tmdbId={1399} />
        <SeasonRows tvId={1399} seasons={seasons} />
      </>
    );
    for (const number of [0, 1]) {
      expect(screen.getByTestId(`season-watched-${number}`)).toBeTruthy();
      expect(screen.getByTestId(`season-rate-${number}`)).toBeTruthy();
    }
    fireEvent.click(screen.getByTestId('season-watched-0'));
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/watch')).toHaveLength(1)
    );
    expect(mutations('PUT', '/api/v1/library/watch')[0].data).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'season',
      seasonNumber: 0,
    });

    fireEvent.click(screen.getByTestId('season-rate-1'));
    fireEvent.click(screen.getByTestId('rating-level-good'));
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/rating')).toHaveLength(1)
    );
    expect(mutations('PUT', '/api/v1/library/rating')[0].data).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'season',
      seasonNumber: 1,
      level: 'good',
    });

    fireEvent.click(screen.getByTestId('series-watched'));
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/watch')).toHaveLength(2)
    );
    expect(mutations('PUT', '/api/v1/library/watch')[1].data).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'show',
    });

    fireEvent.click(screen.getByTestId('series-rate'));
    fireEvent.click(screen.getByTestId('rating-level-meh'));
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/rating')).toHaveLength(2)
    );
    expect(mutations('PUT', '/api/v1/library/rating')[1].data).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'show',
      level: 'meh',
    });
  });

  it('the series page mounts the season rows for every season TMDB lists', async () => {
    const screen = renderWithApi(<TvDetails />);
    expect(await screen.findByTestId('season-row-0')).toBeTruthy();
    expect(screen.getByTestId('season-row-1')).toBeTruthy();
    expect(screen.getByTestId('title-panel')).toBeTruthy();
  });
});

describe('title panel', () => {
  beforeEach(() => {
    titleState = {
      ...titleState,
      partner: person('Bob', {
        watched: true,
        status: 'watched',
        rating: {
          level: 'disliked',
          review: 'Não curti o final.',
          updatedAt: '',
        },
      }),
    };
  });

  it("shows the partner's watched state, rating and review, read-only", async () => {
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    const partner = await screen.findByTestId('partner-state');
    expect(within(partner).getByText('Bob')).toBeTruthy();
    expect(within(partner).getByTestId('partner-watched').textContent).toBe(
      'Watched'
    );
    expect(within(partner).getByTestId('partner-rating').textContent).toBe(
      'Disliked · bad, would not recommend'
    );
    expect(within(partner).getByTestId('partner-review').textContent).toBe(
      'Não curti o final.'
    );
    expect(
      partner.querySelectorAll('button, input, textarea, select')
    ).toHaveLength(0);
  });

  it('saves a non-empty review of at most 10,000 characters through PUT /library/rating', async () => {
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    const text = screen.getByTestId('review-text');
    const save = () => screen.getByTestId('review-save') as HTMLButtonElement;

    fireEvent.change(text, { target: { value: '   ' } });
    expect(save().disabled).toBe(true);
    fireEvent.change(text, { target: { value: 'x'.repeat(10001) } });
    expect(save().disabled).toBe(true);
    fireEvent.submit(screen.getByTestId('review-editor'));
    expect(mutations('PUT', '/api/v1/library/rating')).toHaveLength(0);

    // Counted in characters: 10,000 emoji are 20,000 UTF-16 units and fit.
    fireEvent.change(text, { target: { value: '🎬'.repeat(10000) } });
    expect(save().disabled).toBe(false);
    fireEvent.change(text, { target: { value: 'x'.repeat(10000) } });
    expect(save().disabled).toBe(false);
    fireEvent.click(save());
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/rating')).toHaveLength(1)
    );
    expect(mutations('PUT', '/api/v1/library/rating')[0].data).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
      review: 'x'.repeat(10000),
    });
  });

  it('puts the title on the couple list, and restores it after a 403', async () => {
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    const couple = await screen.findByTestId('list-membership-couple');
    fireEvent.click(couple);
    await waitFor(() =>
      expect(mutations('PUT', '/api/v1/library/lists/2/items')).toHaveLength(1)
    );
    expect(mutations('PUT', '/api/v1/library/lists/2/items')[0].data).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
    });

    rejections.set('PUT /api/v1/library/lists/3/items', 403);
    const custom = screen.getByTestId('list-membership-custom');
    fireEvent.click(custom);
    await waitFor(() => expect(addToast).toHaveBeenCalledTimes(1));
    expect(pressed(screen.getByTestId('list-membership-custom'))).toBe('false');
  });

  it('offers no membership control for the watchlist or the partner lists', async () => {
    const screen = renderWithApi(<TitlePanel mediaType="movie" tmdbId={603} />);
    await screen.findByTestId('list-memberships');
    const ids = screen
      .getAllByRole('button')
      .map((el) => el.getAttribute('data-list-id'))
      .filter((id) => id !== null);
    expect(ids).toEqual(['3', '2']);
  });
});

describe('lists page', () => {
  it('lists mine, the couple list and the partner lists, read-only where the api says so', async () => {
    const screen = renderWithApi(<ListsPage />);
    const partner = await screen.findByTestId('list-group-partner');
    await waitFor(() =>
      expect(within(partner).getAllByTestId('list-item')).toHaveLength(2)
    );
    expect(within(partner).queryByTestId('list-item-remove')).toBeNull();
    expect(within(partner).queryByTestId('list-rename')).toBeNull();
    expect(within(partner).queryByTestId('list-delete')).toBeNull();
    expect(within(partner).queryByTestId('quick-actions')).toBeNull();

    const mine = screen.getByTestId('list-group-me');
    const watchlist = within(mine).getByTestId('library-list-watchlist');
    expect(within(watchlist).queryByTestId('list-rename')).toBeNull();
    expect(within(watchlist).queryByTestId('list-delete')).toBeNull();
    const couple = screen.getByTestId('library-list-couple');
    expect(within(couple).queryByTestId('list-rename')).toBeNull();
    expect(within(couple).queryByTestId('list-delete')).toBeNull();
    await waitFor(() =>
      expect(within(couple).getByTestId('list-item-remove')).toBeTruthy()
    );
    const custom = within(mine).getByTestId('library-list-custom');
    expect(within(custom).getByTestId('list-rename')).toBeTruthy();
    expect(within(custom).getByTestId('list-delete')).toBeTruthy();
  });

  it('creates, renames and deletes a custom list, and removes an item', async () => {
    const screen = renderWithApi(<ListsPage />);
    fireEvent.change(await screen.findByTestId('list-create-name'), {
      target: { value: 'Natal' },
    });
    fireEvent.submit(screen.getByTestId('list-create'));
    await waitFor(() =>
      expect(mutations('POST', '/api/v1/library/lists')).toHaveLength(1)
    );
    expect(mutations('POST', '/api/v1/library/lists')[0].data).toEqual({
      name: 'Natal',
    });

    const custom = within(screen.getByTestId('list-group-me')).getByTestId(
      'library-list-custom'
    );
    fireEvent.click(within(custom).getByTestId('list-rename'));
    fireEvent.change(within(custom).getByTestId('list-rename-input'), {
      target: { value: 'Terror bom' },
    });
    fireEvent.submit(
      within(custom)
        .getByTestId('list-rename-input')
        .closest('form') as HTMLFormElement
    );
    await waitFor(() =>
      expect(mutations('PATCH', '/api/v1/library/lists/3')).toHaveLength(1)
    );
    expect(mutations('PATCH', '/api/v1/library/lists/3')[0].data).toEqual({
      name: 'Terror bom',
    });

    const remove = await within(custom).findByTestId('list-item-remove');
    fireEvent.click(remove);
    await waitFor(() =>
      expect(
        mutations('DELETE', '/api/v1/library/lists/3/items/movie/603')
      ).toHaveLength(1)
    );

    const deleteButton = within(custom)
      .getByTestId('list-delete')
      .closest('button') as HTMLButtonElement;
    fireEvent.click(deleteButton);
    expect(mutations('DELETE', '/api/v1/library/lists/3')).toHaveLength(0);
    fireEvent.click(deleteButton);
    await waitFor(() =>
      expect(mutations('DELETE', '/api/v1/library/lists/3')).toHaveLength(1)
    );
  });
});

describe('history page', () => {
  it('shows a page of 20, then the next page by its cursor', async () => {
    const screen = renderWithApi(<HistoryPage />);
    await waitFor(() =>
      expect(screen.getAllByTestId('history-row')).toHaveLength(20)
    );
    fireEvent.click(screen.getByTestId('history-more'));
    await waitFor(() =>
      expect(screen.getAllByTestId('history-row')).toHaveLength(21)
    );
    const pages = sent.filter(
      (request) => request.path === '/api/v1/library/history'
    );
    expect(pages.map((request) => request.query)).toContain('cursor=cursor-2');
    expect(screen.queryByTestId('history-more')).toBeNull();
  });

  it('shows watched and rated state in one row', async () => {
    const screen = renderWithApi(<HistoryPage />);
    const rows = await screen.findAllByTestId('history-row');
    expect(within(rows[0]).getByTestId('history-watched').textContent).toMatch(
      /^Watched on /
    );
    expect(within(rows[0]).getByTestId('history-rating').textContent).toBe(
      'Loved'
    );
  });

  it('filters by level', async () => {
    const screen = renderWithApi(<HistoryPage />);
    await waitFor(() =>
      expect(screen.getAllByTestId('history-row')).toHaveLength(20)
    );
    fireEvent.change(screen.getByTestId('history-level'), {
      target: { value: 'liked' },
    });
    await waitFor(() =>
      expect(screen.getAllByTestId('history-row')).toHaveLength(1)
    );
    const pages = sent.filter(
      (request) => request.path === '/api/v1/library/history'
    );
    expect(pages.map((request) => request.query)).toContain('level=liked');
    expect(screen.getByText('Muito bom.')).toBeTruthy();
  });
});

describe('discover', () => {
  it('renders "Minha lista" and no "Lista do casal" slider', async () => {
    const screen = renderWithApi(<Discover />);
    expect(await screen.findByTestId('library-slider-watchlist')).toBeTruthy();
    expect(screen.queryByTestId('library-slider-couple')).toBeNull();
  });
});
