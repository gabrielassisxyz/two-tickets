// Every TMDB fixture under testdata/tmdb/, and what it stands for.
//
// A recorded fixture is a real TMDB response, captured once by record.mjs and
// committed. A derived fixture is computed from recorded ones by the edit named
// in its `derive` entry, for the cases TMDB will not produce on demand: adult
// entries (TMDB hides them behind include_adult=false), empty records, the
// result kinds a given endpoint rarely mixes, and 429, 5xx and timeout
// failures. Derived fixtures are committed too, and the generator's --check
// fails when one no longer matches what its derivation produces.

const TMDB_ERROR_BODIES = {
  404: {
    success: false,
    status_code: 34,
    status_message: 'The resource you requested could not be found.',
  },
  429: {
    success: false,
    status_code: 25,
    status_message: 'Your request count (#) is over the allowed limit of (40).',
  },
  500: {
    success: false,
    status_code: 11,
    status_message: 'Internal error: Something went wrong, contact TMDB.',
  },
};

export const tmdbFailure = (kind) =>
  kind === 'timeout'
    ? { error: 'timeout' }
    : { status: Number(kind), body: TMDB_ERROR_BODIES[kind] };

const ERROR_DESCRIPTIONS = {
  404: 'TMDB answers 404 Not Found',
  429: 'TMDB answers 429 Too Many Requests',
  500: 'TMDB answers 500 Internal Server Error',
  timeout: 'the connection to TMDB times out (ETIMEDOUT)',
};

const errorFixtures = (prefix, from, what) =>
  ['404', '429', '500', 'timeout'].map((kind) => ({
    id: `errors/${prefix}-${kind}`,
    category: `TMDB error: ${what}, ${ERROR_DESCRIPTIONS[kind]}`,
    derive: { from, response: tmdbFailure(kind) },
  }));

// Replaces only the keys the base body has, so a narrow response (the
// English trailer lookup) stays narrow.
const emptied = (body, emptyValues) => {
  const result = { ...body };
  for (const [key, value] of Object.entries(emptyValues)) {
    if (Object.hasOwn(body, key)) {
      result[key] = structuredClone(value);
    }
  }
  return result;
};

const EMPTY_EXTERNAL_IDS = {
  imdb_id: null,
  wikidata_id: null,
  facebook_id: null,
  instagram_id: null,
  twitter_id: null,
};

const emptyMovie = (body) =>
  emptied(body, {
    backdrop_path: null,
    belongs_to_collection: null,
    budget: 0,
    genres: [],
    homepage: '',
    imdb_id: null,
    origin_country: [],
    overview: '',
    poster_path: null,
    production_companies: [],
    production_countries: [],
    revenue: 0,
    runtime: 0,
    spoken_languages: [],
    tagline: '',
    credits: { cast: [], crew: [] },
    external_ids: { id: body.id, ...EMPTY_EXTERNAL_IDS },
    videos: { results: [] },
    keywords: { keywords: [] },
    release_dates: { results: [] },
    'watch/providers': { results: {} },
  });

const emptyTv = (body) =>
  emptied(body, {
    backdrop_path: null,
    created_by: [],
    episode_run_time: [],
    genres: [],
    homepage: '',
    languages: [],
    last_episode_to_air: null,
    next_episode_to_air: null,
    networks: [],
    number_of_episodes: 0,
    number_of_seasons: 0,
    origin_country: [],
    overview: '',
    poster_path: null,
    production_companies: [],
    production_countries: [],
    seasons: [],
    spoken_languages: [],
    tagline: '',
    aggregate_credits: { cast: [], crew: [] },
    credits: { cast: [], crew: [] },
    external_ids: { id: body.id, ...EMPTY_EXTERNAL_IDS, tvdb_id: null, tvrage_id: null },
    keywords: { results: [] },
    videos: { results: [] },
    content_ratings: { results: [] },
    'watch/providers': { results: {} },
  });

const withFirstResultAdult = (body) => ({
  ...body,
  results: body.results.map((result, index) =>
    index === 0 ? { ...result, adult: true } : result
  ),
});

const collectionAsResult = (collection) => ({
  adult: false,
  backdrop_path: collection.backdrop_path,
  id: collection.id,
  media_type: 'collection',
  original_language: 'en',
  original_title: collection.name,
  overview: collection.overview,
  poster_path: collection.poster_path,
  title: collection.name,
});

// The genre slider asks for one discover page per genre; each derived page
// keeps the first five results of a recorded page and clears the backdrop of
// all but `count` of them.
const sliderPage = (count) => (body) => ({
  ...body,
  results: body.results
    .slice(0, 5)
    .map((result, index) => (index < count ? result : { ...result, backdrop_path: null })),
});

