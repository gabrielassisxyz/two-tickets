import LibraryCardFor from '@app/components/Library/LibraryCardFor';
import {
  libraryListItemsKey,
  libraryListsKey,
} from '@app/components/Library/libraryRequests';
import type {
  LibraryCursorPage,
  LibraryList,
  LibraryListItem,
} from '@app/components/Library/libraryTypes';
import Slider from '@app/components/Slider';
import defineMessages from '@app/utils/defineMessages';
import { ArrowRightCircleIcon } from '@heroicons/react/24/outline';
import Link from 'next/link';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.Library.LibraryListSlider', {
  empty: 'Nothing here yet.',
});

interface LibraryListSliderProps {
  // "watchlist" is my own watchlist ("Minha lista"), the one list Discover
  // shows.
  kind: 'watchlist';
  title: string;
  sliderKey: string;
}

export const pickLibraryList = (
  lists: LibraryList[] | undefined,
  kind: LibraryListSliderProps['kind']
) => (lists ?? []).find((list) => list.kind === kind && list.owner === 'me');

// A Discover slider over one library list: the first page of its items,
// hydrated by the api into the same result shape search answers.
const LibraryListSlider = ({
  kind,
  title,
  sliderKey,
}: LibraryListSliderProps) => {
  const intl = useIntl();
  const { data: lists } = useSWR<{ results: LibraryList[] }>(libraryListsKey);
  const list = pickLibraryList(lists?.results, kind);
  const { data: page, mutate } = useSWR<LibraryCursorPage<LibraryListItem>>(
    list ? libraryListItemsKey(list.id) : null
  );
  const items = (page?.results ?? []).filter((item) => item.available);

  return (
    <div data-testid={`library-slider-${kind}`}>
      <div className="slider-header">
        <Link href="/lists" className="slider-title min-w-0 pr-16">
          <span className="truncate">{title}</span>
          <ArrowRightCircleIcon />
        </Link>
      </div>
      <Slider
        sliderKey={sliderKey}
        isLoading={!lists || (!!list && !page)}
        isEmpty={!!page && items.length === 0}
        emptyMessage={intl.formatMessage(messages.empty)}
        items={items.map((item) => (
          <LibraryCardFor
            key={`${item.mediaType}-${item.tmdbId}`}
            item={item}
            isAddedToWatchlist={kind === 'watchlist'}
            mutateParent={() => mutate()}
          />
        ))}
      />
    </div>
  );
};

export default LibraryListSlider;
