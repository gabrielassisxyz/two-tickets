import useToasts from '@app/hooks/useToasts';
import defineMessages from '@app/utils/defineMessages';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useIntl } from 'react-intl';

const messages = defineMessages('hooks.useOptimisticLibraryValue', {
  failed: 'Could not save. The previous value was restored.',
});

export interface OptimisticLibraryValue<T> {
  // What the control shows: the confirmed value, or the one being sent.
  value: T;
  // True while a request is in flight; the control shows its loading state
  // and further updates are refused until it settles.
  pending: boolean;
  // Shows `next` at once and sends it. Resolves true when the value was
  // committed (or already was the value, which sends nothing), false when
  // the request failed and the previous value was restored with a toast,
  // or when another update was still in flight.
  update: (next: T, send: (next: T) => Promise<unknown>) => Promise<boolean>;
}

// The one optimistic-update hook every library control uses: watched,
// rating, review, list membership and the watchlist star. `confirmed` is
// what the server last said; it must be a primitive (boolean, string, null)
// so a re-render never reads as a change. Upstream's controls flipped their
// state after the request whatever its outcome; this hook remembers the
// previous value and restores it only when the request fails.
export const useOptimisticLibraryValue = <T>(
  confirmed: T
): OptimisticLibraryValue<T> => {
  const intl = useIntl();
  const { addToast } = useToasts();
  const [value, setValue] = useState<T>(confirmed);
  const [pending, setPending] = useState(false);
  const valueRef = useRef<T>(confirmed);
  const pendingRef = useRef(false);

  useEffect(() => {
    if (pendingRef.current) {
      return;
    }
    valueRef.current = confirmed;
    setValue(confirmed);
  }, [confirmed]);

  const update = useCallback(
    async (next: T, send: (next: T) => Promise<unknown>) => {
      const show = (shown: T) => {
        valueRef.current = shown;
        setValue(shown);
      };
      const settle = () => {
        pendingRef.current = false;
        setPending(false);
      };
      if (pendingRef.current) {
        return false;
      }
      const previous = valueRef.current;
      if (Object.is(previous, next)) {
        return true;
      }
      show(next);
      pendingRef.current = true;
      setPending(true);
      try {
        await send(next);
      } catch {
        show(previous);
        settle();
        addToast(intl.formatMessage(messages.failed), {
          appearance: 'error',
          autoDismiss: true,
        });
        return false;
      }
      settle();
      return true;
    },
    [addToast, intl]
  );

  return { value, pending, update };
};

export default useOptimisticLibraryValue;
