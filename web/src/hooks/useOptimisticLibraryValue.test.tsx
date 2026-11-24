// @vitest-environment jsdom
import { useOptimisticLibraryValue } from '@app/hooks/useOptimisticLibraryValue';
import { act, renderHook } from '@testing-library/react';
import { IntlProvider } from 'react-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const addToast = vi.fn();
vi.mock('@app/hooks/useToasts', () => ({
  default: () => ({ addToast, removeToast: vi.fn() }),
  useToasts: () => ({ addToast, removeToast: vi.fn() }),
}));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <IntlProvider locale="en" messages={{}} onError={() => undefined}>
    {children}
  </IntlProvider>
);

// A request the test settles by hand, so the value can be read while it is
// still in flight.
const deferred = () => {
  let resolve: () => void = () => undefined;
  let reject: (error: Error) => void = () => undefined;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

describe('useOptimisticLibraryValue', () => {
  beforeEach(() => {
    addToast.mockReset();
  });

  it('shows the new value at once and keeps it when the request succeeds', async () => {
    const { result } = renderHook(() => useOptimisticLibraryValue(false), {
      wrapper,
    });
    const request = deferred();
    let committed: Promise<boolean> = Promise.resolve(false);
    act(() => {
      committed = result.current.update(true, () => request.promise);
    });
    expect(result.current.value).toBe(true);
    expect(result.current.pending).toBe(true);
    await act(async () => {
      request.resolve();
      await committed;
    });
    expect(await committed).toBe(true);
    expect(result.current.value).toBe(true);
    expect(result.current.pending).toBe(false);
    expect(addToast).not.toHaveBeenCalled();
  });

  it('restores the previous value with an error toast when the request fails', async () => {
    const { result } = renderHook(() => useOptimisticLibraryValue(true), {
      wrapper,
    });
    const request = deferred();
    let committed: Promise<boolean> = Promise.resolve(true);
    act(() => {
      committed = result.current.update(false, () => request.promise);
    });
    expect(result.current.value).toBe(false);
    await act(async () => {
      request.reject(new Error('rejected'));
      await committed;
    });
    expect(await committed).toBe(false);
    expect(result.current.value).toBe(true);
    expect(result.current.pending).toBe(false);
    expect(addToast).toHaveBeenCalledTimes(1);
    expect(addToast.mock.calls[0][1]).toMatchObject({ appearance: 'error' });
  });

  it('restores a string value, not a flipped one', async () => {
    const { result } = renderHook(
      () => useOptimisticLibraryValue<string | null>('liked'),
      { wrapper }
    );
    await act(async () => {
      await result.current.update('hated', () =>
        Promise.reject(new Error('400'))
      );
    });
    expect(result.current.value).toBe('liked');
  });

  it('sends nothing when the value is already the requested one', async () => {
    const { result } = renderHook(() => useOptimisticLibraryValue(true), {
      wrapper,
    });
    const send = vi.fn(() => Promise.resolve());
    let committed = false;
    await act(async () => {
      committed = await result.current.update(true, send);
    });
    expect(committed).toBe(true);
    expect(send).not.toHaveBeenCalled();
    expect(result.current.value).toBe(true);
  });

  it('refuses a second update while one is in flight', async () => {
    const { result } = renderHook(() => useOptimisticLibraryValue(false), {
      wrapper,
    });
    const request = deferred();
    const second = vi.fn(() => Promise.resolve());
    let first: Promise<boolean> = Promise.resolve(false);
    act(() => {
      first = result.current.update(true, () => request.promise);
    });
    let refused = true;
    await act(async () => {
      refused = await result.current.update(false, second);
    });
    expect(refused).toBe(false);
    expect(second).not.toHaveBeenCalled();
    await act(async () => {
      request.resolve();
      await first;
    });
    expect(result.current.value).toBe(true);
  });

  it('follows a new confirmed value when nothing is in flight', () => {
    const { result, rerender } = renderHook(
      ({ confirmed }) => useOptimisticLibraryValue(confirmed),
      { wrapper, initialProps: { confirmed: false } }
    );
    rerender({ confirmed: true });
    expect(result.current.value).toBe(true);
  });
});
