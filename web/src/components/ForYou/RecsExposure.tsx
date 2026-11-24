import { postRecExposure } from '@app/components/ForYou/recsRequests';
import { useEffect, useRef } from 'react';
import { useInView } from 'react-intersection-observer';

interface RecsExposureProps {
  runId: number;
  rank: number;
  children: React.ReactNode;
}

// Reports one exposure when the card enters the viewport. The backend
// records once per run and rank, so a repeated report is a no-op; the local
// guard keeps a remount from even sending it. A rejected report clears the
// guard, so the next entry retries.
const RecsExposure = ({ runId, rank, children }: RecsExposureProps) => {
  const { ref, inView } = useInView({ triggerOnce: true });
  const reported = useRef(false);

  useEffect(() => {
    if (!inView || reported.current) {
      return;
    }
    reported.current = true;
    postRecExposure(runId, rank).catch(() => {
      reported.current = false;
    });
  }, [inView, runId, rank]);

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
