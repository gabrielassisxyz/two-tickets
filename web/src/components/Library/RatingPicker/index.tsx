import type { RatingLevel } from '@app/components/Library/libraryTypes';
import {
  ratingAnchors,
  ratingLabels,
  ratingLevels,
} from '@app/components/Library/ratingLevels';
import defineMessages from '@app/utils/defineMessages';
import { useState } from 'react';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.RatingPicker', {
  clear: 'Clear',
  close: 'Close',
  pick: 'Tap a label to rate',
});

interface RatingPickerProps {
  value: RatingLevel | null;
  pending?: boolean;
  onPick: (level: RatingLevel) => void;
  onClear: () => void;
  onClose?: () => void;
  compact?: boolean;
}

// All seven labels at once, worst to best; one tap rates. The anchor line
// explains the label under the finger or the keyboard focus, falling back
// to the current rating, because the words alone do not say where "Legal"
// sits against "Gostei".
const RatingPicker = ({
  value,
  pending = false,
  onPick,
  onClear,
  onClose,
  compact = false,
}: RatingPickerProps) => {
  const intl = useIntl();
  const [focused, setFocused] = useState<RatingLevel | null>(null);
  const described = focused ?? value;

  return (
    <div
      className={`flex flex-col ${compact ? 'gap-1' : 'gap-3'}`}
      data-testid="rating-picker"
      aria-busy={pending}
    >
      <div
        className={`flex ${compact ? 'flex-col gap-0.5' : 'flex-wrap gap-2'}`}
        role="radiogroup"
      >
        {ratingLevels.map((level) => (
          <button
            key={level}
            type="button"
            role="radio"
            aria-checked={value === level}
            data-testid={`rating-level-${level}`}
            disabled={pending}
            className={`rounded-md border font-medium transition duration-150 ease-out disabled:opacity-50 ${
              compact ? 'px-2 py-0.5 text-left text-xs' : 'px-3 py-1.5 text-sm'
            } ${
              value === level
                ? 'border-indigo-500 bg-indigo-600 text-white'
                : 'border-gray-600 bg-gray-800 text-gray-200 hover:border-gray-400'
            }`}
            onClick={(e) => {
              e.stopPropagation();
              onPick(level);
            }}
            onMouseEnter={() => setFocused(level)}
            onMouseLeave={() => setFocused(null)}
            onFocus={() => setFocused(level)}
            onBlur={() => setFocused(null)}
          >
            {intl.formatMessage(ratingLabels[level])}
          </button>
        ))}
      </div>
      <p
        className={`text-gray-400 ${compact ? 'text-[11px] leading-tight' : 'text-sm'}`}
        data-testid="rating-anchor"
      >
        {described
          ? intl.formatMessage(ratingAnchors[described])
          : intl.formatMessage(messages.pick)}
      </p>
      <div className="flex gap-2">
        <button
          type="button"
          data-testid="rating-clear"
          disabled={pending || value === null}
          className={`rounded-md border border-gray-600 px-2 py-0.5 text-gray-300 disabled:opacity-40 ${compact ? 'text-xs' : 'text-sm'}`}
          onClick={(e) => {
            e.stopPropagation();
            onClear();
          }}
        >
          {intl.formatMessage(messages.clear)}
        </button>
        {onClose && (
          <button
            type="button"
            data-testid="rating-close"
            className={`rounded-md border border-gray-600 px-2 py-0.5 text-gray-300 ${compact ? 'text-xs' : 'text-sm'}`}
            onClick={(e) => {
              e.stopPropagation();
              onClose();
            }}
          >
            {intl.formatMessage(messages.close)}
          </button>
        )}
      </div>
    </div>
  );
};

export default RatingPicker;
