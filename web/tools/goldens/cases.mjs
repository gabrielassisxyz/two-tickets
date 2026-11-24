// Every golden: the /api/v1 request the frontend makes, the route family it
// belongs to, and the fixtures that answer the TMDB calls upstream makes for it.
// A case's fixtures are listed in the order upstream first asks for them; the
// recorder relies on that order, the generator does not.
//
// Route families: discover (discover pages, sliders, trending, backdrops),
// title (movie, series, season, person and collection pages), search, and
// lookup (genres, languages, regions, studios, networks, keywords, watch
// providers, certifications).

const statusErrorCases = (family, name, url, kinds, fixtureFor) =>
  kinds.map((kind) => ({
    name: `${name}-tmdb-${kind}`,
    family,
    url,
    fixtures: [fixtureFor(kind)],
  }));

const ERROR_KINDS = ['404', '429', '500', 'timeout'];

const SLIDER_BACKDROP_COUNTS = [0, 1, 4, 5];

const discoverCases = [
  {
    name: 'discover-movies',
    url: '/api/v1/discover/movies?page=1',
    fixtures: ['discover/movies-popular'],
  },
  {
    name: 'discover-movies-adult-entry',
    url: '/api/v1/discover/movies?page=1',
    fixtures: ['discover/movies-popular-adult'],
  },
  {
    name: 'discover-movies-keywords',
    url: '/api/v1/discover/movies?page=1&keywords=9715',
    fixtures: ['discover/movies-keyword-superhero', 'keyword/superhero'],
  },
  {
    name: 'discover-movies-genre',
    url: '/api/v1/discover/movies/genre/18?page=1',
    fixtures: ['genre/movie-list', 'discover/movies-genre-drama'],
  },
  {
    name: 'discover-movies-studio',
    url: '/api/v1/discover/movies/studio/2?page=1',
    fixtures: ['company/walt-disney-pictures', 'discover/movies-studio-disney'],
  },
  {
    name: 'discover-movies-language',
    url: '/api/v1/discover/movies/language/pt?page=1',
    fixtures: ['configuration/languages', 'discover/movies-language-pt'],
  },
  {
    name: 'discover-movies-upcoming',
    url: '/api/v1/discover/movies/upcoming?page=1',
    fixtures: ['discover/movies-upcoming'],
  },
  {
    name: 'discover-tv',
    url: '/api/v1/discover/tv?page=1',
    fixtures: ['discover/tv-popular'],
  },
  {
    name: 'discover-tv-genre',
    url: '/api/v1/discover/tv/genre/16?page=1',
    fixtures: ['genre/tv-list', 'discover/tv-genre-animation'],
  },
  {
    name: 'discover-tv-network',
    url: '/api/v1/discover/tv/network/213?page=1',
    fixtures: ['network/netflix', 'discover/tv-network-netflix'],
  },
  {
    name: 'discover-tv-language',
    url: '/api/v1/discover/tv/language/ja?page=1',
    fixtures: ['configuration/languages', 'discover/tv-language-ja'],
  },
  {
    name: 'discover-tv-upcoming',
    url: '/api/v1/discover/tv/upcoming?page=1',
    fixtures: ['discover/tv-upcoming'],
  },
  {
    name: 'discover-trending',
    url: '/api/v1/discover/trending?page=1',
    fixtures: ['trending/all-day'],
  },
  {
    name: 'discover-trending-four-kinds',
    url: '/api/v1/discover/trending?page=1',
    fixtures: ['trending/all-day-four-kinds'],
  },
  {
    name: 'discover-trending-adult-entry',
    url: '/api/v1/discover/trending?page=1',
    fixtures: ['trending/all-day-adult'],
  },
  {
    name: 'discover-trending-movies',
    url: '/api/v1/discover/trending?page=1&mediaType=movie',
    fixtures: ['trending/movie-day'],
  },
  {
    name: 'discover-trending-tv',
    url: '/api/v1/discover/trending?page=1&mediaType=tv',
    fixtures: ['trending/tv-day'],
  },
  {
    name: 'discover-keyword-movies',
    url: '/api/v1/discover/keyword/9715/movies?page=1',
    fixtures: ['keyword/superhero-movies'],
  },
  {
    name: 'discover-genreslider-movie',
    url: '/api/v1/discover/genreslider/movie',
    fixtures: [
      'genre/movie-list-slider',
      ...SLIDER_BACKDROP_COUNTS.map(
        (count) => `discover/movies-slider-backdrops-${count}`
      ),
    ],
  },
  {
    name: 'discover-genreslider-tv',
    url: '/api/v1/discover/genreslider/tv',
    fixtures: [
      'genre/tv-list-slider',
      ...SLIDER_BACKDROP_COUNTS.map(
        (count) => `discover/tv-slider-backdrops-${count}`
      ),
    ],
  },
  {
    name: 'backdrops',
    url: '/api/v1/backdrops',
    fixtures: ['trending/all-week'],
  },
  {
    name: 'backdrops-adult-entry',
    url: '/api/v1/backdrops',
    fixtures: ['trending/all-week-adult'],
  },
  ...statusErrorCases(
    'discover',
    'discover-movies',
    '/api/v1/discover/movies?page=1',
    ERROR_KINDS,
    (kind) => `errors/discover-movies-${kind}`
  ),
];

