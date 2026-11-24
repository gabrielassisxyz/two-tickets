// The shapes /api/v1/library/* answers, as internal/library and
// internal/hydrate write them. Only the fields the library UI reads.
import type { MovieResult, TvResult } from '@app/compat/models/Search';

export type LibraryMediaType = 'movie' | 'tv';

export type LibraryScope = 'movie' | 'show' | 'season';

export type RatingLevel =
  | 'hated'
  | 'disliked'
  | 'meh'
  | 'fine'
  | 'good'
  | 'liked'
  | 'loved';

export interface LibraryRating {
  level: RatingLevel | null;
  review: string | null;
  updatedAt: string;
}

export interface LibrarySeasonState {
  seasonNumber: number;
  watched: boolean;
  watchedAt: string | null;
  rating: LibraryRating | null;
}

export interface LibraryPersonState {
  displayName: string;
  status: 'watched' | 'in_progress' | 'unwatched';
  watched: boolean;
  watchedAt: string | null;
  rating: LibraryRating | null;
  regularSeasonsWatched: number;
  seasons: LibrarySeasonState[];
}

export interface LibraryListMembership {
  id: number;
  kind: LibraryListKind;
  name: string;
}

export interface LibraryTitleState {
  mediaType: LibraryMediaType;
  tmdbId: number;
  me: LibraryPersonState;
  partner: LibraryPersonState;
  lists: LibraryListMembership[];
}

export type LibraryListKind = 'watchlist' | 'couple' | 'custom';

export interface LibraryList {
  id: number;
  kind: LibraryListKind;
  name: string;
  owner: 'me' | 'partner' | 'shared';
  itemCount: number;
  createdAt: string;
  canRename: boolean;
  canChangeItems: boolean;
}

export interface LibraryCard {
  mediaType: LibraryMediaType;
  tmdbId: number;
  available: boolean;
  result: MovieResult | TvResult;
}

export interface LibraryListItem extends LibraryCard {
  addedAt: string;
  addedByMe: boolean;
  note: string | null;
}

export interface LibraryHistoryRow extends LibraryCard {
  scope: LibraryScope;
  seasonNumber: number | null;
  activityAt: string;
  watched: boolean;
  watchedAt: string | null;
  rating: LibraryRating | null;
}

export interface LibraryCursorPage<T> {
  results: T[];
  nextCursor: string | null;
}

// What a watch or a rating applies to: a movie, a whole show, or one season.
export interface LibraryTarget {
  mediaType: LibraryMediaType;
  tmdbId: number;
  scope: LibraryScope;
  seasonNumber?: number;
}
