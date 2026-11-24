import { libraryTargetFor } from '@app/components/Library/libraryTarget';
import { describe, expect, it } from 'vitest';

describe('libraryTargetFor', () => {
  it('scopes a movie as movie, ignoring any season', () => {
    expect(libraryTargetFor('movie', 603)).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
    });
    expect(libraryTargetFor('movie', 603, 2)).toEqual({
      mediaType: 'movie',
      tmdbId: 603,
      scope: 'movie',
    });
  });

  it('scopes a series without a season as the whole show', () => {
    expect(libraryTargetFor('tv', 1399)).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'show',
    });
  });

  it('scopes a season row as that season, season 0 included', () => {
    expect(libraryTargetFor('tv', 1399, 0)).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'season',
      seasonNumber: 0,
    });
    expect(libraryTargetFor('tv', 1399, 3)).toEqual({
      mediaType: 'tv',
      tmdbId: 1399,
      scope: 'season',
      seasonNumber: 3,
    });
  });
});
