// @vitest-environment jsdom
// Renderer tests: every kept page and component renders against empty,
// minimal and full fixtures. Every axios call is recorded to
// web/tests/.runtime-requests.jsonl, which the api-inventory check diffs
// against the route manifest. Rendering a kept page must never request the
// dropped polling routes.
import { DiscoverSliderType } from '@app/compat/constants/discover';
import type { PublicSettingsResponse } from '@app/compat/interfaces/api/settingsInterfaces';
import { SliderId } from '@app/compat/sliderIds';
import About from '@app/components/About';
import CollectionDetails from '@app/components/CollectionDetails';
import CompanyTag from '@app/components/CompanyTag';
import Discover from '@app/components/Discover';
import DiscoverMovieGenre from '@app/components/Discover/DiscoverMovieGenre';
import DiscoverMovieKeyword from '@app/components/Discover/DiscoverMovieKeyword';
import DiscoverMovieLanguage from '@app/components/Discover/DiscoverMovieLanguage';
import DiscoverMovies from '@app/components/Discover/DiscoverMovies';
import DiscoverNetwork from '@app/components/Discover/DiscoverNetwork';
import DiscoverStudio from '@app/components/Discover/DiscoverStudio';
import DiscoverTv from '@app/components/Discover/DiscoverTv';
import DiscoverTvGenre from '@app/components/Discover/DiscoverTvGenre';
import DiscoverTvKeyword from '@app/components/Discover/DiscoverTvKeyword';
import DiscoverTvLanguage from '@app/components/Discover/DiscoverTvLanguage';
import DiscoverTvUpcoming from '@app/components/Discover/DiscoverTvUpcoming';
import DiscoverWatchlist from '@app/components/Discover/DiscoverWatchlist';
import FilterSlideover from '@app/components/Discover/FilterSlideover';
import MovieGenreList from '@app/components/Discover/MovieGenreList';
import Trending from '@app/components/Discover/Trending';
import TvGenreList from '@app/components/Discover/TvGenreList';
import Upcoming from '@app/components/Discover/Upcoming';
import ForYouPage from '@app/components/ForYou/ForYouPage';
import ForYouSlider from '@app/components/ForYou/ForYouSlider';
import GenreTag from '@app/components/GenreTag';
import KeywordTag from '@app/components/KeywordTag';
import LanguageSelector from '@app/components/LanguageSelector';
import UserDropdown from '@app/components/Layout/UserDropdown';
import HistoryPage from '@app/components/Library/HistoryPage';
import ListsPage from '@app/components/Library/ListsPage';
import TitlePanel from '@app/components/Library/TitlePanel';
import MovieDetails from '@app/components/MovieDetails';
import MovieCast from '@app/components/MovieDetails/MovieCast';
import MovieCrew from '@app/components/MovieDetails/MovieCrew';
import MovieRecommendations from '@app/components/MovieDetails/MovieRecommendations';
import MovieSimilar from '@app/components/MovieDetails/MovieSimilar';
import PersonDetails from '@app/components/PersonDetails';
import RegionSelector from '@app/components/RegionSelector';
import Search from '@app/components/Search';
import {
  CompanySelector,
  GenreSelector,
  KeywordSelector,
  WatchProviderSelector,
} from '@app/components/Selector';
import CertificationSelector from '@app/components/Selector/CertificationSelector';
import TitleCard from '@app/components/TitleCard';
import TmdbTitleCard from '@app/components/TitleCard/TmdbTitleCard';
import TvDetails from '@app/components/TvDetails';
import Season from '@app/components/TvDetails/Season';
import TvCast from '@app/components/TvDetails/TvCast';
import TvCrew from '@app/components/TvDetails/TvCrew';
import TvRecommendations from '@app/components/TvDetails/TvRecommendations';
import TvSimilar from '@app/components/TvDetails/TvSimilar';
import { SettingsProvider } from '@app/context/SettingsContext';
import LoginPage from '@app/pages/login';
import { fireEvent, render, waitFor, within } from '@testing-library/react';
import axios from 'axios';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import {
  appendFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
} from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { IntlProvider } from 'react-intl';
import { SWRConfig } from 'swr';
import { beforeAll, describe, expect, it, vi } from 'vitest';

const here = dirname(fileURLToPath(import.meta.url));
const runtimeLog = join(here, '..', '..', 'tests', '.runtime-requests.jsonl');

vi.mock('next/router', () => ({
  useRouter: () => ({
    pathname: '/',
    query: {
      query: 'fixture',
      movieId: '603',
      tvId: '1399',
      personId: '52583',
      collectionId: '2344',
      genreId: '28',
      language: 'pt',
      studioId: '2',
      networkId: '213',
      keywordId: '9715',
      keywords: '9715',
      seasonNumber: '1',
    },
    replace: vi.fn(),
    push: vi.fn(),
  }),
}));

vi.mock('next/dist/client/router', () => ({
  useRouter: () => ({
    pathname: '/',
    query: {
      movieId: '603',
      tvId: '1399',
      personId: '52583',
      collectionId: '2344',
    },
    replace: vi.fn(),
    push: vi.fn(),
  }),
}));

// SVG imports resolve to URLs under plain vite; stub them as components.
vi.mock('@app/assets/services/tmdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/imdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/tvdb.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/rt.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/trakt.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/simkl.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/letterboxd.svg', () => ({
  default: () => null,
}));
vi.mock('@app/assets/services/emby.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/jellyfin.svg', () => ({ default: () => null }));
vi.mock('@app/assets/services/plex.svg', () => ({ default: () => null }));
vi.mock('@app/assets/spinner.svg', () => ({ default: () => null }));
vi.mock('@app/assets/ellipsis.svg', () => ({ default: () => null }));

type Level = 'empty' | 'minimal' | 'full';

let level: Level = 'empty';

