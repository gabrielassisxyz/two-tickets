import type {
  LibraryMediaType,
  LibraryTarget,
} from '@app/components/Library/libraryTypes';

// The scope a control acts on: a movie is always "movie"; a series is
// "show" for the whole series and "season" for one season row. Season 0
// (specials) is a season like any other, so the number is checked against
// undefined, never for truthiness.
export const libraryTargetFor = (
  mediaType: LibraryMediaType,
  tmdbId: number,
  seasonNumber?: number
): LibraryTarget => {
  if (mediaType === 'movie') {
    return { mediaType, tmdbId, scope: 'movie' };
  }
  if (seasonNumber === undefined) {
    return { mediaType, tmdbId, scope: 'show' };
  }
  return { mediaType, tmdbId, scope: 'season', seasonNumber };
};
