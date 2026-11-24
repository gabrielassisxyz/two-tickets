import { setLibraryWatched } from '@app/components/Library/libraryRequests';
import { libraryTargetFor } from '@app/components/Library/libraryTarget';
import type { LibraryMediaType } from '@app/components/Library/libraryTypes';
import {
  onWatchlistIn,
  useLibraryTitle,
} from '@app/components/Library/useLibraryTitle';
import useWatchlistToggle from '@app/components/Library/useWatchlistToggle';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import {
  BookmarkIcon as BookmarkOutline,
  CheckCircleIcon as CheckCircleOutline,
} from '@heroicons/react/24/outline';
import { BookmarkIcon, CheckCircleIcon } from '@heroicons/react/24/solid';
import type { ComponentType, SVGProps } from 'react';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.TitleCard.QuickActions', {
  addToWatchList: 'Add to watchlist',
  removeFromWatchList: 'Remove from watchlist',
  markWatched: 'Mark as watched',
  unmarkWatched: 'Unmark as watched',
});

interface QuickActionsProps {
  mediaType: LibraryMediaType;
  tmdbId: number;
  title: string;
  isAddedToWatchlist: boolean;
  inView: boolean;
  // The card's hover (or first tap) state. The buttons stay mounted while
  // hidden, so a request still in flight keeps its state and its rollback.
  visible: boolean;
  mutateParent?: () => void;
}

interface CardActionProps {
  testId: string;
  label: string;
  active: boolean;
  pending: boolean;
  Icon: ComponentType<SVGProps<SVGSVGElement>>;
  onPress: () => void;
}

// Sized and styled after upstream's hover buttons (a small ghost button);
// an active action fills its icon in the accent.
const CardAction = ({
  testId,
  label,
  active,
  pending,
  Icon,
  onPress,
}: CardActionProps) => (
  <button
    type="button"
    data-testid={testId}
    aria-pressed={active}
    aria-busy={pending}
    aria-label={label}
    title={label}
    disabled={pending}
    className="inline-flex items-center justify-center rounded-md border border-gray-600 bg-gray-900/60 px-2 py-1 text-white transition duration-150 ease-out hover:border-gray-200 disabled:cursor-wait disabled:opacity-70"
    onClick={(e) => {
      e.stopPropagation();
      e.preventDefault();
      onPress();
    }}
  >
    <Icon className={`h-4 w-4 ${active ? 'text-indigo-400' : ''}`} />
  </button>
);

// The two actions a title card offers on hover, as upstream offered its
// watchlist and blocklist buttons: watch later, and watched. Both act on the
// movie, or on the whole series. Rating lives on the title's own page.
const QuickActions = ({
  mediaType,
  tmdbId,
  title,
  isAddedToWatchlist,
  inView,
  visible,
  mutateParent,
}: QuickActionsProps) => {
  const intl = useIntl();
  const { state } = useLibraryTitle(mediaType, tmdbId, inView);
  const target = libraryTargetFor(mediaType, tmdbId);
  const watched = useOptimisticLibraryValue(state?.me.watched ?? false);
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

  return (
    <div
      className={`z-40 flex flex-col gap-1 transition-opacity duration-150 ease-out ${
        visible ? 'opacity-100' : 'pointer-events-none invisible opacity-0'
      }`}
      data-testid="quick-actions"
      data-visible={visible}
    >
      <CardAction
        testId="quick-watchlist"
        label={intl.formatMessage(
          watchlist.onWatchlist
            ? messages.removeFromWatchList
            : messages.addToWatchList
        )}
        active={watchlist.onWatchlist}
        pending={watchlist.pending}
        Icon={watchlist.onWatchlist ? BookmarkIcon : BookmarkOutline}
        onPress={watchlist.toggle}
      />
      <CardAction
        testId="quick-watched"
        label={intl.formatMessage(
          watched.value ? messages.unmarkWatched : messages.markWatched
        )}
        active={watched.value}
        pending={watched.pending}
        Icon={watched.value ? CheckCircleIcon : CheckCircleOutline}
        onPress={() =>
          watched.update(!watched.value, (next) =>
            setLibraryWatched(target, next)
          )
        }
      />
    </div>
  );
};

export default QuickActions;