const settingsFixture: PublicSettingsResponse = JSON.parse(
  readFileSync(
    join(
      here,
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

const emptyPage = { page: 1, results: [], totalPages: 0, totalResults: 0 };

const movieFixture = (withCast: boolean) => ({
  id: 603,
  title: 'Fixture Movie',
  overview: 'An overview.',
  tagline: 'A tagline.',
  backdropPath: null,
  posterPath: null,
  imdbId: 'tt0133093',
  releaseDate: '1999-03-31',
  runtime: 136,
  genres: withCast ? [{ id: 28, name: 'Action' }] : [],
  productionCompanies: [],
  productionCountries: [],
  spokenLanguages: [],
  status: 'Released',
  voteAverage: 8,
  voteCount: 100,
  popularity: 10,
  adult: false,
  budget: 0,
  revenue: 0,
  video: false,
  originalTitle: 'Fixture Movie',
  originalLanguage: 'en',
  credits: {
    cast: withCast
      ? [
          {
            id: 1,
            name: 'Actor',
            character: 'Hero',
            profilePath: null,
            order: 0,
          },
        ]
      : [],
    crew: [],
  },
  releases: { results: [] },
  relatedVideos: [],
  collection: null,
  mediaInfo: null,
  onUserWatchlist: false,
  watchProviders: withCast
    ? [
        {
          iso_3166_1: 'BR',
          link: 'https://www.themoviedb.org/movie/603/watch?locale=BR',
          buy: [],
          flatrate: [
            {
              displayPriority: 1,
              logoPath: '/logo.png',
              id: 8,
              name: 'Netflix',
            },
          ],
        },
      ]
    : [],
});

const tvFixture = (withCast: boolean) => ({
  id: 1399,
  name: 'Fixture Series',
  overview: 'An overview.',
  tagline: '',
  backdropPath: null,
  posterPath: null,
  firstAirDate: '2008-01-20',
  lastAirDate: '2013-09-29',
  episodeRunTime: [45],
  genres: withCast ? [{ id: 18, name: 'Drama' }] : [],
  networks: [],
  createdBy: [],
  languages: ['en'],
  inProduction: false,
  homepage: '',
  contentRatings: { results: [] },
  voteAverage: 9,
  voteCount: 200,
  popularity: 20,
  credits: {
    cast: withCast
      ? [
          {
            id: 2,
            name: 'Actor',
            character: 'Lead',
            profilePath: null,
            order: 0,
          },
        ]
      : [],
    crew: [],
  },
  seasons: withCast
    ? [
        {
          id: 10,
          airDate: '2008-01-20',
          episodeCount: 7,
          name: 'Season 1',
          overview: '',
          posterPath: null,
          seasonNumber: 1,
        },
      ]
    : [],
  relatedVideos: [],
  mediaInfo: null,
  onUserWatchlist: false,
  watchProviders: withCast
    ? [
        {
          iso_3166_1: 'BR',
          link: 'https://www.themoviedb.org/tv/1399/watch?locale=BR',
          buy: [],
          flatrate: [
            {
              displayPriority: 1,
              logoPath: '/logo.png',
              id: 8,
              name: 'Netflix',
            },
          ],
        },
      ]
    : [],
});

const fixtureFor = (pathname: string) => {
  if (pathname === '/api/v1/auth/me') {
    return {
      id: 1,
      displayName: 'Alice',
      email: 'alice@example.com',
      avatar: '',
      permissions: 0,
      userType: 2,
      warnings: [],
      createdAt: '',
      updatedAt: '',
      requestCount: 0,
    };
  }
  // The api's fixed slider manifest: the personal list, the two library
  // lists, then the nine kept built-ins, in its order.
  if (pathname === '/api/v1/settings/discover') {
    return [
      SliderId.FOR_YOU,
      SliderId.MY_LIST,
      SliderId.COUPLE_LIST,
      DiscoverSliderType.TRENDING,
      DiscoverSliderType.POPULAR_MOVIES,
      DiscoverSliderType.MOVIE_GENRES,
      DiscoverSliderType.UPCOMING_MOVIES,
      DiscoverSliderType.STUDIOS,
      DiscoverSliderType.POPULAR_TV,
      DiscoverSliderType.TV_GENRES,
      DiscoverSliderType.UPCOMING_TV,
      DiscoverSliderType.NETWORKS,
    ].map((type, order) => ({
      id: type,
      type,
      order,
      isBuiltIn: true,
      enabled: true,
      title: null,
      data: null,
    }));
  }
  if (pathname === '/api/v1/discover/genreslider/movie') {
    return level === 'empty'
      ? []
      : [{ id: 28, name: 'Action', backdrops: ['/a', '/b', '/c', '/d', '/e'] }];
  }
  if (pathname === '/api/v1/discover/genreslider/tv') {
    return level === 'empty'
      ? []
      : [{ id: 18, name: 'Drama', backdrops: ['/a', '/b', '/c', '/d', '/e'] }];
  }
  if (pathname === '/api/v1/genres/movie' || pathname === '/api/v1/genres/tv') {
    return level === 'empty' ? [] : [{ id: 28, name: 'Action' }];
  }
  if (pathname.startsWith('/api/v1/keyword/')) {
    return { id: 9715, name: 'keyword' };
  }
  if (pathname.startsWith('/api/v1/studio/')) {
    return { id: 2, name: 'Studio' };
  }
  if (pathname.startsWith('/api/v1/network/')) {
    return { id: 213, name: 'Network' };
  }
  if (pathname === '/api/v1/languages') {
    return level === 'empty'
      ? []
      : [{ iso_639_1: 'pt', english_name: 'Portuguese' }];
  }
  if (
    pathname === '/api/v1/regions' ||
    pathname === '/api/v1/watchproviders/regions'
  ) {
    return level === 'empty'
      ? []
      : [{ iso_3166_1: 'BR', english_name: 'Brazil' }];
  }
  if (pathname.startsWith('/api/v1/watchproviders/')) {
    return level === 'empty'
      ? []
      : [
          {
            provider_id: 8,
            provider_name: 'Netflix',
            logo_path: '/logo.png',
            display_priority: 1,
          },
        ];
  }
  if (pathname.startsWith('/api/v1/certifications/')) {
    return level === 'empty'
      ? { certifications: {} }
      : { certifications: { US: [{ certification: 'PG-13' }] } };
  }
  if (
    pathname === '/api/v1/search' ||
    pathname === '/api/v1/search/keyword' ||
    pathname === '/api/v1/search/company'
  ) {
    if (level === 'empty') {
      return { page: 1, results: [], totalPages: 0, totalResults: 0 };
    }
    if (pathname === '/api/v1/search/company') {
      return {
        page: 1,
        results: [{ id: 2, name: 'Studio' }],
        totalPages: 1,
        totalResults: 1,
      };
    }
    return {
      page: 1,
      results: [
        {
          id: 603,
          mediaType: 'movie',
          title: 'Fixture Movie',
          overview: '',
          posterPath: null,
          releaseDate: '1999-03-31',
          voteAverage: 8,
        },
      ],
      totalPages: 1,
      totalResults: 1,
    };
  }
  if (pathname.startsWith('/api/v1/movie/')) {
    if (
      pathname.endsWith('/recommendations') ||
      pathname.endsWith('/similar')
    ) {
      return level === 'empty'
        ? emptyPage
        : {
            page: 1,
            results: [
              {
                id: 604,
                mediaType: 'movie',
                title: 'Related',
                overview: '',
                posterPath: null,
                releaseDate: '2000-01-01',
                voteAverage: 7,
              },
            ],
            totalPages: 1,
            totalResults: 1,
          };
    }
    return movieFixture(level !== 'empty');
  }
  if (pathname.startsWith('/api/v1/tv/')) {
    if (
      pathname.endsWith('/recommendations') ||
      pathname.endsWith('/similar')
    ) {
      return level === 'empty'
        ? emptyPage
        : {
            page: 1,
            results: [
              {
                id: 1400,
                mediaType: 'tv',
                name: 'Related Series',
                overview: '',
                posterPath: null,
                firstAirDate: '2010-01-01',
                voteAverage: 7,
              },
            ],
            totalPages: 1,
            totalResults: 1,
          };
    }
    const seasonMatch = pathname.match(/\/api\/v1\/tv\/\d+\/season\/\d+$/);
    if (seasonMatch) {
      return {
        id: 10,
        airDate: '2008-01-20',
        name: 'Season 1',
        overview: '',
        posterPath: null,
        seasonNumber: 1,
        episodes:
          level === 'full'
            ? [
                {
                  id: 100,
                  airDate: '2008-01-20',
                  episodeNumber: 1,
                  name: 'Pilot',
                  overview: '',
                  productionCode: '',
                  seasonNumber: 1,
                  showId: 1399,
                  voteAverage: 8,
                  voteCount: 10,
                },
              ]
            : [],
        externalIds: {},
      };
    }
    return tvFixture(level !== 'empty');
  }
  if (pathname.startsWith('/api/v1/collection/')) {
    return {
      id: 2344,
      name: 'Fixture Collection',
      overview: '',
      backdropPath: null,
      posterPath: null,
      parts:
        level === 'empty'
          ? []
          : [
              {
                id: 603,
                title: 'Fixture Movie',
                overview: '',
                posterPath: null,
                releaseDate: '1999-03-31',
                voteAverage: 8,
                genreIds: [],
                mediaType: 'movie',
              },
            ],
    };
  }
  if (pathname.startsWith('/api/v1/person/')) {
    if (pathname.endsWith('/combined_credits')) {
      return level === 'empty'
        ? { cast: [], crew: [] }
        : {
            cast: [
              {
                id: 603,
                mediaType: 'movie',
                title: 'Fixture Movie',
                character: 'Hero',
                overview: '',
                posterPath: null,
                releaseDate: '1999-03-31',
                voteAverage: 8,
                voteCount: 100,
              },
            ],
            crew: [],
          };
    }
    return {
      id: 52583,
      name: 'Fixture Person',
      biography: 'A biography.',
      birthday: '1970-01-01',
      deathday: null,
      placeOfBirth: 'Brasil',
      profilePath: null,
      knownForDepartment: 'Acting',
      homepage: '',
      imdbId: 'nm0000001',
      popularity: 5,
    };
  }
  if (
    pathname === '/api/v1/discover/watchlist' ||
    pathname === '/api/v1/watchlist'
  ) {
    return level === 'empty'
      ? emptyPage
      : {
          page: 1,
          results: [
            {
              id: 1,
              ratingKey: '1',
              tmdbId: 603,
              mediaType: 'movie',
              title: 'Fixture Movie',
            },
          ],
          totalPages: 1,
          totalResults: 1,
        };
  }
  if (pathname.startsWith('/api/v1/discover/')) {
    if (level === 'empty') {
      return {
        ...emptyPage,
        keywords: [],
        genre: { id: 28, name: 'Action' },
        studio: { id: 2, name: 'Studio' },
        network: { id: 213, name: 'Network' },
      };
    }
    return {
      page: 1,
      results: [
        {
          id: 603,
          mediaType: 'movie',
          title: 'Fixture Movie',
          overview: '',
          posterPath: null,
          releaseDate: '1999-03-31',
          voteAverage: 8,
        },
      ],
      totalPages: 1,
      totalResults: 1,
      keywords: [{ id: 9715, name: 'keyword' }],
      genre: { id: 28, name: 'Action' },
      studio: { id: 2, name: 'Studio' },
      network: { id: 213, name: 'Network' },
    };
  }
  if (pathname.startsWith('/api/v1/library/title/')) {
    const person = (displayName: string) => ({
      displayName,
      status: 'unwatched',
      watched: false,
      watchedAt: null,
      rating:
        level === 'full'
          ? { level: 'liked', review: 'Uma resenha.', updatedAt: '' }
          : null,
      regularSeasonsWatched: 0,
      seasons: [],
    });
    return {
      mediaType: pathname.includes('/tv/') ? 'tv' : 'movie',
      tmdbId: Number(pathname.split('/').pop()),
      me: person('Alice'),
      partner: person('Bob'),
      lists: [],
    };
  }
  if (pathname === '/api/v1/library/lists') {
    const list = (
      id: number,
      kind: string,
      owner: string,
      canRename: boolean,
      canChangeItems: boolean
    ) => ({
      id,
      kind,
      name: `List ${id}`,
      owner,
      itemCount: 1,
      createdAt: '',
      canRename,
      canChangeItems,
    });
    return {
      results:
        level === 'empty'
          ? []
          : [
              list(1, 'watchlist', 'me', false, true),
              list(3, 'custom', 'me', true, true),
              list(2, 'couple', 'shared', false, true),
              list(4, 'watchlist', 'partner', false, false),
            ],
    };
  }
  if (/^\/api\/v1\/library\/lists\/\d+\/items$/.test(pathname)) {
    return {
      results:
        level === 'empty'
          ? []
          : [
              {
                mediaType: 'movie',
                tmdbId: 603,
                available: true,
                result: {
                  id: 603,
                  mediaType: 'movie',
                  title: 'Fixture Movie',
                  overview: '',
                  posterPath: null,
                  releaseDate: '1999-03-31',
                  voteAverage: 8,
                },
                addedAt: '',
                addedByMe: true,
                note: null,
              },
            ],
      nextCursor: null,
    };
  }
  if (pathname === '/api/v1/library/history') {
    return {
      results:
        level === 'empty'
          ? []
          : [
              {
                mediaType: 'tv',
                tmdbId: 1399,
                available: true,
                result: {
                  id: 1399,
                  mediaType: 'tv',
                  name: 'Fixture Series',
                  overview: '',
                  posterPath: null,
                  firstAirDate: '2008-01-20',
                  voteAverage: 9,
                },
                scope: 'season',
                seasonNumber: 0,
                activityAt: '',
                watched: true,
                watchedAt: '2026-11-01T00:00:00.000Z',
                rating: { level: 'meh', review: null, updatedAt: '' },
              },
            ],
      nextCursor: null,
    };
  }
  if (pathname === '/api/v1/backdrops') {
    return level === 'empty' ? [] : ['/backdrop.jpg'];
  }
  if (pathname === '/api/v1/recommendations') {
    if (level === 'empty') {
      return {
        page: 1,
        totalPages: 0,
        totalResults: 0,
        personal: false,
        runId: 0,
        results: [],
      };
    }
    if (level === 'minimal') {
      return {
        page: 1,
        totalPages: 1,
        totalResults: 1,
        personal: false,
        runId: 7,
        results: [
          {
            mediaType: 'movie',
            tmdbId: 603,
            available: true,
            result: {
              id: 603,
              mediaType: 'movie',
              title: 'Fixture Movie',
              overview: '',
              posterPath: null,
              releaseDate: '1999-03-31',
              voteAverage: 8,
            },
            reasons: [],
            rank: 1,
            runId: 7,
            onWatchlist: false,
          },
        ],
      };
    }
    return {
      page: 1,
      totalPages: 1,
      totalResults: 2,
      personal: true,
      runId: 7,
      results: [
        {
          mediaType: 'movie',
          tmdbId: 603,
          available: true,
          result: {
            id: 603,
            mediaType: 'movie',
            title: 'Fixture Movie',
            overview: '',
            posterPath: null,
            releaseDate: '1999-03-31',
            voteAverage: 8,
          },
          reasons: ['Seed One', 'Seed Two'],
          rank: 1,
          runId: 7,
          onWatchlist: false,
        },
        {
          mediaType: 'tv',
          tmdbId: 1399,
          available: true,
          result: {
            id: 1399,
            mediaType: 'tv',
            name: 'Fixture Series',
            overview: '',
            posterPath: null,
            firstAirDate: '2008-01-20',
            voteAverage: 9,
          },
          reasons: ['Seed Two'],
          rank: 2,
          runId: 7,
          onWatchlist: true,
        },
      ],
    };
  }
  return {};
};

vi.mock('axios');

const mockedAxios = vi.mocked(axios, true);

const record = (url: string) => {
  const path = url.split('?')[0];
  appendFileSync(runtimeLog, `${JSON.stringify({ url: path })}\n`);
};

const Wrapper = ({ children }: { children: React.ReactNode }) => (
  <SWRConfig
    value={{
      provider: () => new Map(),
      fetcher: (url: string) => mockedAxios.get(url).then((r) => r.data),
    }}
  >
    <IntlProvider
      locale="pt-BR"
      defaultLocale="en"
      messages={{}}
      onError={() => undefined}
    >
      <SettingsProvider currentSettings={settingsFixture}>
        {children}
      </SettingsProvider>
    </IntlProvider>
  </SWRConfig>
);

const wrapper = Wrapper;

const loggedPaths = (): string[] => {
  if (!existsSync(runtimeLog)) {
    return [];
  }
  return readFileSync(runtimeLog, 'utf8')
    .split('\n')
    .filter((line) => line.trim() !== '')
    .map((line) => (JSON.parse(line) as { url: string }).url);
};

const renderAndSettle = async (node: React.ReactNode) => {
  const { unmount } = render(node, { wrapper });
  // Let pending SWR fetches resolve.
  await new Promise((resolve) => setTimeout(resolve, 150));
  unmount();
};

beforeAll(async () => {
  mkdirSync(dirname(runtimeLog), { recursive: true });
  rmSync(runtimeLog, { force: true });
  if (typeof IntersectionObserver === 'undefined') {
    class StubObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
    vi.stubGlobal('IntersectionObserver', StubObserver);
  }
  if (typeof ResizeObserver === 'undefined') {
    class StubResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
    vi.stubGlobal('ResizeObserver', StubResizeObserver);
  }
  mockedAxios.get.mockImplementation(async (url: string) => {
    record(url);
    return { data: fixtureFor(url.split('?')[0]) };
  });
  mockedAxios.post.mockImplementation(async (url: string) => {
    record(url);
    return { data: {}, status: 200 };
  });
  mockedAxios.delete.mockImplementation(async (url: string) => {
    record(url);
    return { data: {}, status: 204 };
  });
  mockedAxios.put.mockImplementation(async (url: string) => {
    record(url);
    return { data: {}, status: 200 };
  });
  mockedAxios.patch.mockImplementation(async (url: string) => {
    record(url);
    return { data: {}, status: 200 };
  });
});

describe('renderers against fixtures', () => {
  it('renders the discover page empty, minimal and full', async () => {
    for (const next of ['empty', 'minimal', 'full'] as Level[]) {
      level = next;
      await renderAndSettle(<Discover />);
    }
  });

  it('renders discover subpages', async () => {
    level = 'full';
    await renderAndSettle(<Trending />);
    await renderAndSettle(<Upcoming />);
    await renderAndSettle(<DiscoverTvUpcoming />);
    await renderAndSettle(<MovieGenreList />);
    await renderAndSettle(<TvGenreList />);
    await renderAndSettle(<DiscoverMovies />);
    await renderAndSettle(<DiscoverTv />);
    await renderAndSettle(<DiscoverMovieGenre />);
    await renderAndSettle(<DiscoverTvGenre />);
    await renderAndSettle(<DiscoverMovieLanguage />);
    await renderAndSettle(<DiscoverTvLanguage />);
    await renderAndSettle(<DiscoverStudio />);
    await renderAndSettle(<DiscoverNetwork />);
    await renderAndSettle(<DiscoverMovieKeyword />);
    await renderAndSettle(<DiscoverTvKeyword />);
    await renderAndSettle(<DiscoverWatchlist />);
  });

  it('renders the movie page empty, minimal and full', async () => {
    for (const next of ['empty', 'minimal', 'full'] as Level[]) {
      level = next;
      await renderAndSettle(<MovieDetails />);
    }
    level = 'full';
    await renderAndSettle(<MovieCast />);
    await renderAndSettle(<MovieCrew />);
    await renderAndSettle(<MovieRecommendations />);
    await renderAndSettle(<MovieSimilar />);
  });

  it('renders the series page empty, minimal and full, with a season', async () => {
    for (const next of ['empty', 'minimal', 'full'] as Level[]) {
      level = next;
      await renderAndSettle(<TvDetails />);
    }
    level = 'full';
    await renderAndSettle(<Season seasonNumber={1} tvId={1399} />);
    await renderAndSettle(<TvCast />);
    await renderAndSettle(<TvCrew />);
    await renderAndSettle(<TvRecommendations />);
    await renderAndSettle(<TvSimilar />);
  });

  it('renders the About page with the four credit lines', async () => {
    level = 'full';
    const view = render(
      <Wrapper>
        <About />
      </Wrapper>
    );
    await waitFor(() => {
      expect(
        view.getByText(
          'This product uses the TMDB API but is not endorsed or certified by TMDB.'
        )
      ).toBeTruthy();
    });
    expect(
      view.getByText(
        'Interface built on Seerr, an open-source project under the MIT licence.'
      )
    ).toBeTruthy();
    expect(
      view.getByText(
        'Posters and images load directly from the TMDB image servers. If they are unreachable, artwork may be missing.'
      )
    ).toBeTruthy();
    expect(
      view.getByText('Streaming provider data provided by JustWatch.')
    ).toBeTruthy();
    expect(view.getByAltText('The Movie Database')).toBeTruthy();
    view.unmount();
  });

  it('shows the JustWatch credit next to the provider lists', async () => {
    level = 'full';
    const movie = render(
      <Wrapper>
        <MovieDetails />
      </Wrapper>
    );
    await waitFor(() => {
      expect(movie.getByText('Currently Streaming On')).toBeTruthy();
    });
    expect(movie.getByAltText('Netflix')).toBeTruthy();
    expect(
      movie.getByText('Streaming provider data provided by JustWatch.')
    ).toBeTruthy();
    movie.unmount();

    const series = render(
      <Wrapper>
        <TvDetails />
      </Wrapper>
    );
    await waitFor(() => {
      expect(series.getByText('Currently Streaming On')).toBeTruthy();
    });
    expect(series.getByAltText('Netflix')).toBeTruthy();
    expect(
      series.getByText('Streaming provider data provided by JustWatch.')
    ).toBeTruthy();
    series.unmount();
  });

  it('renders the collection and person pages empty, minimal and full', async () => {
    for (const next of ['empty', 'minimal', 'full'] as Level[]) {
      level = next;
      await renderAndSettle(<CollectionDetails />);
      await renderAndSettle(<PersonDetails />);
    }
  });

  it('renders search and the lookup selectors', async () => {
    level = 'full';
    await renderAndSettle(<Search />);
    await renderAndSettle(
      <CompanySelector defaultValue="2" onChange={() => undefined} />
    );
    await renderAndSettle(
      <GenreSelector type="movie" isMulti onChange={() => undefined} />
    );
    await renderAndSettle(
      <GenreSelector type="tv" isMulti onChange={() => undefined} />
    );
    await renderAndSettle(
      <KeywordSelector defaultValue="9715" isMulti onChange={() => undefined} />
    );
    await renderAndSettle(
      <WatchProviderSelector
        type="movie"
        region="BR"
        activeProviders={[]}
        onChange={() => undefined}
      />
    );
    await renderAndSettle(
      <WatchProviderSelector
        type="tv"
        region="BR"
        activeProviders={[]}
        onChange={() => undefined}
      />
    );
    await renderAndSettle(<LanguageSelector setFieldValue={() => undefined} />);
    await renderAndSettle(<RegionSelector value="BR" name="region" />);
    await renderAndSettle(
      <CertificationSelector type="movie" onChange={() => undefined} />
    );
    await renderAndSettle(
      <CertificationSelector type="tv" onChange={() => undefined} />
    );
    await renderAndSettle(<CompanyTag type="studio" companyId={2} />);
    await renderAndSettle(<CompanyTag type="network" companyId={213} />);
    await renderAndSettle(<GenreTag type="movie" genreId={28} />);
    await renderAndSettle(<GenreTag type="tv" genreId={18} />);
    await renderAndSettle(<KeywordTag keywordId={9715} />);
    await renderAndSettle(<TmdbTitleCard id={603} tmdbId={603} type="movie" />);
    await renderAndSettle(
      <TitleCard id={603} title="Fixture" mediaType="movie" />
    );
  });

  it('drives the async selectors, covering the search-company and search-keyword lookups', async () => {
    level = 'full';
    const company = render(
      <Wrapper>
        <CompanySelector onChange={() => undefined} />
      </Wrapper>
    );
    const companyInput = company.container.querySelector(
      'input'
    ) as HTMLInputElement;
    fireEvent.change(companyInput, { target: { value: 'studio' } });
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/search/company');
    });
    company.unmount();

    const keyword = render(
      <Wrapper>
        <KeywordSelector isMulti onChange={() => undefined} />
      </Wrapper>
    );
    const keywordInput = keyword.container.querySelector(
      'input'
    ) as HTMLInputElement;
    fireEvent.change(keywordInput, { target: { value: 'keyword' } });
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/search/keyword');
    });
    keyword.unmount();
  });

  it('signs in through the login form, covering auth/local', async () => {
    level = 'empty';
    const loginRouter = {
      pathname: '/login',
      route: '/login',
      asPath: '/login',
      query: {},
      push: vi.fn(),
      replace: vi.fn(),
    };
    const screen = render(
      <RouterContext.Provider value={loginRouter as never}>
        <Wrapper>
          <LoginPage />
        </Wrapper>
      </RouterContext.Provider>
    );
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/backdrops');
    });
    fireEvent.change(screen.getByPlaceholderText('Username'), {
      target: { value: 'alice' },
    });
    fireEvent.change(screen.getByPlaceholderText('Password'), {
      target: { value: 'sentinelsentinelsentinel' },
    });
    fireEvent.submit(screen.container.querySelector('form') as HTMLFormElement);
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/auth/local');
    });
    screen.unmount();
  });

  it('signs out through the user menu, covering auth/logout', async () => {
    level = 'empty';
    const screen = render(
      <Wrapper>
        <UserDropdown />
      </Wrapper>
    );
    fireEvent.click(screen.getByTestId('user-menu'));
    await waitFor(() => {
      expect(screen.getByText('Sair')).toBeTruthy();
    });
    fireEvent.click(screen.getByText('Sair'));
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/auth/logout');
    });
    screen.unmount();
  });

  it('toggles the watchlist star, covering the watchlist routes', async () => {
    level = 'empty';
    const add = render(
      <Wrapper>
        <TitleCard id={603} title="Fixture" mediaType="movie" />
      </Wrapper>
    );
    fireEvent.click(add.getByTestId('quick-watchlist'));
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/watchlist');
    });
    add.unmount();

    const remove = render(
      <Wrapper>
        <TitleCard
          id={603}
          title="Fixture"
          mediaType="movie"
          isAddedToWatchlist={1}
        />
      </Wrapper>
    );
    fireEvent.click(remove.getByTestId('quick-watchlist'));
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/watchlist/603');
    });
    remove.unmount();
  });

  it('renders the lists and history pages empty, minimal and full', async () => {
    for (const next of ['empty', 'minimal', 'full'] as Level[]) {
      level = next;
      await renderAndSettle(<ListsPage />);
      await renderAndSettle(<HistoryPage />);
    }
  });

  it('drives the library controls, covering the library mutation routes', async () => {
    level = 'full';
    const card = render(
      <Wrapper>
        <TitleCard id={603} title="Fixture" mediaType="movie" />
      </Wrapper>
    );
    fireEvent.click(card.getByTestId('quick-watched'));
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/library/watch');
    });
    card.unmount();

    // Rating lives on the title's page, not on the card.
    const panel = render(
      <Wrapper>
        <TitlePanel mediaType="movie" tmdbId={603} />
      </Wrapper>
    );
    fireEvent.click(panel.getByTestId('title-rate'));
    fireEvent.click(panel.getByTestId('rating-level-loved'));
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/library/rating');
    });
    panel.unmount();

    const lists = render(
      <Wrapper>
        <ListsPage />
      </Wrapper>
    );
    const custom = await lists.findByTestId('library-list-custom');
    const removeItem = await within(custom).findByTestId('list-item-remove');
    fireEvent.click(removeItem);
    await waitFor(() => {
      expect(loggedPaths()).toContain(
        '/api/v1/library/lists/3/items/movie/603'
      );
    });
    const deleteList = within(custom)
      .getByTestId('list-delete')
      .closest('button') as HTMLButtonElement;
    fireEvent.click(within(custom).getByTestId('list-rename'));
    fireEvent.change(within(custom).getByTestId('list-rename-input'), {
      target: { value: 'Renamed' },
    });
    fireEvent.submit(
      within(custom)
        .getByTestId('list-rename-input')
        .closest('form') as HTMLFormElement
    );
    await waitFor(() => {
      expect(loggedPaths()).toContain('/api/v1/library/lists/3');
    });
    await waitFor(() => {
      expect(
        (within(custom).getByTestId('list-rename') as HTMLButtonElement)
          .disabled
      ).toBe(false);
    });
    fireEvent.click(deleteList);
    fireEvent.click(deleteList);
    await waitFor(() => {
      expect(
        loggedPaths().filter((path) => path === '/api/v1/library/lists/3')
      ).toHaveLength(2);
    });
    lists.unmount();
  });

  it('makes no request to dropped polling routes', async () => {
    const urls = loggedPaths();
    expect(urls.length).toBeGreaterThan(0);
    for (const url of urls) {
      expect(url).not.toContain('/api/v1/status');
      expect(url).not.toContain('/quota');
      expect(url).not.toContain('/api/v1/request/count');
      expect(url).not.toContain('/api/v1/issue/count');
    }
  });
});