const SLIDER_GENRES = {
  movie: [
    { id: 28, count: 0 },
    { id: 12, count: 1 },
    { id: 16, count: 4 },
    { id: 35, count: 5 },
  ],
  tv: [
    { id: 10759, count: 0 },
    { id: 16, count: 1 },
    { id: 35, count: 4 },
    { id: 80, count: 5 },
  ],
};

const sliderFixtures = (kind, listId, pageId) => [
  {
    id: `genre/${kind}-list-slider`,
    category: `genre slider: the ${kind} genre list cut to four genres`,
    derive: {
      from: listId,
      body: (body) => ({
        genres: body.genres.filter((genre) =>
          SLIDER_GENRES[kind].some((slider) => slider.id === genre.id)
        ),
      }),
    },
  },
  ...SLIDER_GENRES[kind].map(({ id, count }) => ({
    id: `discover/${kind === 'movie' ? 'movies' : 'tv'}-slider-backdrops-${count}`,
    category: `genre slider: a ${kind} genre with ${count} valid backdrop${count === 1 ? '' : 's'}`,
    derive: {
      from: pageId,
      // The slider's TheMovieDb has no discover region and the server locale.
      request: (request) => {
        const params = { ...request.params, with_genres: String(id), language: 'en', region: '' };
        delete params.with_original_language;
        return { path: request.path, params };
      },
      body: sliderPage(count),
    },
  })),
];

const recorded = (id, category) => ({ id, category });

const discoverFixtures = [
  recorded('discover/movies-popular', 'discover: movies, first page'),
  {
    id: 'discover/movies-popular-adult',
    category: 'adult: an adult entry in discover movies',
    derive: { from: 'discover/movies-popular', body: withFirstResultAdult },
  },
  recorded('discover/movies-keyword-superhero', 'discover: movies filtered by a keyword'),
  recorded('keyword/superhero', 'lookup: keyword details'),
  recorded('genre/movie-list', 'lookup: movie genre list (pt-BR)'),
  recorded('discover/movies-genre-drama', 'discover: movies of one genre'),
  recorded('company/walt-disney-pictures', 'lookup: studio details'),
  recorded('discover/movies-studio-disney', 'discover: movies of one studio'),
  recorded('configuration/languages', 'lookup: TMDB language list'),
  recorded('discover/movies-language-pt', 'discover: movies in one original language'),
  recorded('discover/movies-upcoming', 'discover: upcoming movies from the recording date'),
  recorded('discover/tv-popular', 'discover: series, first page'),
  recorded('genre/tv-list', 'lookup: series genre list (pt-BR)'),
  recorded('discover/tv-genre-animation', 'discover: series of one genre'),
  recorded('network/netflix', 'lookup: network details'),
  recorded('discover/tv-network-netflix', 'discover: series of one network'),
  recorded('discover/tv-language-ja', 'discover: series in one original language'),
  recorded('discover/tv-upcoming', 'discover: upcoming series from the recording date'),
  recorded('trending/all-day', 'trending: all kinds, day window'),
  {
    id: 'trending/all-day-four-kinds',
    category: 'result kinds: trending holding a movie, a series, a person and a collection',
    derive: {
      from: 'trending/all-day',
      with: ['search/multi-cooper', 'collection/full'],
      body: (body, [search, collection]) => {
        const person = search.results.find((result) => result.media_type === 'person');
        const [first, second, ...rest] = body.results;
        return { ...body, results: [first, second, person, collectionAsResult(collection), ...rest] };
      },
    },
  },
  {
    id: 'trending/all-day-adult',
    category: 'adult: an adult entry in trending',
    derive: { from: 'trending/all-day', body: withFirstResultAdult },
  },
  recorded('trending/movie-day', 'trending: movies, day window'),
  recorded('trending/tv-day', 'trending: series, day window'),
  recorded('keyword/superhero-movies', 'discover: movies of one keyword'),
  ...sliderFixtures('movie', 'genre/movie-list', 'discover/movies-genre-drama'),
  ...sliderFixtures('tv', 'genre/tv-list', 'discover/tv-genre-animation'),
  recorded('trending/all-week', 'backdrops: trending of the week'),
  {
    id: 'trending/all-week-adult',
    category: 'adult: an adult entry behind /backdrops',
    derive: { from: 'trending/all-week', body: withFirstResultAdult },
  },
  ...errorFixtures('discover-movies', 'discover/movies-popular', 'discover movies'),
];

