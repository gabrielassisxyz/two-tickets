// @vitest-environment jsdom
// Renderer tests: every kept page and component renders against empty,
// minimal and full fixtures. Every axios call is recorded to
// web/tests/.runtime-requests.jsonl, which the api-inventory check diffs
// against the route manifest. Rendering a kept page must never request the
// dropped polling routes.
import { DiscoverSliderType } from '@app/compat/constants/discover';
import type { PublicSettingsResponse } from '@app/compat/interfaces/api/settingsInterfaces';
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
import MovieGenreList from '@app/components/Discover/MovieGenreList';
import Trending from '@app/components/Discover/Trending';
import TvGenreList from '@app/components/Discover/TvGenreList';
import Upcoming from '@app/components/Discover/Upcoming';
import GenreTag from '@app/components/GenreTag';
import KeywordTag from '@app/components/KeywordTag';
import LanguageSelector from '@app/components/LanguageSelector';
import UserDropdown from '@app/components/Layout/UserDropdown';
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
import { fireEvent, render, waitFor } from '@testing-library/react';
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
  // The api's fixed slider manifest: the nine kept built-ins, in its order.
  if (pathname === '/api/v1/settings/discover') {
    return [
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
  if (pathname === '/api/v1/backdrops') {
    return level === 'empty' ? [] : ['/backdrop.jpg'];
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
    fireEvent.click(add.container.querySelector('a') as HTMLAnchorElement);
    await waitFor(() => {
      expect(add.getByTitle('Quero ver')).toBeTruthy();
    });
    fireEvent.click(add.getByTitle('Quero ver'));
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
    fireEvent.click(remove.container.querySelector('a') as HTMLAnchorElement);
    await waitFor(() => {
      expect(remove.container.querySelector('button')).toBeTruthy();
    });
    fireEvent.click(
      remove.container.querySelector('button') as HTMLButtonElement
    );
    await waitFor(() => {
      expect(
        loggedPaths().filter((path) => path === '/api/v1/watchlist').length
      ).toBeGreaterThan(0);
    });
    remove.unmount();
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