describe('discover filters (tt-t50)', () => {
  const setRouterQuery = async (query: Record<string, string>) => {
    const routerModule = await import('next/router');
    const replace = vi.fn();
    const push = vi.fn();
    const spy = vi.spyOn(routerModule, 'useRouter').mockReturnValue({
      pathname: '/',
      query,
      asPath: '/',
      replace,
      push,
    } as never);
    return { spy, replace };
  };

  const settleSlideover = async () => {
    await waitFor(
      () => {
        expect(document.body.textContent).toContain('Runtime');
      },
      { timeout: 3000 }
    );
  };

  it('shows the movie date, runtime, score and vote-count controls', async () => {
    level = 'full';
    const { spy } = await setRouterQuery({});
    const view = render(
      <Wrapper>
        <FilterSlideover
          show
          type="movie"
          currentFilters={{}}
          onClose={() => undefined}
        />
      </Wrapper>
    );
    await settleSlideover();
    expect(document.body.textContent).toContain('Release Date');
    expect(document.body.textContent).toContain('TMDB User Score');
    expect(document.body.textContent).toContain('TMDB User Vote Count');
    expect(document.body.querySelector('input[name="fromdate"]')).toBeTruthy();
    expect(document.body.querySelector('input[name="todate"]')).toBeTruthy();
    expect(document.body.querySelectorAll('input[type="range"]').length).toBe(
      6
    );
    expect(document.body.textContent).not.toContain('First Air Date');
    expect(document.body.textContent).not.toContain('Status');
    expect(document.body.textContent).not.toMatch(
      /hide-available|hide-requested|blocklist/i
    );
    view.unmount();
    spy.mockRestore();
  });

  it('shows the series first-air-date range and status selector', async () => {
    level = 'full';
    const { spy } = await setRouterQuery({});
    const view = render(
      <Wrapper>
        <FilterSlideover
          show
          type="tv"
          currentFilters={{}}
          onClose={() => undefined}
        />
      </Wrapper>
    );
    await settleSlideover();
    expect(document.body.textContent).toContain('First Air Date');
    expect(document.body.textContent).toContain('Status');
    expect(document.body.textContent).toContain('Runtime');
    expect(document.body.textContent).toContain('TMDB User Score');
    expect(document.body.textContent).toContain('TMDB User Vote Count');
    expect(document.body.textContent).toContain('Select status...');
    expect(document.body.querySelector('input[name="fromdate"]')).toBeTruthy();
    expect(document.body.querySelector('input[name="todate"]')).toBeTruthy();
    expect(document.body.querySelectorAll('input[type="range"]').length).toBe(
      6
    );
    expect(document.body.textContent).not.toContain('Release Date');
    expect(document.body.textContent).not.toMatch(
      /hide-available|hide-requested|blocklist/i
    );
    view.unmount();
    spy.mockRestore();
  });

  it('puts every movie filter on the discover movies request', async () => {
    level = 'full';
    const { spy } = await setRouterQuery({
      primaryReleaseDateGte: '2020-01-01',
      primaryReleaseDateLte: '2024-12-31',
      withRuntimeGte: '60',
      withRuntimeLte: '180',
      voteAverageGte: '7',
      voteAverageLte: '9',
      voteCountGte: '100',
      voteCountLte: '5000',
    });
    mockedAxios.get.mockClear();
    const view = render(
      <Wrapper>
        <DiscoverMovies />
      </Wrapper>
    );
    await waitFor(
      () => {
        const urls = mockedAxios.get.mock.calls.map(([url]) =>
          decodeURIComponent(String(url))
        );
        const discover = urls.filter((url) =>
          url.startsWith('/api/v1/discover/movies?')
        );
        expect(discover.length).toBeGreaterThan(0);
        const first = discover[0];
        expect(first).toContain('primaryReleaseDateGte=2020-01-01');
        expect(first).toContain('primaryReleaseDateLte=2024-12-31');
        expect(first).toContain('withRuntimeGte=60');
        expect(first).toContain('withRuntimeLte=180');
        expect(first).toContain('voteAverageGte=7');
        expect(first).toContain('voteAverageLte=9');
        expect(first).toContain('voteCountGte=100');
        expect(first).toContain('voteCountLte=5000');
      },
      { timeout: 5000 }
    );
    view.unmount();
    spy.mockRestore();
  });

  it('puts every series filter on the discover tv request', async () => {
    level = 'full';
    const { spy } = await setRouterQuery({
      firstAirDateGte: '2010-01-01',
      firstAirDateLte: '2020-12-31',
      status: '0|3',
      withRuntimeGte: '45',
      withRuntimeLte: '120',
      voteAverageGte: '6',
      voteAverageLte: '8',
      voteCountGte: '50',
      voteCountLte: '2000',
    });
    mockedAxios.get.mockClear();
    const view = render(
      <Wrapper>
        <DiscoverTv />
      </Wrapper>
    );
    await waitFor(
      () => {
        const urls = mockedAxios.get.mock.calls.map(([url]) =>
          decodeURIComponent(String(url))
        );
        const discover = urls.filter((url) =>
          url.startsWith('/api/v1/discover/tv?')
        );
        expect(discover.length).toBeGreaterThan(0);
        const first = discover[0];
        expect(first).toContain('firstAirDateGte=2010-01-01');
        expect(first).toContain('firstAirDateLte=2020-12-31');
        expect(first).toContain('status=0|3');
        expect(first).toContain('withRuntimeGte=45');
        expect(first).toContain('withRuntimeLte=120');
        expect(first).toContain('voteAverageGte=6');
        expect(first).toContain('voteAverageLte=8');
        expect(first).toContain('voteCountGte=50');
        expect(first).toContain('voteCountLte=2000');
      },
      { timeout: 5000 }
    );
    view.unmount();
    spy.mockRestore();
  });

  it('sets the movie query key from each movie control', async () => {
    level = 'full';
    const { spy, replace } = await setRouterQuery({});
    const view = render(
      <Wrapper>
        <FilterSlideover
          show
          type="movie"
          currentFilters={{}}
          onClose={() => undefined}
        />
      </Wrapper>
    );
    await settleSlideover();

    const replacedWith = (key: string, value: string) =>
      replace.mock.calls.some(([pathname]) =>
        String(pathname).includes(`${key}=${value}`)
      );

    fireEvent.change(
      document.body.querySelector('input[name="fromdate"]') as HTMLInputElement,
      { target: { value: '2020-01-15' } }
    );
    await waitFor(
      () => {
        expect(replacedWith('primaryReleaseDateGte', '2020-01-15')).toBe(true);
      },
      { timeout: 3000 }
    );
    replace.mockClear();

    fireEvent.change(
      document.body.querySelector('input[name="todate"]') as HTMLInputElement,
      { target: { value: '2024-06-30' } }
    );
    await waitFor(
      () => {
        expect(replacedWith('primaryReleaseDateLte', '2024-06-30')).toBe(true);
      },
      { timeout: 3000 }
    );
    replace.mockClear();

    const ranges = document.body.querySelectorAll('input[type="range"]');
    expect(ranges.length).toBe(6);
    const steps: [number, string, string][] = [
      [0, 'withRuntimeGte', '100'],
      [1, 'withRuntimeLte', '300'],
      [2, 'voteAverageGte', '7'],
      [3, 'voteAverageLte', '9'],
      [4, 'voteCountGte', '50'],
      [5, 'voteCountLte', '800'],
    ];
    for (const [index, key, value] of steps) {
      fireEvent.change(ranges[index], { target: { value } });
      await waitFor(
        () => {
          expect(replacedWith(key, value)).toBe(true);
        },
        { timeout: 3000 }
      );
      replace.mockClear();
    }
    view.unmount();
    spy.mockRestore();
  });

  it('sets the series query key from each series-only control', async () => {
    level = 'full';
    const { spy, replace } = await setRouterQuery({});
    const view = render(
      <Wrapper>
        <FilterSlideover
          show
          type="tv"
          currentFilters={{}}
          onClose={() => undefined}
        />
      </Wrapper>
    );
    await settleSlideover();

    fireEvent.change(
      document.body.querySelector('input[name="fromdate"]') as HTMLInputElement,
      { target: { value: '2010-03-20' } }
    );
    await waitFor(
      () => {
        expect(
          replace.mock.calls.some(([pathname]) =>
            String(pathname).includes('firstAirDateGte=2010-03-20')
          )
        ).toBe(true);
      },
      { timeout: 3000 }
    );
    replace.mockClear();

    fireEvent.change(
      document.body.querySelector('input[name="todate"]') as HTMLInputElement,
      { target: { value: '2020-11-05' } }
    );
    await waitFor(
      () => {
        expect(
          replace.mock.calls.some(([pathname]) =>
            String(pathname).includes('firstAirDateLte=2020-11-05')
          )
        ).toBe(true);
      },
      { timeout: 3000 }
    );
    replace.mockClear();

    const containers = [
      ...document.body.querySelectorAll('.react-select-container'),
    ];
    const statusContainer = containers.find((container) =>
      container.textContent?.includes('Select status')
    );
    expect(statusContainer).toBeTruthy();
    const statusInput = statusContainer?.querySelector(
      'input'
    ) as HTMLInputElement;
    fireEvent.focus(statusInput);
    fireEvent.keyDown(statusInput, { key: 'ArrowDown', code: 'ArrowDown' });
    await waitFor(
      () => {
        expect(document.body.textContent).toContain('Returning Series');
      },
      { timeout: 3000 }
    );
    const options = [
      ...document.body.querySelectorAll('.react-select__option'),
    ];
    expect(options.length).toBeGreaterThan(0);
    fireEvent.click(options[0]);
    await waitFor(
      () => {
        expect(
          replace.mock.calls.some(([pathname]) =>
            String(pathname).includes('status=0')
          )
        ).toBe(true);
      },
      { timeout: 3000 }
    );
    view.unmount();
    spy.mockRestore();
  });
});

