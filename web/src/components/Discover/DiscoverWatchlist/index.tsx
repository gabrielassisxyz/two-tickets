import type { WatchlistItem } from '@app/compat/interfaces/api/discoverInterfaces';
import Header from '@app/components/Common/Header';
import ListView from '@app/components/Common/ListView';
import PageTitle from '@app/components/Common/PageTitle';
import useDiscover from '@app/hooks/useDiscover';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Discover.DiscoverWatchlist', {
  discoverwatchlist: 'Your Watchlist',
});

// The watchlist page reads the api's watchlist feed, which the library bead
// serves. Until then every render records the call for the inventory check.
const DiscoverWatchlist = () => {
  const intl = useIntl();

  const {
    isLoadingInitialData,
    isEmpty,
    isLoadingMore,
    isReachingEnd,
    titles,
    fetchMore,
    error,
    mutate,
  } = useDiscover<WatchlistItem>('/api/v1/discover/watchlist');

  if (error) {
    return <ErrorPage statusCode={500} />;
  }

  const title = intl.formatMessage(messages.discoverwatchlist);

  return (
    <>
      <PageTitle title={[title, '']} />
      <div className="mb-5 mt-1">
        <Header subtext="">{title}</Header>
      </div>
      <ListView
        plexItems={titles}
        isEmpty={isEmpty}
        isLoading={
          isLoadingInitialData || (isLoadingMore && (titles?.length ?? 0) > 0)
        }
        isReachingEnd={isReachingEnd}
        onScrollBottom={fetchMore}
        mutateParent={mutate}
      />
    </>
  );
};

export default DiscoverWatchlist;
