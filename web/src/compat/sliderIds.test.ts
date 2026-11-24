import { DiscoverSliderType } from '@app/compat/constants/discover';
import { SliderId } from '@app/compat/sliderIds';
import { describe, expect, it } from 'vitest';

describe('project slider ids', () => {
  it('reserves two ids at 100 or above', () => {
    expect(SliderId.FOR_YOU).toBeGreaterThanOrEqual(100);
    expect(SliderId.MY_LIST).toBeGreaterThanOrEqual(100);
    expect(new Set(Object.values(SliderId)).size).toBe(2);
  });

  it('collides with no upstream DiscoverSliderType value', () => {
    const upstream = new Set(
      Object.values(DiscoverSliderType).filter(
        (value): value is number => typeof value === 'number'
      )
    );
    for (const value of Object.values(SliderId)) {
      expect(upstream.has(value)).toBe(false);
    }
  });
});
