// Copied from Seerr `server/api/rating/rottentomatoes.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`). Only the rating shape the frontend renders is kept
// here.
export interface RTRating {
  title: string;
  year: number;
  criticsRating: 'Certified Fresh' | 'Fresh' | 'Rotten';
  criticsScore: number;
  audienceRating?: 'Upright' | 'Spilled';
  audienceScore?: number;
  url: string;
}
