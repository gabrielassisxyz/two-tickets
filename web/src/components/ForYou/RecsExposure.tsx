import { postRecExposure } from '@app/components/ForYou/recsRequests';
import { useEffect, useState } from 'react';
import { useInView } from 'react-intersection-observer';

interface RecsExposureProps {
  runId: number;
  rank: number;
  children: React.ReactNode;
}

// Reports one exposure when the card enters the viewport, once per run and
// rank: the request helper dedupes, so a remount never reposts. A rejected
// report retries twice, two seconds apart, then gives up.
const maxRecExposureAttempts = 2;

const RecsExposure = ({ runId, rank, children }: RecsExposureProps) => {
  const { ref, inView } = useInView({ triggerOnce: true });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!inView) {
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    postRecExposure(runId, rank).catch(() => {
      if (!cancelled && attempt < maxRecExposureAttempts) {
        timer = setTimeout(() => setAttempt(attempt + 1), 2000);
      }
    });
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        clearTimeout(timer);
      }
    };
  }, [inView, runId, rank, attempt]);

  return (
    <div
      ref={ref}
      data-testid={`recs-exposure-${runId}-${rank}`}
      data-run-id={runId}
      data-rank={rank}
    >
      {children}
    </div>
  );
};

export default RecsExposure;
