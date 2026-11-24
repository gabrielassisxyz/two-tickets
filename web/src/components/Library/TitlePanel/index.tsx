import { libraryTargetFor } from '@app/components/Library/libraryTarget';
import type { LibraryMediaType } from '@app/components/Library/libraryTypes';
import ListMemberships from '@app/components/Library/ListMemberships';
import PartnerState from '@app/components/Library/PartnerState';
import RatingControl from '@app/components/Library/RatingControl';
import ReviewEditor from '@app/components/Library/ReviewEditor';
import {
  ratingLevelOf,
  reviewOf,
  useLibraryTitle,
} from '@app/components/Library/useLibraryTitle';
import WatchedToggle from '@app/components/Library/WatchedToggle';
import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.TitlePanel', {
  mine: 'Me',
  wholeSeries: 'Whole series',
  inProgress: 'In progress: {count} seasons watched',
});

interface TitlePanelProps {
  mediaType: LibraryMediaType;
  tmdbId: number;
}

// My state for a title, editable, and the partner's, read-only. On a series
// the controls here act on the whole series; each season row has its own.
const TitlePanel = ({ mediaType, tmdbId }: TitlePanelProps) => {
  const intl = useIntl();
  const { state } = useLibraryTitle(mediaType, tmdbId);
  const target = libraryTargetFor(mediaType, tmdbId);
  const me = state?.me;

  return (
    <section
      className="mt-6 flex flex-col gap-5 rounded-md border border-gray-700 bg-gray-800/60 p-4"
      data-testid="title-panel"
    >
      <div className="flex flex-col gap-3">
        <h3 className="text-lg font-semibold text-gray-100">
          {me?.displayName || intl.formatMessage(messages.mine)}
        </h3>
        {mediaType === 'tv' && (
          <span className="text-sm font-medium text-gray-400">
            {intl.formatMessage(messages.wholeSeries)}
          </span>
        )}
        <div className="flex flex-wrap items-start gap-2">
          <WatchedToggle
            target={target}
            watched={me?.watched ?? false}
            testId={mediaType === 'tv' ? 'series-watched' : 'title-watched'}
          />
          <RatingControl
            target={target}
            level={ratingLevelOf(me?.rating)}
            hasReview={reviewOf(me?.rating) !== ''}
            testId={mediaType === 'tv' ? 'series-rate' : 'title-rate'}
          />
        </div>
        {mediaType === 'tv' && me?.status === 'in_progress' && (
          <span className="text-sm text-gray-400" data-testid="series-progress">
            {intl.formatMessage(messages.inProgress, {
              count: me.regularSeasonsWatched,
            })}
          </span>
        )}
        <ReviewEditor target={target} review={reviewOf(me?.rating)} />
        <ListMemberships mediaType={mediaType} tmdbId={tmdbId} state={state} />
      </div>
      {state && <PartnerState mediaType={mediaType} partner={state.partner} />}
    </section>
  );
};

export default TitlePanel;
