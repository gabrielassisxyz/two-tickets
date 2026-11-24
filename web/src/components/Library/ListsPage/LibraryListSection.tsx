import ConfirmButton from '@app/components/Common/ConfirmButton';
import LibraryCardFor from '@app/components/Library/LibraryCardFor';
import {
  deleteLibraryList,
  deleteLibraryListItem,
  libraryListItemsKey,
  renameLibraryList,
} from '@app/components/Library/libraryRequests';
import type {
  LibraryCursorPage,
  LibraryList,
  LibraryListItem,
} from '@app/components/Library/libraryTypes';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import { useState } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.Library.ListsPage.Section', {
  watchlist: 'Watchlist',
  coupleList: 'Couple list',
  itemCount: '{count, plural, one {# title} other {# titles}}',
  rename: 'Rename',
  saveName: 'Save name',
  cancel: 'Cancel',
  delete: 'Delete list',
  confirmDelete: 'Delete it?',
  removeItem: 'Remove from list',
  more: 'Show more',
  empty: 'No titles on this list.',
  unavailable: 'Title unavailable',
});

interface ListItemTileProps {
  list: LibraryList;
  item: LibraryListItem;
}

// One item, with its remove control only when the api says this list's
// items are mine to change. Removal hides the item at once and brings it
// back if the api refuses.
const ListItemTile = ({ list, item }: ListItemTileProps) => {
  const intl = useIntl();
  const present = useOptimisticLibraryValue(true);
  if (!present.value) {
    return null;
  }
  return (
    <li className="flex flex-col gap-2" data-testid="list-item">
      {item.available ? (
        <LibraryCardFor
          item={item}
          canExpand
          isAddedToWatchlist={list.kind === 'watchlist' && list.owner === 'me'}
          showQuickActions={list.owner !== 'partner'}
        />
      ) : (
        // A title TMDB cannot answer right now: named, never linked, and
        // still removable.
        <div
          className="flex aspect-[2/3] w-full items-center justify-center rounded-xl border border-gray-700 bg-gray-800 p-3 text-center text-sm text-gray-400"
          data-testid="list-item-unavailable"
        >
          {intl.formatMessage(messages.unavailable)}
        </div>
      )}
      {list.canChangeItems && (
        <button
          type="button"
          data-testid="list-item-remove"
          disabled={present.pending}
          className="rounded-md border border-gray-600 px-2 py-1 text-xs text-gray-300 disabled:opacity-50"
          onClick={() =>
            present.update(false, () =>
              deleteLibraryListItem(list.id, item.mediaType, item.tmdbId)
            )
          }
        >
          {intl.formatMessage(messages.removeItem)}
        </button>
      )}
    </li>
  );
};

interface ListItemsPageProps {
  list: LibraryList;
  cursor?: string;
}

// A page of a list's items; "Ver mais" renders the next page after it, so
// every page keeps its own cache key and a removal refreshes only its page.
const ListItemsPage = ({ list, cursor }: ListItemsPageProps) => {
  const intl = useIntl();
  const [showNext, setShowNext] = useState(false);
  const key = cursor
    ? `${libraryListItemsKey(list.id)}?cursor=${encodeURIComponent(cursor)}`
    : libraryListItemsKey(list.id);
  const { data } = useSWR<LibraryCursorPage<LibraryListItem>>(key);
  const items = data?.results ?? [];
  if (!cursor && data && items.length === 0) {
    return (
      <p className="text-sm text-gray-400">
        {intl.formatMessage(messages.empty)}
      </p>
    );
  }
  return (
    <>
      <ul className="cards-vertical">
        {items.map((item) => (
          <ListItemTile
            key={`${item.mediaType}-${item.tmdbId}`}
            list={list}
            item={item}
          />
        ))}
      </ul>
      {data?.nextCursor &&
        (showNext ? (
          <ListItemsPage list={list} cursor={data.nextCursor} />
        ) : (
          <button
            type="button"
            className="self-start rounded-md border border-gray-600 px-3 py-1.5 text-sm text-gray-200"
            onClick={() => setShowNext(true)}
          >
            {intl.formatMessage(messages.more)}
          </button>
        ))}
    </>
  );
};

interface ListNameProps {
  list: LibraryList;
}

const ListName = ({ list }: ListNameProps) => {
  const intl = useIntl();
  const name = useOptimisticLibraryValue(list.name);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(list.name);
  const fixedName =
    list.kind === 'watchlist'
      ? intl.formatMessage(messages.watchlist)
      : list.kind === 'couple'
        ? intl.formatMessage(messages.coupleList)
        : null;

  if (!editing) {
    return (
      <div className="flex flex-wrap items-center gap-3">
        <h3 className="text-xl font-semibold text-gray-100">
          {fixedName ?? name.value}
        </h3>
        <span className="text-sm text-gray-400">
          {intl.formatMessage(messages.itemCount, { count: list.itemCount })}
        </span>
        {list.canRename && (
          <button
            type="button"
            data-testid="list-rename"
            disabled={name.pending}
            className="rounded-md border border-gray-600 px-2 py-1 text-xs text-gray-300"
            onClick={() => {
              setDraft(name.value);
              setEditing(true);
            }}
          >
            {intl.formatMessage(messages.rename)}
          </button>
        )}
      </div>
    );
  }
  return (
    <form
      className="flex flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        const trimmed = draft.trim();
        if (trimmed === '') {
          return;
        }
        setEditing(false);
        name.update(trimmed, (next) => renameLibraryList(list.id, next));
      }}
    >
      <input
        data-testid="list-rename-input"
        className="rounded-md border border-gray-600 bg-gray-800 px-2 py-1 text-gray-100"
        value={draft}
        maxLength={100}
        onChange={(e) => setDraft(e.target.value)}
      />
      <button
        type="submit"
        className="rounded-md border border-indigo-500 bg-indigo-600 px-2 py-1 text-xs text-white"
      >
        {intl.formatMessage(messages.saveName)}
      </button>
      <button
        type="button"
        className="rounded-md border border-gray-600 px-2 py-1 text-xs text-gray-300"
        onClick={() => setEditing(false)}
      >
        {intl.formatMessage(messages.cancel)}
      </button>
    </form>
  );
};

interface LibraryListSectionProps {
  list: LibraryList;
}

// One list. Rename and delete exist only where the api's canRename says so,
// which leaves them off the watchlist, the couple list and every list of
// the partner's; item removal follows canChangeItems the same way.
const LibraryListSection = ({ list }: LibraryListSectionProps) => {
  const intl = useIntl();
  const present = useOptimisticLibraryValue(true);
  if (!present.value) {
    return null;
  }
  return (
    <section
      className="flex flex-col gap-3"
      data-testid={`library-list-${list.kind}`}
      data-list-id={list.id}
      data-owner={list.owner}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <ListName list={list} />
        {list.canRename && (
          <ConfirmButton
            className="text-xs"
            confirmText={intl.formatMessage(messages.confirmDelete)}
            onClick={() =>
              present.update(false, () => deleteLibraryList(list.id))
            }
          >
            <span data-testid="list-delete">
              {intl.formatMessage(messages.delete)}
            </span>
          </ConfirmButton>
        )}
      </div>
      <ListItemsPage list={list} />
    </section>
  );
};

export default LibraryListSection;
