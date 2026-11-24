// Every call the library UI makes to /api/v1/library/* and to the Seerr
// watchlist routes. Each mutation resolves on a 2xx and rejects otherwise,
// which is all the optimistic hook needs to commit or restore.
import type {
  LibraryMediaType,
  LibraryTarget,
  LibraryTitleState,
  RatingLevel,
} from '@app/components/Library/libraryTypes';
import axios from 'axios';
import { mutate } from 'swr';

export const libraryTitleKey = (mediaType: LibraryMediaType, tmdbId: number) =>
  `/api/v1/library/title/${mediaType}/${tmdbId}`;

export const libraryListsKey = '/api/v1/library/lists';

export const libraryListItemsKey = (listId: number) =>
  `/api/v1/library/lists/${listId}/items`;

// The answer to a watch or rating mutation is the title's new state; storing
// it keeps every control showing this title in agreement without a refetch.
const storeTitleState = (state: LibraryTitleState | undefined) => {
  if (state && state.mediaType && state.tmdbId) {
    mutate(libraryTitleKey(state.mediaType, state.tmdbId), state, false);
  }
};

export const putLibraryWatch = async (target: LibraryTarget) => {
  const response = await axios.put<LibraryTitleState>(
    '/api/v1/library/watch',
    target
  );
  storeTitleState(response.data);
};

export const deleteLibraryWatch = async (target: LibraryTarget) => {
  const response = await axios.delete<LibraryTitleState>(
    '/api/v1/library/watch',
    { data: target }
  );
  storeTitleState(response.data);
};

export const setLibraryWatched = (target: LibraryTarget, watched: boolean) =>
  watched ? putLibraryWatch(target) : deleteLibraryWatch(target);

export const putLibraryRatingLevel = async (
  target: LibraryTarget,
  level: RatingLevel | ''
) => {
  const response = await axios.put<LibraryTitleState>(
    '/api/v1/library/rating',
    { ...target, level }
  );
  storeTitleState(response.data);
};

export const putLibraryReview = async (
  target: LibraryTarget,
  review: string
) => {
  const response = await axios.put<LibraryTitleState>(
    '/api/v1/library/rating',
    { ...target, review }
  );
  storeTitleState(response.data);
};

export const deleteLibraryRating = async (target: LibraryTarget) => {
  const response = await axios.delete<LibraryTitleState>(
    '/api/v1/library/rating',
    { data: target }
  );
  storeTitleState(response.data);
};

// Clearing the level must not take a written review with it: with a review,
// only the level is cleared; without one, the rating row is deleted.
export const clearLibraryRating = (
  target: LibraryTarget,
  hasReview: boolean
) =>
  hasReview ? putLibraryRatingLevel(target, '') : deleteLibraryRating(target);

export const setLibraryRating = (
  target: LibraryTarget,
  level: RatingLevel | null,
  hasReview: boolean
) =>
  level === null
    ? clearLibraryRating(target, hasReview)
    : putLibraryRatingLevel(target, level);

const refreshTitle = (mediaType: LibraryMediaType, tmdbId: number) => {
  mutate(libraryTitleKey(mediaType, tmdbId));
  mutate(libraryListsKey);
};

export const putLibraryListItem = async (
  listId: number,
  mediaType: LibraryMediaType,
  tmdbId: number
) => {
  await axios.put(`/api/v1/library/lists/${listId}/items`, {
    mediaType,
    tmdbId,
  });
  mutate(libraryListItemsKey(listId));
  refreshTitle(mediaType, tmdbId);
};

export const deleteLibraryListItem = async (
  listId: number,
  mediaType: LibraryMediaType,
  tmdbId: number
) => {
  await axios.delete(
    `/api/v1/library/lists/${listId}/items/${mediaType}/${tmdbId}`
  );
  mutate(libraryListItemsKey(listId));
  refreshTitle(mediaType, tmdbId);
};

export const setLibraryListMembership = (
  listId: number,
  mediaType: LibraryMediaType,
  tmdbId: number,
  member: boolean
) =>
  member
    ? putLibraryListItem(listId, mediaType, tmdbId)
    : deleteLibraryListItem(listId, mediaType, tmdbId);

export const createLibraryList = async (name: string) => {
  await axios.post('/api/v1/library/lists', { name });
  mutate(libraryListsKey);
};

export const renameLibraryList = async (listId: number, name: string) => {
  await axios.patch(`/api/v1/library/lists/${listId}`, { name });
  mutate(libraryListsKey);
};

export const deleteLibraryList = async (listId: number) => {
  await axios.delete(`/api/v1/library/lists/${listId}`);
  mutate(libraryListsKey);
};

// The Seerr watchlist routes the kept stars call. Both are idempotent on the
// api: a repeated add answers 201 and a repeated remove 204.
export const addToWatchlist = async (
  mediaType: LibraryMediaType,
  tmdbId: number,
  title: string
) => {
  await axios.post('/api/v1/watchlist', { tmdbId, mediaType, title });
  refreshWatchlist(mediaType, tmdbId);
};

export const removeFromWatchlist = async (
  mediaType: LibraryMediaType,
  tmdbId: number
) => {
  await axios.delete(`/api/v1/watchlist/${tmdbId}?mediaType=${mediaType}`);
  refreshWatchlist(mediaType, tmdbId);
};

const refreshWatchlist = (mediaType: LibraryMediaType, tmdbId: number) => {
  mutate('/api/v1/discover/watchlist');
  refreshTitle(mediaType, tmdbId);
};

export const setOnWatchlist = (
  mediaType: LibraryMediaType,
  tmdbId: number,
  title: string,
  onWatchlist: boolean
) =>
  onWatchlist
    ? addToWatchlist(mediaType, tmdbId, title)
    : removeFromWatchlist(mediaType, tmdbId);
