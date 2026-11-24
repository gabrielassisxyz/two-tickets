import { libraryTitleKey } from '@app/components/Library/libraryRequests';
import type {
  LibraryMediaType,
  LibraryPersonState,
  LibraryRating,
  LibraryTitleState,
} from '@app/components/Library/libraryTypes';
import { isRatingLevel } from '@app/components/Library/ratingLevels';
import useSWR from 'swr';

// Both people's state for one title, read only once `enabled` (a card asks
// when it scrolls into view). A missing or malformed answer reads as "nothing
// recorded" rather than breaking the card.
export const useLibraryTitle = (
  mediaType: LibraryMediaType,
  tmdbId: number,
  enabled = true
) => {
  const { data } = useSWR<LibraryTitleState>(
    enabled ? libraryTitleKey(mediaType, tmdbId) : null
  );
  const loaded = !!data && !!data.me && !!data.partner;
  return { state: loaded ? data : undefined };
};

export const ratingLevelOf = (rating: LibraryRating | null | undefined) =>
  isRatingLevel(rating?.level) ? rating.level : null;

export const reviewOf = (rating: LibraryRating | null | undefined) =>
  rating?.review ?? '';

export const seasonStateOf = (
  person: LibraryPersonState | undefined,
  seasonNumber: number
) => person?.seasons?.find((season) => season.seasonNumber === seasonNumber);

export const onWatchlistIn = (state: LibraryTitleState | undefined) =>
  state?.lists === undefined
    ? undefined
    : state.lists.some((list) => list.kind === 'watchlist');
