import Header from '@app/components/Common/Header';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import {
  createLibraryList,
  libraryListsKey,
} from '@app/components/Library/libraryRequests';
import type { LibraryList } from '@app/components/Library/libraryTypes';
import LibraryListSection from '@app/components/Library/ListsPage/LibraryListSection';
import useToasts from '@app/hooks/useToasts';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { useState } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.Library.ListsPage', {
  lists: 'Lists',
  mine: 'My lists',
  shared: 'Ours',
  partner: "Partner's lists",
  partnerReadOnly: 'View only: these lists belong to your partner.',
  newList: 'New list',
  newListName: 'List name',
  create: 'Create list',
  createFailed: 'Could not create the list.',
});

const CreateListForm = () => {
  const intl = useIntl();
  const { addToast } = useToasts();
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);

  return (
    <form
      className="flex flex-wrap items-center gap-2"
      data-testid="list-create"
      onSubmit={async (e) => {
        e.preventDefault();
        const trimmed = name.trim();
        if (trimmed === '' || creating) {
          return;
        }
        setCreating(true);
        try {
          await createLibraryList(trimmed);
          setName('');
        } catch {
          addToast(intl.formatMessage(messages.createFailed), {
            appearance: 'error',
            autoDismiss: true,
          });
        }
        setCreating(false);
      }}
    >
      <input
        aria-label={intl.formatMessage(messages.newListName)}
        data-testid="list-create-name"
        className="rounded-md border border-gray-600 bg-gray-800 px-3 py-1.5 text-gray-100"
        placeholder={intl.formatMessage(messages.newList)}
        maxLength={100}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <button
        type="submit"
        disabled={creating || name.trim() === ''}
        className="rounded-md border border-indigo-500 bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
      >
        {intl.formatMessage(messages.create)}
      </button>
    </form>
  );
};

// My watchlist and custom lists, the couple list, then the partner's lists
// read-only, in the order the api answers them.
const ListsPage = () => {
  const intl = useIntl();
  const { data, error } = useSWR<{ results: LibraryList[] }>(libraryListsKey);

  if (error) {
    return <ErrorPage statusCode={500} />;
  }

  const title = intl.formatMessage(messages.lists);
  const lists = data?.results ?? [];
  const groups: {
    owner: LibraryList['owner'];
    heading: string;
    note?: string;
  }[] = [
    { owner: 'me', heading: intl.formatMessage(messages.mine) },
    { owner: 'shared', heading: intl.formatMessage(messages.shared) },
    {
      owner: 'partner',
      heading: intl.formatMessage(messages.partner),
      note: intl.formatMessage(messages.partnerReadOnly),
    },
  ];

  return (
    <>
      <PageTitle title={title} />
      <div className="mb-5 mt-1">
        <Header>{title}</Header>
      </div>
      {!data ? (
        <LoadingSpinner />
      ) : (
        <div className="flex flex-col gap-10">
          {groups.map((group) => {
            const owned = lists.filter((list) => list.owner === group.owner);
            if (owned.length === 0 && group.owner !== 'me') {
              return null;
            }
            return (
              <div
                key={group.owner}
                className="flex flex-col gap-6"
                data-testid={`list-group-${group.owner}`}
              >
                <h2 className="text-2xl font-bold text-gray-100">
                  {group.heading}
                </h2>
                {group.note && (
                  <p className="text-sm text-gray-400">{group.note}</p>
                )}
                {group.owner === 'me' && <CreateListForm />}
                {owned.map((list) => (
                  <LibraryListSection key={list.id} list={list} />
                ))}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
};

export default ListsPage;
