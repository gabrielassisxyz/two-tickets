import { libraryTargetFor } from '@app/components/Library/libraryTarget';
import type { LibraryMediaType } from '@app/components/Library/libraryTypes';
import RatingControl from '@app/components/Library/RatingControl';
import {
  onWatchlistIn,
  ratingLevelOf,
  reviewOf,
  useLibraryTitle,
} from '@app/components/Library/useLibraryTitle';
import useWatchlistToggle from '@app/components/Library/useWatchlistToggle';
import WatchedToggle from '@app/components/Library/WatchedToggle';
import defineMessages from '@app/utils/defineMessages';
import { StarIcon as StarOutline } from '@heroicons/react/24/outline';
import { StarIcon } from '@heroicons/react/24/solid';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.TitleCard.QuickActions', {
  addToWatchList: 'Add to watchlist',
  removeFromWatchList: 'Remove from watchlist',
});

interface QuickActionsProps {
  mediaType: LibraryMediaType;
  tmdbId: number;
  title: string;
  isAddedToWatchlist: boolean;
  inView: boolean;
  mutateParent?: () => void;
}

// The three actions every title card carries, always visible: there is no
// hover on a phone. Watched and the rating act on the movie, or on the whole
// series; "Quero ver" is the watchlist star.
const QuickActions = ({
  mediaType,
  tmdbId,
  title,
  isAddedToWatchlist,
  inView,
  mutateParent,
}: QuickActionsProps) => {
  const intl = useIntl();
  const { state } = useLibraryTitle(mediaType, tmdbId, inView);
  const target = libraryTargetFor(mediaType, tmdbId);
  const watchlist = useWatchlistToggle(
    mediaType,
    tmdbId,
    title,
    onWatchlistIn(state) ?? isAddedToWatchlist,
    (onWatchlist) => {
      if (!onWatchlist && mutateParent) {
        mutateParent();
      }
    }
  );
  const starLabel = intl.formatMessage(
    watchlist.onWatchlist
      ? messages.removeFromWatchList
      : messages.addToWatchList
  );
  const Star = watchlist.onWatchlist ? StarIcon : StarOutline;

  return (
    // The layer covers the whole poster so the rating picker can open over
    // it, and lets taps through everywhere but the bar itself.
    <div className="pointer-events-none absolute inset-0 z-40 flex flex-col justify-end">
      <div
        className="pointer-events-auto flex items-center justify-around gap-1 bg-gray-900/80 p-1.5"
        data-testid="quick-actions"
        onClick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <WatchedToggle
          target={target}
          watched={state?.me.watched ?? false}
          iconOnly
          testId="quick-watched"
        />
        <button
          type="button"
          data-testid="quick-watchlist"
          aria-pressed={watchlist.onWatchlist}
          aria-busy={watchlist.pending}
          aria-label={starLabel}
          title={starLabel}
          disabled={watchlist.pending}
          className={`inline-flex h-9 w-9 items-center justify-center rounded-md border transition duration-150 ease-out disabled:cursor-wait disabled:opacity-70 ${
            watchlist.onWatchlist
              ? 'border-indigo-500 bg-indigo-600 text-white'
              : 'border-gray-600 bg-gray-800/80 text-gray-200'
          }`}
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            watchlist.toggle();
          }}
        >
          <Star className="h-5 w-5" />
        </button>
        <RatingControl
          target={target}
          level={ratingLevelOf(state?.me.rating)}
          hasReview={reviewOf(state?.me.rating) !== ''}
          overlay
          iconOnly
          testId="quick-rate"
        />
      </div>
    </div>
  );
};

export default QuickActions;