const movieCases = [
  {
    name: 'movie-details-full',
    url: '/api/v1/movie/603',
    fixtures: ['movie/details-full'],
  },
  {
    name: 'movie-details-without-local-trailer',
    url: '/api/v1/movie/26564',
    fixtures: [
      'movie/details-without-local-trailer',
      'movie/videos-en-without-local-trailer',
    ],
  },
  {
    name: 'movie-details-empty-overview',
    url: '/api/v1/movie/11441',
    fixtures: [
      'movie/details-empty-overview',
      'movie/videos-en-empty-overview',
      'movie/details-en-empty-overview',
    ],
  },
  {
    name: 'movie-details-minimal',
    url: '/api/v1/movie/485013',
    fixtures: [
      'movie/details-minimal',
      'movie/videos-en-minimal',
      'movie/details-en-minimal',
    ],
  },
  {
    name: 'movie-details-empty',
    url: '/api/v1/movie/485013',
    fixtures: [
      'movie/details-empty',
      'movie/videos-en-empty',
      'movie/details-en-empty',
    ],
  },
  {
    name: 'movie-details-adult',
    url: '/api/v1/movie/485013',
    fixtures: [
      'movie/details-adult',
      'movie/videos-en-minimal',
      'movie/details-en-minimal',
    ],
  },
  {
    name: 'movie-details-not-found',
    url: '/api/v1/movie/999999999',
    fixtures: ['movie/details-not-found'],
  },
  {
    name: 'movie-details-non-numeric-id',
    url: '/api/v1/movie/abc',
    fixtures: [],
  },
  {
    name: 'movie-recommendations',
    url: '/api/v1/movie/603/recommendations?page=1',
    fixtures: ['movie/recommendations'],
  },
  {
    name: 'movie-recommendations-adult-entry',
    url: '/api/v1/movie/603/recommendations?page=1',
    fixtures: ['movie/recommendations-adult'],
  },
  {
    name: 'movie-similar',
    url: '/api/v1/movie/603/similar?page=1',
    fixtures: ['movie/similar'],
  },
  {
    name: 'movie-similar-adult-entry',
    url: '/api/v1/movie/603/similar?page=1',
    fixtures: ['movie/similar-adult'],
  },
];

