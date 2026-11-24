import {
  libraryListsKey,
  setLibraryListMembership,
} from '@app/components/Library/libraryRequests';
import type {
  LibraryList,
  LibraryMediaType,
  LibraryTitleState,
} from '@app/components/Library/libraryTypes';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import { CheckIcon, PlusIcon } from '@heroicons/react/24/outline';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.Library.ListMemberships', {
  lists: 'Lists',
  coupleList: 'Couple list',
});

interface ListMembershipChipProps {
  list: LibraryList;
  mediaType: LibraryMediaType;
  tmdbId: number;
  member: boolean;
}

const ListMembershipChip = ({
  list,
  mediaType,
  tmdbId,
  member,
}: ListMembershipChipProps) => {
  const intl = useIntl();
  const { value, pending, update } = useOptimisticLibraryValue(member);
  const name =
    list.kind === 'couple'
      ? intl.formatMessage(messages.coupleList)
      : list.name;
  const Icon = value ? CheckIcon : PlusIcon;

  return (
    <button
      type="button"
      data-testid={`list-membership-${list.kind}`}
      data-list-id={list.id}
      aria-pressed={value}
      aria-busy={pending}
      disabled={pending}
      className={`inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm transition duration-150 ease-out disabled:cursor-wait disabled:opacity-70 ${
        value
          ? 'border-indigo-500 bg-indigo-600 text-ink'
          : 'border-gray-600 bg-gray-800 text-gray-200 hover:border-gray-400'
      }`}
      onClick={() =>
        update(!value, (next) =>
          setLibraryListMembership(list.id, mediaType, tmdbId, next)
        )
      }
    >
      <Icon className="h-4 w-4" />
      <span>{name}</span>
    </button>
  );
};

interface ListMembershipsProps {
  mediaType: LibraryMediaType;
  tmdbId: number;
  state: LibraryTitleState | undefined;
}

// The lists this title can be put on: the couple list and my custom lists,
// as the api's canChangeItems says. The watchlist has its own star, and the
// partner's lists are not mine to change.
const ListMemberships = ({
  mediaType,
  tmdbId,
  state,
}: ListMembershipsProps) => {
  const intl = useIntl();
  const { data } = useSWR<{ results: LibraryList[] }>(libraryListsKey);
  const lists = (data?.results ?? []).filter(
    (list) => list.canChangeItems && list.kind !== 'watchlist'
  );
  if (lists.length === 0) {
    return null;
  }
  const memberOf = new Set((state?.lists ?? []).map((list) => list.id));

  return (
    <div className="flex flex-col gap-2" data-testid="list-memberships">
      <span className="text-sm font-medium text-gray-300">
        {intl.formatMessage(messages.lists)}
      </span>
      <div className="flex flex-wrap gap-2">
        {lists.map((list) => (
          <ListMembershipChip
            key={list.id}
            list={list}
            mediaType={mediaType}
            tmdbId={tmdbId}
            member={memberOf.has(list.id)}
          />
        ))}
      </div>
    </div>
  );
};

export default ListMemberships;
