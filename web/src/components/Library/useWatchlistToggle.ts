import { setOnWatchlist } from '@app/components/Library/libraryRequests';
import type { LibraryMediaType } from '@app/components/Library/libraryTypes';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';

// The "Quero ver" star on cards, sliders and both title pages. Upstream
// flipped the star in a `finally`, so a rejected request still showed the
// title as added or removed; this goes through the optimistic hook, which
// restores the previous state when the request fails. `onCommitted` runs
// only after the api accepted the change (a slider of the watchlist drops
// the card then, and not before).
export const useWatchlistToggle = (
  mediaType: LibraryMediaType,
  tmdbId: number,
  title: string,
  onWatchlist: boolean,
  onCommitted?: (onWatchlist: boolean) => void
) => {
  const { value, pending, update } = useOptimisticLibraryValue(onWatchlist);

  const set = async (next: boolean) => {
    const committed = await update(next, (sent) =>
      setOnWatchlist(mediaType, tmdbId, title, sent)
    );
    if (committed && onCommitted) {
      onCommitted(next);
    }
    return committed;
  };

  return {
    onWatchlist: value,
    pending,
    add: () => set(true),
    remove: () => set(false),
    toggle: () => set(!value),
  };
};

export default useWatchlistToggle;
