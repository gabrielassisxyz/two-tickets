import RatingPicker from '@app/components/Library/RatingPicker';
import { setLibraryRating } from '@app/components/Library/libraryRequests';
import type {
  LibraryTarget,
  RatingLevel,
} from '@app/components/Library/libraryTypes';
import { ratingLabels } from '@app/components/Library/ratingLevels';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import { FaceSmileIcon as FaceSmileOutline } from '@heroicons/react/24/outline';
import { FaceSmileIcon } from '@heroicons/react/24/solid';
import { useState } from 'react';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.RatingControl', {
  rate: 'Rate',
});

interface RatingControlProps {
  target: LibraryTarget;
  level: RatingLevel | null;
  hasReview: boolean;
  testId?: string;
}

const RatingControl = ({
  target,
  level,
  hasReview,
  testId = 'rating-toggle',
}: RatingControlProps) => {
  const intl = useIntl();
  const [open, setOpen] = useState(false);
  const { value, pending, update } =
    useOptimisticLibraryValue<RatingLevel | null>(level);
  const label = value
    ? intl.formatMessage(ratingLabels[value])
    : intl.formatMessage(messages.rate);
  const Icon = value ? FaceSmileIcon : FaceSmileOutline;

  const choose = (next: RatingLevel | null) => {
    setOpen(false);
    update(next, (sent) => setLibraryRating(target, sent, hasReview));
  };

  const picker = (
    <RatingPicker
      value={value}
      pending={pending}
      onPick={choose}
      onClear={() => choose(null)}
    />
  );

  return (
    <>
      <button
        type="button"
        data-testid={testId}
        aria-expanded={open}
        aria-busy={pending}
        aria-label={label}
        title={label}
        disabled={pending}
        className={`inline-flex items-center justify-center gap-1.5 rounded-md border px-3 py-1.5 text-sm font-medium transition duration-150 ease-out disabled:cursor-wait disabled:opacity-70 ${
          value
            ? 'border-indigo-500 bg-indigo-600 text-white'
            : 'border-gray-600 bg-gray-800/80 text-gray-200 hover:border-gray-400'
        }`}
        onClick={(e) => {
          e.stopPropagation();
          e.preventDefault();
          setOpen((wasOpen) => !wasOpen);
        }}
      >
        <Icon className="h-5 w-5" />
        <span>{label}</span>
      </button>
      {open && <div className="mt-3 w-full">{picker}</div>}
    </>
  );
};

export default RatingControl;