describe('for-you recommendations', () => {
  it('renders the slider and the page empty, cold and personal', async () => {
    level = 'empty';
    const emptySlider = render(
      <Wrapper>
        <ForYouSlider title="For you" sliderKey="for-you" />
      </Wrapper>
    );
    await waitFor(() => {
      expect(emptySlider.getByTestId('for-you-slider')).toBeTruthy();
    });
    expect(emptySlider.getByTestId('for-you-cold')).toBeTruthy();
    expect(emptySlider.queryByTestId('recs-exposure-0-1')).toBeNull();
    emptySlider.unmount();

    const emptyPage = render(
      <Wrapper>
        <ForYouPage />
      </Wrapper>
    );
    await waitFor(() => {
      expect(emptyPage.getByTestId('for-you-empty')).toBeTruthy();
    });
    expect(emptyPage.getByTestId('for-you-cold')).toBeTruthy();
    emptyPage.unmount();

    level = 'minimal';
    const coldSlider = render(
      <Wrapper>
        <ForYouSlider title="For you" sliderKey="for-you" />
      </Wrapper>
    );
    await waitFor(() => {
      expect(coldSlider.getByTestId('recs-exposure-7-1')).toBeTruthy();
    });
    expect(coldSlider.getByTestId('for-you-cold')).toBeTruthy();
    expect(coldSlider.queryByText(/Because you liked/)).toBeNull();
    coldSlider.unmount();

    level = 'full';
    const slider = render(
      <Wrapper>
        <ForYouSlider title="For you" sliderKey="for-you" />
      </Wrapper>
    );
    await waitFor(() => {
      expect(
        slider.getByText('Because you liked Seed One, Seed Two')
      ).toBeTruthy();
    });
    expect(slider.getByText('Because you liked Seed Two')).toBeTruthy();
    expect(slider.queryByTestId('for-you-cold')).toBeNull();
    slider.unmount();

    // A card entering the viewport posts its exposure: with an observer
    // that reports everything visible at once, the slider records both
    // ranks, which the api inventory diffs against the route manifest.
    class VisibleObserver {
      callback: IntersectionObserverCallback;
      constructor(callback: IntersectionObserverCallback) {
        this.callback = callback;
      }
      observe(target: Element) {
        this.callback(
          [
            {
              isIntersecting: true,
              intersectionRatio: 1,
              target,
            } as IntersectionObserverEntry,
          ],
          this as unknown as IntersectionObserver
        );
      }
      unobserve() {}
      disconnect() {}
    }
    vi.stubGlobal('IntersectionObserver', VisibleObserver);
    mockedAxios.post.mockClear();
    const exposed = render(
      <Wrapper>
        <ForYouSlider title="For you" sliderKey="for-you" />
      </Wrapper>
    );
    await waitFor(() => {
      expect(
        loggedPaths().filter(
          (path) => path === '/api/v1/recommendations/exposures'
        )
      ).toHaveLength(2);
    });
    exposed.unmount();

    const page = render(
      <Wrapper>
        <ForYouPage />
      </Wrapper>
    );
    await waitFor(() => {
      expect(page.getByTestId('for-you-grid')).toBeTruthy();
    });
    expect(page.getByText('Because you liked Seed One, Seed Two')).toBeTruthy();
    expect(page.queryByTestId('for-you-cold')).toBeNull();
    fireEvent.click(page.getByTestId('for-you-tab-movie'));
    await waitFor(() => {
      expect(page.getByTestId('for-you-grid')).toBeTruthy();
    });
    page.unmount();
  });
});

