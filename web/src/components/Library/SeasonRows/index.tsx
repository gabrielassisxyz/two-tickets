import type { Season } from '@app/compat/models/Tv';
import CachedImage from '@app/components/Common/CachedImage';
import { libraryTargetFor } from '@app/components/Library/libraryTarget';
import RatingControl from '@app/components/Library/RatingControl';
import {
  ratingLevelOf,
  reviewOf,
  seasonStateOf,
  useLibraryTitle,
} from '@app/components/Library/useLibraryTitle';
import WatchedToggle from '@app/components/Library/WatchedToggle';
import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.SeasonRows', {
  episodes: '{count} episodes',
});

interface SeasonRowsProps {
  tvId: number;
  seasons: Season[];
}

// One row per season TMDB lists, specials (season 0) included, each with
// its own watched toggle and rating; both act on that season only.
const SeasonRows = ({ tvId, seasons }: SeasonRowsProps) => {
  const intl = useIntl();
  const { state } = useLibraryTitle('tv', tvId);

  return (
    <ul
      className="flex flex-col divide-y divide-gray-700 rounded-md border border-gray-700"
      data-testid="season-rows"
    >
      {seasons.map((season) => {
        const mine = seasonStateOf(state?.me, season.seasonNumber);
        const target = libraryTargetFor('tv', tvId, season.seasonNumber);
        return (
          <li
            key={season.seasonNumber}
            className="flex flex-wrap items-center gap-3 p-3"
            data-testid={`season-row-${season.seasonNumber}`}
          >
            <div className="relative h-16 w-11 flex-shrink-0 overflow-hidden rounded bg-gray-800">
              {season.posterPath && (
                <CachedImage
                  type="tmdb"
                  src={`https://image.tmdb.org/t/p/w185${season.posterPath}`}
                  alt=""
                  fill
                  style={{ objectFit: 'cover' }}
                />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate font-semibold text-gray-100">
                {season.name}
              </div>
              <div className="text-sm text-gray-400">
                {intl.formatMessage(messages.episodes, {
                  count: season.episodeCount,
                })}
              </div>
            </div>
            <div className="flex flex-wrap items-start gap-2">
              <WatchedToggle
                target={target}
                watched={mine?.watched ?? false}
                iconOnly
                testId={`season-watched-${season.seasonNumber}`}
              />
              <RatingControl
                target={target}
                level={ratingLevelOf(mine?.rating)}
                hasReview={reviewOf(mine?.rating) !== ''}
                testId={`season-rate-${season.seasonNumber}`}
              />
            </div>
          </li>
        );
      })}
    </ul>
  );
};

export default SeasonRows;
