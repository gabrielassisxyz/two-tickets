// @vitest-environment jsdom
import RatingPicker from '@app/components/Library/RatingPicker';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { IntlProvider } from 'react-intl';
import { afterEach, describe, expect, it, vi } from 'vitest';

const renderPicker = (props: Partial<Parameters<typeof RatingPicker>[0]>) => {
  const onPick = vi.fn();
  const onClear = vi.fn();
  const screen = render(
    <IntlProvider locale="en" messages={{}} onError={() => undefined}>
      <RatingPicker value={null} onPick={onPick} onClear={onClear} {...props} />
    </IntlProvider>
  );
  return { screen, onPick, onClear };
};

describe('RatingPicker', () => {
  afterEach(cleanup);

  it('shows all seven labels at once, worst to best', () => {
    const { screen } = renderPicker({});
    const levels = [...screen.container.querySelectorAll('[role="radio"]')].map(
      (el) => el.getAttribute('data-testid')
    );
    expect(levels).toEqual([
      'rating-level-hated',
      'rating-level-disliked',
      'rating-level-meh',
      'rating-level-fine',
      'rating-level-good',
      'rating-level-liked',
      'rating-level-loved',
    ]);
  });

  it('rates with one tap on a label', () => {
    const { screen, onPick } = renderPicker({});
    fireEvent.click(screen.getByTestId('rating-level-good'));
    expect(onPick).toHaveBeenCalledTimes(1);
    expect(onPick).toHaveBeenCalledWith('good');
  });

  it('shows the anchor of the focused label, else of the current rating', () => {
    const { screen } = renderPicker({ value: 'loved' });
    expect(screen.getByTestId('rating-anchor').textContent).toBe(
      'it stayed with me, would rewatch'
    );
    fireEvent.focus(screen.getByTestId('rating-level-meh'));
    expect(screen.getByTestId('rating-anchor').textContent).toBe(
      'whatever, it did not grab me'
    );
  });

  it('clears only a rating that exists', () => {
    const empty = renderPicker({});
    expect(
      (empty.screen.getByTestId('rating-clear') as HTMLButtonElement).disabled
    ).toBe(true);
    cleanup();
    const rated = renderPicker({ value: 'fine' });
    fireEvent.click(rated.screen.getByTestId('rating-clear'));
    expect(rated.onClear).toHaveBeenCalledTimes(1);
    expect(rated.onPick).not.toHaveBeenCalled();
  });
});