describe('title pages keep upstream layout (tt-s73)', () => {
  // The api's own answer for the full fixtures, as the goldens record it.
  // The goldens replace volatile numbers with a placeholder, so the score
  // fields get concrete values here.
  const goldenTitleBody = (name: string) => ({
    ...JSON.parse(
      readFileSync(
        join(here, '..', '..', '..', 'testdata', 'golden', 'title', name),
        'utf8'
      )
    ).body,
    voteAverage: 8.2,
    voteCount: 26000,
  });

  // Serves `body` for the page's own details path and everything else from
  // the shared fixture router, then puts the shared router back.
  const renderTitleWith = async (
    detailsPath: string,
    body: Record<string, unknown>,
    node: React.ReactNode,
    check: (view: ReturnType<typeof render>) => Promise<void> | void
  ) => {
    level = 'full';
    mockedAxios.get.mockImplementation(async (url: string) => {
      record(url);
      const pathname = url.split('?')[0];
      if (pathname === detailsPath) {
        return { data: body };
      }
      // The trailer link is built from the api's public settings
      // (`youtubeUrl`), which the shared router answers with `{}`.
      if (pathname === '/api/v1/settings/public') {
        return { data: settingsFixture };
      }
      return { data: fixtureFor(pathname) };
    });
    const view = render(<Wrapper>{node}</Wrapper>);
    try {
      await waitFor(() => {
        expect(view.container.querySelector('.media-facts')).toBeTruthy();
      });
      await check(view);
    } finally {
      view.unmount();
      mockedAxios.get.mockImplementation(async (url: string) => {
        record(url);
        return { data: fixtureFor(url.split('?')[0]) };
      });
    }
  };

  const movieBody = () => goldenTitleBody('movie-details-full.json');
  const seriesBody = () => goldenTitleBody('tv-details-full.json');
  const renderMovie = (
    body: Record<string, unknown>,
    check: (view: ReturnType<typeof render>) => Promise<void> | void
  ) => renderTitleWith('/api/v1/movie/603', body, <MovieDetails />, check);
  const renderSeries = (
    body: Record<string, unknown>,
    check: (view: ReturnType<typeof render>) => Promise<void> | void
  ) => renderTitleWith('/api/v1/tv/1399', body, <TvDetails />, check);

  const factValue = (view: ReturnType<typeof render>, label: string) => {
    const fact = [
      ...view.container.querySelectorAll('.media-facts .media-fact'),
    ].find((el) => el.firstElementChild?.textContent === label);
    return fact?.querySelector('.media-fact-value') ?? null;
  };
  const link = (view: ReturnType<typeof render>, href: string) =>
    view.container.querySelector(`a[href="${href}"]`);

  it('shows the movie facts column', async () => {
    await renderMovie(movieBody(), (view) => {
      expect(factValue(view, 'Original Title')?.textContent).toBe('The Matrix');
      expect(factValue(view, 'Status')?.textContent).toBe('Released');
      expect(factValue(view, 'Release Dates')?.children).toHaveLength(2);
      expect(factValue(view, 'Revenue')?.textContent).toContain('463');
      expect(factValue(view, 'Budget')?.textContent).toContain('63');
      expect(
        factValue(view, 'Original Language')?.querySelector(
          'a[href="/discover/movies/language/en"]'
        )
      ).toBeTruthy();
      expect(factValue(view, 'Production Country')?.textContent).toBeTruthy();
      expect(
        factValue(view, 'Studios')?.querySelectorAll(
          'a[href^="/discover/movies/studio/"]'
        )
      ).toHaveLength(4);
      expect(
        view.container.querySelector('.media-attributes')?.textContent
      ).toContain('136 minutes');
    });
  });

  it('shows the movie TMDB score, trailer, crew link, genres, keywords and collection', async () => {
    await renderMovie(movieBody(), (view) => {
      const score = view.container.querySelector(
        '.media-ratings a.media-rating[href="https://www.themoviedb.org/movie/603?language=pt-BR"]'
      );
      expect(score?.textContent).toBe('82%');

      const trailer = view.getByText('Watch Trailer').closest('a');
      expect(trailer?.getAttribute('href')).toMatch(
        /^https:\/\/www\.youtube\.com\/watch\?v=/
      );

      expect(link(view, '/movie/603/crew')?.textContent).toBe('View Full Crew');
      expect(
        view.container.querySelectorAll('ul.media-crew a[href^="/person/"]')
          .length
      ).toBeGreaterThan(0);

      expect(link(view, '/discover/movies?genre=28')?.textContent).toBe('Ação');
      expect(link(view, '/discover/movies?genre=878')).toBeTruthy();

      expect(
        view.container.querySelectorAll('a[href^="/discover/movies?keywords="]')
      ).toHaveLength(19);
      expect(link(view, '/discover/movies?keywords=312')?.textContent).toBe(
        'man vs machine'
      );

      expect(link(view, '/collection/2344')?.textContent).toContain(
        'Matrix: Coleção'
      );
    });
  });

  it('folds a long studio list behind show more and show less', async () => {
    const body = movieBody();
    body.productionCompanies = [
      ...body.productionCompanies,
      { id: 9999, name: 'Fifth Studio', originCountry: 'BR' },
    ];
    await renderMovie(body, async (view) => {
      const studios = () =>
        factValue(view, 'Studios')?.querySelectorAll(
          'a[href^="/discover/movies/studio/"]'
        ).length;
      expect(studios()).toBe(3);
      fireEvent.click(view.getByText('Show More'));
      await waitFor(() => expect(studios()).toBe(5));
      fireEvent.click(view.getByText('Show Less'));
      await waitFor(() => expect(studios()).toBe(3));
    });
  });

  it('shows the series facts column with seasons and networks', async () => {
    await renderSeries(seriesBody(), (view) => {
      expect(factValue(view, 'Original Title')?.textContent).toBe(
        'Adolescence'
      );
      expect(factValue(view, 'Status')?.textContent).toBe('Ended');
      expect(factValue(view, 'First Air Date')?.textContent).toBeTruthy();
      expect(
        factValue(view, 'Original Language')?.querySelector(
          'a[href="/discover/tv/language/en"]'
        )
      ).toBeTruthy();
      expect(factValue(view, 'Production Countries')?.children).toHaveLength(2);
      expect(
        factValue(view, 'Network')?.querySelector(
          'a[href="/discover/tv/network/213"]'
        )?.textContent
      ).toBe('Netflix');
      expect(
        view.container.querySelector('.media-attributes')?.textContent
      ).toContain('1 Season');
      expect(view.getByText('Seasons').tagName).toBe('H2');
    });
  });

  it('shows the series TMDB score, trailer, crew link, genres and keywords', async () => {
    await renderSeries(seriesBody(), (view) => {
      const score = view.container.querySelector(
        '.media-ratings a.media-rating[href="https://www.themoviedb.org/tv/249042?language=pt-BR"]'
      );
      expect(score?.textContent).toBe('82%');

      const trailer = view.getByText('Watch Trailer').closest('a');
      expect(trailer?.getAttribute('href')).toBe(
        'https://www.youtube.com/watch?v=mh48KXaCSxM'
      );

      expect(link(view, '/tv/249042/crew')?.textContent).toBe('View Full Crew');
      const crew = view.container.querySelector('ul.media-crew li');
      expect(crew?.textContent).toBe('CreatorStephen Graham');

      expect(link(view, '/discover/tv?genre=18')?.textContent).toBe('Drama');
      expect(link(view, '/discover/tv?keywords=392')?.textContent).toBe(
        'england'
      );
    });
  });

  it('lists the BR providers once, with the JustWatch credit beside them', async () => {
    await renderMovie(movieBody(), (view) => {
      const labels = view.getAllByText('Currently Streaming On');
      expect(labels).toHaveLength(1);
      const fact = labels[0].closest('.media-fact') as HTMLElement;
      expect(within(fact).getByAltText('HBO Max')).toBeTruthy();
      expect(
        within(fact).getByText('Streaming provider data provided by JustWatch.')
      ).toBeTruthy();
      expect(
        view.getAllByText('Streaming provider data provided by JustWatch.')
      ).toHaveLength(1);
      // The About page's separate block put the providers under an h2.
      expect(
        [...view.container.querySelectorAll('h2')].map((h) => h.textContent)
      ).not.toContain('Currently Streaming On');
    });
    await renderSeries(seriesBody(), (view) => {
      const labels = view.getAllByText('Currently Streaming On');
      expect(labels).toHaveLength(1);
      const fact = labels[0].closest('.media-fact') as HTMLElement;
      expect(
        within(fact).getByAltText('Netflix Standard with Ads')
      ).toBeTruthy();
      expect(
        within(fact).getByText('Streaming provider data provided by JustWatch.')
      ).toBeTruthy();
      expect(
        view.getAllByText('Streaming provider data provided by JustWatch.')
      ).toHaveLength(1);
    });
  });

  it('shows neither the providers nor the credit for a title with no BR provider', async () => {
    const withoutBrazil = (body: Record<string, unknown>) => ({
      ...body,
      watchProviders: (body.watchProviders as { iso_3166_1: string }[]).filter(
        (entry) => entry.iso_3166_1 !== 'BR'
      ),
    });
    for (const [page, body] of [
      [renderMovie, movieBody()],
      [renderSeries, seriesBody()],
    ] as const) {
      expect((body.watchProviders as unknown[]).length).toBeGreaterThan(1);
      await page(withoutBrazil(body), (view) => {
        expect(view.queryByText('Currently Streaming On')).toBeNull();
        expect(
          view.queryByText('Streaming provider data provided by JustWatch.')
        ).toBeNull();
      });
    }
  });

  it('keeps the library panels on both pages, calling the library routes', async () => {
    const before = loggedPaths().length;
    await renderMovie(movieBody(), async (view) => {
      expect(await view.findByTestId('title-panel')).toBeTruthy();
      expect(view.getByTestId('title-watchlist-add')).toBeTruthy();
    });
    await renderSeries(seriesBody(), async (view) => {
      expect(await view.findByTestId('title-panel')).toBeTruthy();
      expect(view.getByTestId('title-watchlist-add')).toBeTruthy();
      expect(view.getByTestId('season-row-1')).toBeTruthy();
    });
    const called = loggedPaths().slice(before);
    expect(called).toContain('/api/v1/library/title/movie/603');
    expect(called).toContain('/api/v1/library/title/tv/249042');
  });

  it('renders and imports nothing request-, issue-, media-server- or arr-related', async () => {
    const forbiddenText =
      /Request|Report an Issue|Manage (Movie|Series)|Play on|Play 4K|Radarr|Sonarr|Download Status|Mark as Available|Rotten Tomatoes|IMDB User Score|Blocklist/;
    const before = loggedPaths().length;
    await renderMovie(movieBody(), (view) => {
      expect(view.container.textContent).not.toMatch(forbiddenText);
    });
    await renderSeries(seriesBody(), (view) => {
      expect(view.container.textContent).not.toMatch(forbiddenText);
    });
    for (const path of loggedPaths().slice(before)) {
      expect(path).not.toMatch(/\/ratings(combined)?$|\/request|\/issue/);
    }

    const forbiddenImport =
      /RequestButton|RequestModal|IssueModal|ManageSlideOver|BlocklistModal|StatusBadge|useDeepLinks|constants\/server|constants\/issue|api\/ratings|api\/rottentomatoes|refreshIntervalHelper|PWA/;
    for (const page of ['MovieDetails', 'TvDetails']) {
      const source = readFileSync(
        join(here, '..', 'components', page, 'index.tsx'),
        'utf8'
      );
      const imports = source
        .split('\n')
        .filter((line) => /^import |from '/.test(line));
      expect(imports.join('\n')).not.toMatch(forbiddenImport);
    }
  });
});
