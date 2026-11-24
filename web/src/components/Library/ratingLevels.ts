import type { RatingLevel } from '@app/components/Library/libraryTypes';
import defineMessages from '@app/utils/defineMessages';

// The seven-step scale, worst to best, as internal/library/levels.go names
// it. The words people see are the messages below, translated in the i18n
// files; only the level names travel to the api.
export const ratingLevels: readonly RatingLevel[] = [
  'hated',
  'disliked',
  'meh',
  'fine',
  'good',
  'liked',
  'loved',
];

export const isRatingLevel = (value: unknown): value is RatingLevel =>
  typeof value === 'string' &&
  (ratingLevels as readonly string[]).includes(value);

export const ratingLabels = defineMessages('components.Library.ratingLabels', {
  hated: 'Hated',
  disliked: 'Disliked',
  meh: 'Meh',
  fine: 'Fine',
  good: 'Good',
  liked: 'Liked',
  loved: 'Loved',
});

export const ratingAnchors = defineMessages(
  'components.Library.ratingAnchors',
  {
    hated: 'angry I watched it',
    disliked: 'bad, would not recommend',
    meh: 'whatever, it did not grab me',
    fine: 'ok, passed the time, soon forgotten',
    good: 'good, but it did not stay with me',
    liked: 'it grabbed me, would recommend',
    loved: 'it stayed with me, would rewatch',
  }
);
