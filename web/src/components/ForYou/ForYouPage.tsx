import Header from '@app/components/Common/Header';
import PageTitle from '@app/components/Common/PageTitle';
import RecsExposure from '@app/components/ForYou/RecsExposure';
import { recsPageKey } from '@app/components/ForYou/recsRequests';
import type {
  RecsItem,
  RecsMediaType,
  RecsPage,
} from '@app/components/ForYou/recsTypes';
import LibraryCardFor from '@app/components/Library/LibraryCardFor';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { useState } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.ForYou.ForYouPage', {
  title: 'For you',
  coldStart:
    'Popular this week: rate a few titles and this page learns your taste.',
  empty: 'Rate a few titles and your picks will show up here.',
  becauseYouLiked: 'Because you liked {titles}',
  all: 'All',
  movies: 'Movies',
  series: 'Series',
  previous: 'Previous',
  next: 'Next',
});

const tabs: { key: RecsMediaType; message: keyof typeof messages }[] = [
  { key: 'all', message: 'all' },
  { key: 'movie', message: 'movies' },
  { key: 'tv', message: 'series' },
];

const ForYouCard = ({ item }: { item: RecsItem }) => {
  const intl = useIntl();
  return (
    <RecsExposure runId={item.runId} rank={item.rank}>
      <LibraryCardFor item={item} isAddedToWatchlist={item.onWatchlist} />
      {item.reasons.length > 0 && (
        <p
          data-testid={`recs-reasons-${item.runId}-${item.rank}`}
          className="mt-1 text-xs text-gray-400"
        >
          {intl.formatMessage(messages.becauseYouLiked, {
            titles: item.reasons.join(', '),
          })}
        </p>
      )}
    </RecsExposure>
  );
};

// The paged personal page: the same cards as the carousel, with a media
// type filter and page controls. The filter runs on the server before
// pagination, so the tabs switch a query key rather than slicing a page.
const ForYouPage = () => {
  const intl = useIntl();
  const [mediaType, setMediaType] = useState<RecsMediaType>('all');
  const [page, setPage] = useState(1);
  const { data, error } = useSWR<RecsPage>(recsPageKey(mediaType, page));

  if (error) {
    return <ErrorPage statusCode={500} />;
  }

  const title = intl.formatMessage(messages.title);
  const results = data?.results ?? [];
  const totalPages = data?.totalPages ?? 0;

  const switchTab = (next: RecsMediaType) => {
    setMediaType(next);
    setPage(1);
  };

  return (
    <>
      <PageTitle title={[title, '']} />
      <div className="mb-5 mt-1">
        <Header subtext="">{title}</Header>
      </div>
      <div className="mb-4 flex gap-2" data-testid="for-you-tabs">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            data-testid={`for-you-tab-${tab.key}`}
            aria-pressed={mediaType === tab.key}
            onClick={() => switchTab(tab.key)}
            className={`rounded-full px-4 py-1 text-sm ${
              mediaType === tab.key
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-800 text-gray-300'
            }`}
          >
            {intl.formatMessage(messages[tab.message])}
          </button>
        ))}
      </div>
      {data && !data.personal && (
        <p data-testid="for-you-cold" className="mb-4 text-sm text-gray-400">
          {intl.formatMessage(messages.coldStart)}
        </p>
      )}
      {!data ? (
        <p className="text-gray-400">…</p>
      ) : results.length === 0 ? (
        <p data-testid="for-you-empty" className="text-gray-400">
          {intl.formatMessage(messages.empty)}
        </p>
      ) : (
        <div
          data-testid="for-you-grid"
          className="grid grid-cols-3 gap-4 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6"
        >
          {results.map((item) => (
            <ForYouCard key={`${item.mediaType}-${item.tmdbId}`} item={item} />
          ))}
        </div>
      )}
      {totalPages > 1 && (
        <div className="mt-6 flex items-center justify-between">
          <button
            type="button"
            data-testid="for-you-previous"
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
            className="rounded bg-gray-800 px-4 py-2 text-sm text-gray-100 disabled:opacity-50"
          >
            {intl.formatMessage(messages.previous)}
          </button>
          <span className="text-sm text-gray-400">
            {page} / {totalPages}
          </span>
          <button
            type="button"
            data-testid="for-you-next"
            disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}
            className="rounded bg-gray-800 px-4 py-2 text-sm text-gray-100 disabled:opacity-50"
          >
            {intl.formatMessage(messages.next)}
          </button>
        </div>
      )}
    </>
  );
};

export default ForYouPage;