const movieFixtures = [
  recorded('movie/details-full', 'movie details, full: collection, pt-BR trailers, watch providers, release dates'),
  recorded('movie/details-without-local-trailer', 'movie details without a pt-BR trailer'),
  recorded('movie/videos-en-without-local-trailer', 'movie English trailer lookup, for a title without a pt-BR trailer'),
  recorded('movie/details-empty-overview', 'movie details with an empty pt-BR overview'),
  recorded('movie/videos-en-empty-overview', 'movie English trailer lookup, for the empty-overview title'),
  recorded('movie/details-en-empty-overview', 'movie details in English, the overview fallback'),
  recorded('movie/details-minimal', 'movie details, minimal: no poster, overview, collection or trailer'),
  recorded('movie/videos-en-minimal', 'movie English trailer lookup, for the minimal title'),
  recorded('movie/details-en-minimal', 'movie details in English, for the minimal title'),
  {
    id: 'movie/details-empty',
    category: 'movie details, empty: every list empty and every optional field null',
    derive: { from: 'movie/details-minimal', body: emptyMovie },
  },
  {
    id: 'movie/videos-en-empty',
    category: 'movie English trailer lookup, empty',
    derive: { from: 'movie/videos-en-minimal', body: emptyMovie },
  },
  {
    id: 'movie/details-en-empty',
    category: 'movie details in English, empty',
    derive: { from: 'movie/details-en-minimal', body: emptyMovie },
  },
  {
    id: 'movie/details-adult',
    category: 'adult: a direct adult movie title',
    derive: { from: 'movie/details-minimal', body: (body) => ({ ...body, adult: true }) },
  },
  recorded('movie/details-not-found', 'invalid id: movie details for an id TMDB does not have (404)'),
  recorded('movie/recommendations', 'movie recommendations'),
  {
    id: 'movie/recommendations-adult',
    category: 'adult: an adult entry in movie recommendations',
    derive: { from: 'movie/recommendations', body: withFirstResultAdult },
  },
  recorded('movie/similar', 'similar movies'),
  {
    id: 'movie/similar-adult',
    category: 'adult: an adult entry in similar movies',
    derive: { from: 'movie/similar', body: withFirstResultAdult },
  },
  // The title family's 404 is the recorded movie/details-not-found.
  ...errorFixtures('movie-details', 'movie/details-full', 'movie details').filter(
    (fixture) => !fixture.id.endsWith('-404')
  ),
];

const tvFixtures = [
  recorded('tv/details-en-full', 'series details in English, full: aggregate_credits, content ratings, networks'),
  recorded('tv/details-full', 'series details, full: aggregate_credits, pt-BR trailer, content ratings'),
  recorded('tv/details-en-without-local-trailer', 'series details in English, for a title without a pt-BR trailer'),
  recorded('tv/details-without-local-trailer', 'series details without a pt-BR trailer'),
  recorded('tv/videos-en-without-local-trailer', 'series English trailer lookup, for a title without a pt-BR trailer'),
  recorded('tv/details-en-anime', 'series details in English, anime keyword'),
  recorded('tv/details-anime', 'series details carrying the anime keyword (210024)'),
  recorded('tv/details-en-minimal', 'series details in English, minimal'),
  recorded('tv/details-minimal', 'series details, minimal: no overview, trailer or networks to speak of'),
  recorded('tv/videos-en-minimal', 'series English trailer lookup, for the minimal title'),
  recorded('tv/details-en-empty-overview', 'series details in English, the overview fallback'),
  recorded('tv/details-empty-overview', 'series details with an empty pt-BR overview'),
  recorded('tv/videos-en-empty-overview', 'series English trailer lookup, for the empty-overview title'),
  {
    id: 'tv/details-en-empty',
    category: 'series details in English, empty',
    derive: { from: 'tv/details-en-minimal', body: emptyTv },
  },
  {
    id: 'tv/details-empty',
    category: 'series details, empty: every list empty and every optional field null',
    derive: { from: 'tv/details-minimal', body: emptyTv },
  },
  {
    id: 'tv/videos-en-empty',
    category: 'series English trailer lookup, empty',
    derive: { from: 'tv/videos-en-minimal', body: emptyTv },
  },
  recorded('tv/season-with-stills', 'season whose episodes all have stills'),
  recorded('tv/details-en-null-stills', 'series details in English, for the season with null stills'),
  recorded('tv/season-null-stills', 'season whose episode stills are null'),
  recorded('tv/recommendations', 'series recommendations'),
  {
    id: 'tv/recommendations-adult',
    category: 'adult: an adult entry in series recommendations',
    derive: { from: 'tv/recommendations', body: withFirstResultAdult },
  },
  recorded('tv/similar', 'similar series'),
  {
    id: 'tv/similar-adult',
    category: 'adult: an adult entry in similar series',
    derive: { from: 'tv/similar', body: withFirstResultAdult },
  },
];