const tvCases = [
  {
    name: 'tv-details-full',
    url: '/api/v1/tv/249042',
    fixtures: ['tv/details-en-full', 'tv/details-full'],
  },
  {
    name: 'tv-details-without-local-trailer',
    url: '/api/v1/tv/241554',
    fixtures: [
      'tv/details-en-without-local-trailer',
      'tv/details-without-local-trailer',
      'tv/videos-en-without-local-trailer',
    ],
  },
  {
    name: 'tv-details-anime',
    url: '/api/v1/tv/97009',
    fixtures: ['tv/details-en-anime', 'tv/details-anime'],
  },
  {
    name: 'tv-details-minimal',
    url: '/api/v1/tv/91073',
    fixtures: [
      'tv/details-en-minimal',
      'tv/details-minimal',
      'tv/videos-en-minimal',
    ],
  },
  {
    name: 'tv-details-empty-overview',
    url: '/api/v1/tv/241462',
    fixtures: [
      'tv/details-en-empty-overview',
      'tv/details-empty-overview',
      'tv/videos-en-empty-overview',
    ],
  },
  {
    name: 'tv-details-empty',
    url: '/api/v1/tv/91073',
    fixtures: ['tv/details-en-empty', 'tv/details-empty', 'tv/videos-en-empty'],
  },
  {
    name: 'tv-season-with-stills',
    url: '/api/v1/tv/249042/season/1',
    fixtures: ['tv/details-en-full', 'tv/season-with-stills'],
  },
  {
    name: 'tv-season-null-stills',
    url: '/api/v1/tv/125164/season/0',
    fixtures: ['tv/details-en-null-stills', 'tv/season-null-stills'],
  },
  {
    name: 'tv-season-empty',
    url: '/api/v1/tv/125164/season/0',
    fixtures: ['tv/details-en-null-stills', 'tv/season-empty'],
  },
  {
    name: 'tv-recommendations',
    url: '/api/v1/tv/249042/recommendations?page=1',
    fixtures: ['tv/recommendations'],
  },
  {
    name: 'tv-recommendations-adult-entry',
    url: '/api/v1/tv/249042/recommendations?page=1',
    fixtures: ['tv/recommendations-adult'],
  },
  {
    name: 'tv-similar',
    url: '/api/v1/tv/249042/similar?page=1',
    fixtures: ['tv/similar'],
  },
  {
    name: 'tv-similar-adult-entry',
    url: '/api/v1/tv/249042/similar?page=1',
    fixtures: ['tv/similar-adult'],
  },
];

const personAndCollectionCases = [
  {
    name: 'person-details-full',
    url: '/api/v1/person/52583',
    fixtures: ['person/details-full'],
  },
  {
    name: 'person-details-minimal',
    url: '/api/v1/person/5153308',
    fixtures: ['person/details-minimal'],
  },
  {
    name: 'person-details-empty',
    url: '/api/v1/person/5153308',
    fixtures: ['person/details-empty'],
  },
  {
    name: 'person-combined-credits',
    url: '/api/v1/person/5225942/combined_credits',
    fixtures: ['person/combined-credits'],
  },
  {
    name: 'person-combined-credits-adult-credit',
    url: '/api/v1/person/5225942/combined_credits',
    fixtures: ['person/combined-credits-adult-credit'],
  },
  {
    name: 'collection-full',
    url: '/api/v1/collection/2344',
    fixtures: ['collection/full'],
  },
  {
    name: 'collection-minimal',
    url: '/api/v1/collection/1620352',
    fixtures: ['collection/minimal'],
  },
  {
    name: 'collection-empty',
    url: '/api/v1/collection/1620352',
    fixtures: ['collection/empty'],
  },
  {
    name: 'collection-adult-part',
    url: '/api/v1/collection/2344',
    fixtures: ['collection/adult-part'],
  },
  {
    name: 'collection-missing-and-equal-release-dates',
    url: '/api/v1/collection/2344',
    fixtures: ['collection/missing-and-equal-release-dates'],
  },
];

const titleErrorCases = [
  ...statusErrorCases(
    'title',
    'movie-details',
    '/api/v1/movie/603',
    ['429', '500', 'timeout'],
    (kind) => `errors/movie-details-${kind}`
  ),
  // A 404 from TMDB is recorded for real, on an id that does not exist.
  {
    name: 'movie-details-tmdb-404',
    url: '/api/v1/movie/999999999',
    fixtures: ['movie/details-not-found'],
  },
];

