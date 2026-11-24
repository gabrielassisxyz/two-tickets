// The settings the goldens are produced under. Upstream's defaults, except the
// discover region, which this project fixes to BR. The display locale pt-BR is
// the signed-in user's own setting (see stubs/datasource.mjs), so the server
// default stays upstream's 'en' and the empty-overview fallback still asks TMDB
// for English, as it does in a stock install.
export const MetadataProviderType = {
  TMDB: 'tmdb',
  TVDB: 'tvdb',
};

const settings = {
  main: {
    apiKey: '',
    locale: 'en',
    discoverRegion: 'BR',
    streamingRegion: '',
    originalLanguage: '',
    blocklistRegion: '',
    blocklistLanguage: '',
  },
  metadataSettings: {
    tv: MetadataProviderType.TMDB,
    anime: MetadataProviderType.TMDB,
  },
  network: { csrfProtection: false, trustProxy: false },
  fullPublicSettings: { versionCheck: false },
};

export const getSettings = () => settings;
