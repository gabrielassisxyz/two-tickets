import RecsExposure from '@app/components/ForYou/RecsExposure';
import { recsPageKey } from '@app/components/ForYou/recsRequests';
import type { RecsPage } from '@app/components/ForYou/recsTypes';
import LibraryCardFor from '@app/components/Library/LibraryCardFor';
import Slider from '@app/components/Slider';
import defineMessages from '@app/utils/defineMessages';
import { ArrowRightCircleIcon } from '@heroicons/react/24/outline';
import Link from 'next/link';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.ForYou.ForYouSlider', {
  coldStart: 'Popular this week',
  empty: 'Rate a few titles and your picks will show up here.',
  becauseYouLiked: 'Because you liked {titles}',
});

interface ForYouSliderProps {
  title: string;
  sliderKey: string;
}

// A Discover slider over the personal list: the first page of hydrated
// cards, each with its "because you liked" line. Titles TMDB could not
// answer render unlinked, the way unavailable library items do, so one
// unreachable title degrades one card and never the row.
const ForYouSlider = ({ title, sliderKey }: ForYouSliderProps) => {
  const intl = useIntl();
  const { data } = useSWR<RecsPage>(recsPageKey('all', 1));

  const items = (data?.results ?? []).map((item) => (
    <RecsExposure
      key={`${item.mediaType}-${item.tmdbId}`}
      runId={item.runId}
      rank={item.rank}
    >
      <LibraryCardFor item={item} isAddedToWatchlist={item.onWatchlist} />
      {item.reasons.length > 0 && (
        <p
          data-testid={`recs-reasons-${item.runId}-${item.rank}`}
          className="mt-1 max-w-36 truncate text-xs text-gray-400 sm:max-w-44 md:max-w-44"
        >
          {intl.formatMessage(messages.becauseYouLiked, {
            titles: item.reasons.join(', '),
          })}
        </p>
      )}
    </RecsExposure>
  ));

  return (
    <div data-testid="for-you-slider">
      <div className="slider-header">
        <Link href="/discover/for-you" className="slider-title min-w-0 pr-16">
          <span className="truncate">{title}</span>
          <ArrowRightCircleIcon />
        </Link>
      </div>
      {data && !data.personal && (
        <p data-testid="for-you-cold" className="mb-2 text-sm text-gray-400">
          {intl.formatMessage(messages.coldStart)}
        </p>
      )}
      <Slider
        sliderKey={sliderKey}
        isLoading={!data}
        isEmpty={!!data && items.length === 0}
        emptyMessage={intl.formatMessage(messages.empty)}
        items={items}
      />
    </div>
  );
};

export default ForYouSlider;
