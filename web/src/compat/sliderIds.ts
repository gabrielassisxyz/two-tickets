// Project-owned slider ids. Upstream `DiscoverSliderType` values occupy 1-21 at
// the pinned commit, so project sliders start at 100 and must never collide
// with an upstream value. Later beads attach routes and components to these.
import { DiscoverSliderType } from '@app/compat/constants/discover';

export const SliderId = {
  FOR_YOU: 100,
  MY_LIST: 101,
} as const;

export type SliderId = (typeof SliderId)[keyof typeof SliderId];

const upstreamValues = new Set<number>(
  Object.values(DiscoverSliderType).filter(
    (value): value is number => typeof value === 'number'
  )
);

for (const value of Object.values(SliderId)) {
  if (upstreamValues.has(value)) {
    throw new Error(
      `SliderId ${value} collides with an upstream DiscoverSliderType value`
    );
  }
}
