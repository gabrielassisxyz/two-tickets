import { setLibraryWatched } from '@app/components/Library/libraryRequests';
import type { LibraryTarget } from '@app/components/Library/libraryTypes';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import { CheckCircleIcon as CheckCircleOutline } from '@heroicons/react/24/outline';
import { CheckCircleIcon } from '@heroicons/react/24/solid';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.WatchedToggle', {
  watched: 'Watched',
  markWatched: 'Mark as watched',
  unmarkWatched: 'Unmark as watched',
});

interface WatchedToggleProps {
  target: LibraryTarget;
  watched: boolean;
  label?: string;
  iconOnly?: boolean;
  testId?: string;
}

const WatchedToggle = ({
  target,
  watched,
  label,
  iconOnly = false,
  testId = 'watched-toggle',
}: WatchedToggleProps) => {
  const intl = useIntl();
  const { value, pending, update } = useOptimisticLibraryValue(watched);
  const action = intl.formatMessage(
    value ? messages.unmarkWatched : messages.markWatched
  );
  const Icon = value ? CheckCircleIcon : CheckCircleOutline;

  return (
    <button
      type="button"
      data-testid={testId}
      aria-pressed={value}
      aria-busy={pending}
      aria-label={action}
      title={action}
      disabled={pending}
      className={`inline-flex items-center justify-center gap-1.5 rounded-md border transition duration-150 ease-out disabled:cursor-wait disabled:opacity-70 ${
        iconOnly ? 'h-9 w-9' : 'px-3 py-1.5 text-sm font-medium'
      } ${
        value
          ? 'border-indigo-500 bg-indigo-600 text-white'
          : 'border-gray-600 bg-gray-800/80 text-gray-200 hover:border-gray-400'
      }`}
      onClick={(e) => {
        e.stopPropagation();
        e.preventDefault();
        update(!value, (next) => setLibraryWatched(target, next));
      }}
    >
      <Icon className="h-5 w-5" />
      {!iconOnly && (
        <span>{label ?? intl.formatMessage(messages.watched)}</span>
      )}
    </button>
  );
};

export default WatchedToggle;
