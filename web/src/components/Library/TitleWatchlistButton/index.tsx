import Button from '@app/components/Common/Button';
import type { LibraryMediaType } from '@app/components/Library/libraryTypes';
import useWatchlistToggle from '@app/components/Library/useWatchlistToggle';
import defineMessages from '@app/utils/defineMessages';
import { MinusCircleIcon, StarIcon } from '@heroicons/react/24/outline';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.TitleWatchlistButton', {
  addToWatchlist: 'Add to watchlist',
  removeFromWatchlist: 'Remove from watchlist',
});

interface TitleWatchlistButtonProps {
  mediaType: LibraryMediaType;
  tmdbId: number;
  title: string;
  // What the title page's own record says: `onUserWatchlist`.
  onWatchlist: boolean;
}

// The watchlist control of the movie and series pages: upstream's two
// buttons, driven by the optimistic hook so a rejected request restores the
// previous state instead of flipping it anyway.
const TitleWatchlistButton = ({
  mediaType,
  tmdbId,
  title,
  onWatchlist,
}: TitleWatchlistButtonProps) => {
  const intl = useIntl();
  const watchlist = useWatchlistToggle(mediaType, tmdbId, title, onWatchlist);

  if (!watchlist.onWatchlist) {
    const label = intl.formatMessage(messages.addToWatchlist);
    return (
      <Button
        buttonType="primary"
        onClick={() => watchlist.add()}
        disabled={watchlist.pending}
        aria-busy={watchlist.pending}
        data-testid="title-watchlist-add"
        title={label}
      >
        <StarIcon />
        <span>{label}</span>
      </Button>
    );
  }
  const label = intl.formatMessage(messages.removeFromWatchlist);
  return (
    <Button
      buttonType="ghost"
      onClick={() => watchlist.remove()}
      disabled={watchlist.pending}
      aria-busy={watchlist.pending}
      data-testid="title-watchlist-remove"
      title={label}
    >
      <MinusCircleIcon />
      <span>{label}</span>
    </Button>
  );
};

export default TitleWatchlistButton;
