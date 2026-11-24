// Copied from Seerr `server/api/ratings.ts` (and the rating providers it
// re-exports) at the pinned upstream commit (`web/UPSTREAM.md`).
import type { RTRating } from '@app/compat/api/rottentomatoes';

export interface IMDBRating {
  title: string;
  year?: number;
  url: string;
  criticsScore: number;
  criticsScoreCount: number;
}

export interface RatingResponse {
  rt?: RTRating;
  imdb?: IMDBRating;
}
