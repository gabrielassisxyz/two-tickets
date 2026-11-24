import { putLibraryReview } from '@app/components/Library/libraryRequests';
import type { LibraryTarget } from '@app/components/Library/libraryTypes';
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import defineMessages from '@app/utils/defineMessages';
import { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Library.ReviewEditor', {
  review: 'My review',
  placeholder: 'What did you think?',
  save: 'Save review',
  saving: 'Saving…',
  saved: 'Review saved',
  tooLong: 'The review is over the limit.',
  count: '{count} / {max}',
});

// The api's own bound on a review, counted in characters (code points), not
// in UTF-16 units: an emoji is one character to the schema.
export const reviewMaxChars = 10000;

export const reviewLength = (text: string) => [...text].length;

// A review is saved only when it says something and fits the schema.
export const reviewIsSavable = (text: string) =>
  text.trim() !== '' && reviewLength(text) <= reviewMaxChars;

interface ReviewEditorProps {
  target: LibraryTarget;
  review: string;
}

const ReviewEditor = ({ target, review }: ReviewEditorProps) => {
  const intl = useIntl();
  const saved = useOptimisticLibraryValue(review);
  const [draft, setDraft] = useState(review);

  // A review loaded or changed elsewhere replaces an untouched draft.
  useEffect(() => {
    setDraft(review);
  }, [review]);

  const length = reviewLength(draft);
  const tooLong = length > reviewMaxChars;
  const unchanged = draft === saved.value;

  return (
    <form
      className="flex flex-col gap-2"
      data-testid="review-editor"
      onSubmit={(e) => {
        e.preventDefault();
        if (!reviewIsSavable(draft)) {
          return;
        }
        saved.update(draft, (next) => putLibraryReview(target, next));
      }}
    >
      <label
        htmlFor={`review-${target.mediaType}-${target.tmdbId}`}
        className="text-sm font-medium text-gray-300"
      >
        {intl.formatMessage(messages.review)}
      </label>
      <textarea
        id={`review-${target.mediaType}-${target.tmdbId}`}
        data-testid="review-text"
        className="min-h-[6rem] w-full rounded-md border border-gray-600 bg-gray-800 p-3 text-gray-100"
        placeholder={intl.formatMessage(messages.placeholder)}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
      />
      <div className="flex items-center justify-between gap-3">
        <span
          className={`text-xs tabular-nums ${tooLong ? 'text-red-400' : 'text-gray-400'}`}
          data-testid="review-count"
        >
          {tooLong
            ? intl.formatMessage(messages.tooLong)
            : intl.formatMessage(messages.count, {
                count: length,
                max: reviewMaxChars,
              })}
        </span>
        <button
          type="submit"
          data-testid="review-save"
          disabled={saved.pending || unchanged || !reviewIsSavable(draft)}
          className="rounded-md border border-indigo-500 bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
        >
          {saved.pending
            ? intl.formatMessage(messages.saving)
            : unchanged && draft !== ''
              ? intl.formatMessage(messages.saved)
              : intl.formatMessage(messages.save)}
        </button>
      </div>
    </form>
  );
};

export default ReviewEditor;
