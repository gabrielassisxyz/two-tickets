// Every call the For you UI makes to /api/v1/recommendations. Exposures
// resolve on a 204 and reject otherwise; a card that never entered the
// viewport never posts.
import type { RecsMediaType } from '@app/components/ForYou/recsTypes';
import axios from 'axios';

export const recsPageKey = (mediaType: RecsMediaType, page: number) =>
  `/api/v1/recommendations?mediaType=${mediaType}&page=${page}`;

// A card enters the viewport once per run, so a remount or a strict-mode
// double effect never records the same rank twice. A rank joins the posted
// set only once its POST succeeds, so a rejected report retries instead of
// vanishing from the measurement later models train on.
const postedRecExposures = new Set<string>();
const pendingRecExposures = new Set<string>();

export const postRecExposure = async (runId: number, rank: number) => {
  const key = `${runId}:${rank}`;
  if (postedRecExposures.has(key) || pendingRecExposures.has(key)) {
    return;
  }
  pendingRecExposures.add(key);
  try {
    await axios.post('/api/v1/recommendations/exposures', {
      runId,
      ranks: [rank],
    });
    postedRecExposures.add(key);
  } finally {
    pendingRecExposures.delete(key);
  }
};