const searchCases = [
  {
    name: 'search-multi',
    url: '/api/v1/search?query=cooper&page=1',
    fixtures: ['search/multi-cooper'],
  },
  {
    name: 'search-multi-four-kinds',
    url: '/api/v1/search?query=cooper&page=1',
    fixtures: ['search/multi-four-kinds'],
  },
  {
    name: 'search-multi-adult-entry',
    url: '/api/v1/search?query=cooper&page=1',
    fixtures: ['search/multi-adult'],
  },
  {
    // Partial success: the movie exists under this id, the series and the person do not.
    name: 'search-tmdb-prefix',
    url: '/api/v1/search?query=tmdb%3A485013',
    fixtures: [
      'movie/details-minimal',
      'search/tmdb-prefix-tv-not-found',
      'search/tmdb-prefix-person-not-found',
      'movie/videos-en-minimal',
    ],
  },
  {
    name: 'search-tmdb-prefix-invalid-id',
    url: '/api/v1/search?query=tmdb%3A999999999',
    fixtures: [
      'movie/details-not-found',
      'search/tmdb-prefix-tv-not-found-invalid-id',
      'search/tmdb-prefix-person-not-found-invalid-id',
    ],
  },
  {
    name: 'search-imdb-prefix',
    url: '/api/v1/search?query=imdb%3Att0133093',
    fixtures: ['search/find-imdb'],
  },
  {
    name: 'search-tvdb-prefix',
    url: '/api/v1/search?query=tvdb%3A81189',
    fixtures: ['search/find-tvdb'],
  },
  {
    name: 'search-year-prefix',
    url: '/api/v1/search?query=friends%20year%3A1994',
    fixtures: ['search/year-prefix-movies', 'search/year-prefix-tv'],
  },
  {
    // Partial success: the series search fails and the movie results still come back.
    name: 'search-year-prefix-tv-fails',
    url: '/api/v1/search?query=friends%20year%3A1994',
    fixtures: ['search/year-prefix-movies', 'errors/search-year-prefix-tv-500'],
  },
  {
    name: 'search-year-field-only',
    url: '/api/v1/search?query=year%3A1999',
    fixtures: ['search/year-only-movies', 'search/year-only-tv'],
  },
  { name: 'search-missing-query', url: '/api/v1/search', fixtures: [] },
  {
    name: 'search-keyword',
    url: '/api/v1/search/keyword?query=anime&page=1',
    fixtures: ['search/keyword-anime'],
  },
  {
    name: 'search-company',
    url: '/api/v1/search/company?query=marvel&page=1',
    fixtures: ['search/company-marvel'],
  },
  ...statusErrorCases(
    'search',
    'search-multi',
    '/api/v1/search?query=cooper&page=1',
    ERROR_KINDS,
    (kind) => `errors/search-multi-${kind}`
  ),
];

const lookupCases = [
  {
    name: 'regions',
    url: '/api/v1/regions',
    fixtures: ['configuration/countries'],
  },
  {
    name: 'languages',
    url: '/api/v1/languages',
    fixtures: ['configuration/languages'],
  },
  {
    name: 'studio',
    url: '/api/v1/studio/2',
    fixtures: ['company/walt-disney-pictures'],
  },
  {
    name: 'network',
    url: '/api/v1/network/213',
    fixtures: ['network/netflix'],
  },
  {
    name: 'genres-movie',
    url: '/api/v1/genres/movie',
    fixtures: ['genre/movie-list'],
  },
  { name: 'genres-tv', url: '/api/v1/genres/tv', fixtures: ['genre/tv-list'] },
  {
    name: 'keyword',
    url: '/api/v1/keyword/9715',
    fixtures: ['keyword/superhero'],
  },
  {
    name: 'watchproviders-regions',
    url: '/api/v1/watchproviders/regions',
    fixtures: ['watch-providers/regions'],
  },
  {
    name: 'watchproviders-movies',
    url: '/api/v1/watchproviders/movies?watchRegion=BR',
    fixtures: ['watch-providers/movie-br'],
  },
  {
    name: 'watchproviders-tv',
    url: '/api/v1/watchproviders/tv?watchRegion=BR',
    fixtures: ['watch-providers/tv-br'],
  },
  {
    name: 'certifications-movie',
    url: '/api/v1/certifications/movie',
    fixtures: ['certification/movie'],
  },
  {
    name: 'certifications-tv',
    url: '/api/v1/certifications/tv',
    fixtures: ['certification/tv'],
  },
  ...statusErrorCases(
    'lookup',
    'genres-movie',
    '/api/v1/genres/movie',
    ERROR_KINDS,
    (kind) => `errors/genres-movie-${kind}`
  ),
];

const inFamily = (family, cases) =>
  cases.map((entry) => ({ family, ...entry }));

export const CASES = [
  ...inFamily('discover', discoverCases),
  ...inFamily('title', [
    ...movieCases,
    ...tvCases,
    ...personAndCollectionCases,
    ...titleErrorCases,
  ]),
  ...inFamily('search', searchCases),
  ...inFamily('lookup', lookupCases),
];
