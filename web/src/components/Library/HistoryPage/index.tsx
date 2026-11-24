import CachedImage from '@app/components/Common/CachedImage';
import Header from '@app/components/Common/Header';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import type {
  LibraryCursorPage,
  LibraryHistoryRow,
  RatingLevel,
} from '@app/components/Library/libraryTypes';
import {
  isRatingLevel,
  ratingLabels,
  ratingLevels,
} from '@app/components/Library/ratingLevels';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import Link from 'next/link';
import { useState } from 'react';
import { useIntl } from 'react-intl';
import useSWRInfinite from 'swr/infinite';

const messages = defineMessages('components.Library.HistoryPage', {
  history: 'History',
  filter: 'Rating',
  allLevels: 'All ratings',
  movie: 'Movie',
  wholeSeries: 'Whole series',
  season: 'Season {number}',
  specials: 'Specials',
  watchedOn: 'Watched on {date}',
  notWatched: 'Not watched',
  notRated: 'Not rated',
  unavailable: 'Title unavailable',
  more: 'Load more',
  empty: 'Nothing recorded yet.',
});

// The api's history page key. A filter starts a new chain of pages; every
// later page is the previous page's opaque cursor.
export const historyPageKey = (
  level: RatingLevel | '',
  cursor: string | null
) => {
  const params = new URLSearchParams();
  if (level) {
    params.set('level', level);
  }
  if (cursor) {
    params.set('cursor', cursor);
  }
  const query = params.toString();
  return query ? `/api/v1/library/history?${query}` : '/api/v1/library/history';
};

const titleOf = (row: LibraryHistoryRow) =>
  row.result.mediaType === 'movie' ? row.result.title : row.result.name;

interface HistoryRowProps {
  row: LibraryHistoryRow;
}

// One title and scope, with both what was watched and how it was rated, so
// a rated and watched title is one row and not two.
const HistoryRowItem = ({ row }: HistoryRowProps) => {
  const intl = useIntl();
  const scope =
    row.scope === 'movie'
      ? intl.formatMessage(messages.movie)
      : row.scope === 'show'
        ? intl.formatMessage(messages.wholeSeries)
        : row.seasonNumber === 0
          ? intl.formatMessage(messages.specials)
          : intl.formatMessage(messages.season, { number: row.seasonNumber });
  const level = isRatingLevel(row.rating?.level) ? row.rating.level : null;
  const href =
    row.mediaType === 'movie' ? `/movie/${row.tmdbId}` : `/tv/${row.tmdbId}`;

  return (
    <li
      className="flex gap-3 py-3"
      data-testid="history-row"
      data-scope={row.scope}
      data-media-type={row.mediaType}
      data-tmdb-id={row.tmdbId}
    >
      <div className="relative h-20 w-14 flex-shrink-0 overflow-hidden rounded bg-gray-800">
        {row.available && row.result.posterPath && (
          <CachedImage
            type="tmdb"
            src={`https://image.tmdb.org/t/p/w185${row.result.posterPath}`}
            alt=""
            fill
            style={{ objectFit: 'cover' }}
          />
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        {row.available ? (
          <Link href={href} className="truncate font-semibold text-gray-100">
            {titleOf(row)}
          </Link>
        ) : (
          // A title TMDB cannot answer has no page to link to.
          <span className="truncate font-semibold text-gray-400">
            {intl.formatMessage(messages.unavailable)}
          </span>
        )}
        <span className="text-sm text-gray-400">{scope}</span>
        <div className="flex flex-wrap gap-x-4 text-sm">
          <span className="text-gray-200" data-testid="history-watched">
            {row.watched && row.watchedAt
              ? intl.formatMessage(messages.watchedOn, {
                  date: intl.formatDate(row.watchedAt, {
                    year: 'numeric',
                    month: 'short',
                    day: 'numeric',
                  }),
                })
              : intl.formatMessage(messages.notWatched)}
          </span>
          <span className="text-gray-200" data-testid="history-rating">
            {level
              ? intl.formatMessage(ratingLabels[level])
              : intl.formatMessage(messages.notRated)}
          </span>
        </div>
        {row.rating?.review && (
          <p className="line-clamp-2 max-w-prose text-sm text-gray-300">
            {row.rating.review}
          </p>
        )}
      </div>
    </li>
  );
};

// The current-state index: one row per title and scope, newest activity
// first, 20 to a page as the api pages them, filterable by rating level.
const HistoryPage = () => {
  const intl = useIntl();
  const [level, setLevel] = useState<RatingLevel | ''>('');
  const { data, error, size, setSize, isValidating } = useSWRInfinite<
    LibraryCursorPage<LibraryHistoryRow>
  >(
    (index, previous: LibraryCursorPage<LibraryHistoryRow> | null) => {
      if (index === 0) {
        return historyPageKey(level, null);
      }
      return previous?.nextCursor
        ? historyPageKey(level, previous.nextCursor)
        : null;
    },
    { revalidateFirstPage: false }
  );

  if (error) {
    return <ErrorPage statusCode={500} />;
  }

  const title = intl.formatMessage(messages.history);
  const rows = (data ?? []).flatMap((page) => page?.results ?? []);
  const last = data?.[data.length - 1];
  const loadingMore = isValidating && size > (data?.length ?? 0);

  return (
    <>
      <PageTitle title={title} />
      <div className="mb-5 mt-1">
        <Header>{title}</Header>
      </div>
      <label className="mb-4 flex items-center gap-3 text-sm text-gray-300">
        <span>{intl.formatMessage(messages.filter)}</span>
        <select
          data-testid="history-level"
          className="rounded-md border border-gray-600 bg-gray-800 py-1.5 text-gray-100"
          value={level}
          onChange={(e) =>
            setLevel(isRatingLevel(e.target.value) ? e.target.value : '')
          }
        >
          <option value="">{intl.formatMessage(messages.allLevels)}</option>
          {ratingLevels.map((option) => (
            <option key={option} value={option}>
              {intl.formatMessage(ratingLabels[option])}
            </option>
          ))}
        </select>
      </label>
      {!data ? (
        <LoadingSpinner />
      ) : rows.length === 0 ? (
        <p className="text-gray-400">{intl.formatMessage(messages.empty)}</p>
      ) : (
        <ul
          className="flex flex-col divide-y divide-gray-700"
          data-testid="history-rows"
        >
          {rows.map((row) => (
            <HistoryRowItem
              key={`${row.mediaType}-${row.tmdbId}-${row.scope}-${row.seasonNumber ?? 'none'}`}
              row={row}
            />
          ))}
        </ul>
      )}
      {last?.nextCursor && (
        <button
          type="button"
          data-testid="history-more"
          disabled={loadingMore}
          className="mt-4 rounded-md border border-gray-600 px-3 py-1.5 text-sm text-gray-200 disabled:opacity-50"
          onClick={() => setSize(size + 1)}
        >
          {intl.formatMessage(messages.more)}
        </button>
      )}
    </>
  );
};

export default HistoryPage;
