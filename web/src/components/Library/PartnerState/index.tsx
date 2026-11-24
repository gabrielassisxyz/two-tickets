import type {
  LibraryMediaType,
  LibraryPersonState,
} from '@app/components/Library/libraryTypes';
import {
  ratingAnchors,
  ratingLabels,
} from '@app/components/Library/ratingLevels';
import {
  ratingLevelOf,
  reviewOf,
} from '@app/components/Library/useLibraryTitle';
import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.PartnerState', {
  watched: 'Watched',
  inProgress: 'Watching',
  unwatched: 'Not watched yet',
  noRating: 'No rating',
  noReview: 'No review',
});

interface PartnerStateProps {
  mediaType: LibraryMediaType;
  partner: LibraryPersonState;
}

// The partner's watched state, rating and review, read-only: nothing here
// is a control, because every library mutation acts on the caller only.
const PartnerState = ({ mediaType, partner }: PartnerStateProps) => {
  const intl = useIntl();
  const level = ratingLevelOf(partner.rating);
  const review = reviewOf(partner.rating);
  const status =
    mediaType === 'tv' && partner.status === 'in_progress'
      ? messages.inProgress
      : partner.watched || partner.status === 'watched'
        ? messages.watched
        : messages.unwatched;

  return (
    <section
      className="flex flex-col gap-2 border-t border-gray-700 pt-4"
      data-testid="partner-state"
    >
      <h3 className="text-lg font-semibold text-gray-100">
        {partner.displayName}
      </h3>
      <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
        <div>
          <dt className="sr-only">status</dt>
          <dd className="text-gray-200" data-testid="partner-watched">
            {intl.formatMessage(status)}
          </dd>
        </div>
        <div>
          <dt className="sr-only">rating</dt>
          <dd className="text-gray-200" data-testid="partner-rating">
            {level
              ? `${intl.formatMessage(ratingLabels[level])} · ${intl.formatMessage(ratingAnchors[level])}`
              : intl.formatMessage(messages.noRating)}
          </dd>
        </div>
      </dl>
      <p
        className="max-w-prose whitespace-pre-line text-gray-300"
        data-testid="partner-review"
      >
        {review || intl.formatMessage(messages.noReview)}
      </p>
    </section>
  );
};

export default PartnerState;