const personAndCollectionFixtures = [
  recorded('person/details-full', 'person details, full: pt-BR biography, birthday, place of birth'),
  recorded('person/details-minimal', 'person details, minimal: name and department only'),
  {
    id: 'person/details-empty',
    category: 'person details, empty: every optional field null or empty',
    derive: {
      from: 'person/details-minimal',
      body: (body) => ({
        ...body,
        also_known_as: [],
        biography: '',
        birthday: null,
        deathday: null,
        homepage: null,
        imdb_id: null,
        known_for_department: '',
        place_of_birth: null,
        profile_path: null,
      }),
    },
  },
  recorded('person/combined-credits', 'person combined credits'),
  {
    id: 'person/combined-credits-adult-credit',
    category: 'adult: a non-adult person with an adult credit',
    derive: {
      from: 'person/combined-credits',
      body: (body) => ({
        ...body,
        cast: body.cast.map((credit, index) => (index === 0 ? { ...credit, adult: true } : credit)),
      }),
    },
  },
  recorded('collection/full', 'collection details, full: overview, artwork, several parts'),
  recorded('collection/minimal', 'collection details, minimal: no overview or backdrop, parts out of date order'),
  {
    id: 'collection/empty',
    category: 'collection details, empty: no parts and no artwork',
    derive: {
      from: 'collection/minimal',
      body: (body) => ({ ...body, overview: '', poster_path: null, backdrop_path: null, parts: [] }),
    },
  },
  {
    id: 'collection/adult-part',
    category: 'adult: a collection with one adult part',
    derive: {
      from: 'collection/full',
      body: (body) => ({
        ...body,
        parts: body.parts.map((part, index) => (index === 1 ? { ...part, adult: true } : part)),
      }),
    },
  },
  {
    id: 'collection/missing-and-equal-release-dates',
    category: 'collection with one part missing its release date and two parts sharing one',
    derive: {
      from: 'collection/full',
      body: (body) => {
        const parts = structuredClone(body.parts);
        parts[2].release_date = parts[1].release_date;
        parts[3].release_date = '';
        return { ...body, parts };
      },
    },
  },
];

const searchFixtures = [
  recorded('search/multi-cooper', 'search: multi search mixing movies, series and people'),
  {
    id: 'search/multi-four-kinds',
    category: 'result kinds: multi search holding a movie, a series, a person and a collection',
    derive: {
      from: 'search/multi-cooper',
      with: ['collection/full'],
      body: (body, [collection]) => ({
        ...body,
        results: [body.results[0], collectionAsResult(collection), ...body.results.slice(1)],
      }),
    },
  },
  {
    id: 'search/multi-adult',
    category: 'adult: an adult entry in search',
    derive: { from: 'search/multi-cooper', body: withFirstResultAdult },
  },
  recorded('search/tmdb-prefix-tv-not-found', 'search prefix tmdb:, the series lookup that fails (404)'),
  recorded('search/tmdb-prefix-person-not-found', 'search prefix tmdb:, the person lookup that fails (404)'),
  recorded('search/tmdb-prefix-tv-not-found-invalid-id', 'invalid id: tmdb: prefix, series lookup (404)'),
  recorded('search/tmdb-prefix-person-not-found-invalid-id', 'invalid id: tmdb: prefix, person lookup (404)'),
  recorded('search/find-imdb', 'search prefix imdb:, find by IMDb id'),
  recorded('search/find-tvdb', 'search prefix tvdb:, find by TVDB id'),
  recorded('search/year-prefix-movies', 'search prefix year:, the movie search'),
  recorded('search/year-prefix-tv', 'search prefix year:, the series search'),
  {
    id: 'errors/search-year-prefix-tv-500',
    category: 'search prefix year: with partial success, the series search failing (500)',
    derive: { from: 'search/year-prefix-tv', response: tmdbFailure('500') },
  },
  recorded('search/year-only-movies', 'search on a year field alone, the movie search'),
  recorded('search/year-only-tv', 'search on a year field alone, the series search'),
  recorded('search/keyword-anime', 'search: keywords'),
  recorded('search/company-marvel', 'search: companies'),
  ...errorFixtures('search-multi', 'search/multi-cooper', 'multi search'),
];

const lookupFixtures = [
  recorded('configuration/countries', 'lookup: TMDB country list'),
  recorded('watch-providers/regions', 'lookup: watch provider regions'),
  recorded('watch-providers/movie-br', 'lookup: movie watch providers in BR'),
  recorded('watch-providers/tv-br', 'lookup: series watch providers in BR'),
  recorded('certification/movie', 'lookup: movie certifications'),
  recorded('certification/tv', 'lookup: series certifications'),
  ...errorFixtures('genres-movie', 'genre/movie-list', 'movie genre list'),
];

export const FIXTURES = [
  ...discoverFixtures,
  ...movieFixtures,
  ...tvFixtures,
  ...personAndCollectionFixtures,
  ...searchFixtures,
  ...lookupFixtures,
];
