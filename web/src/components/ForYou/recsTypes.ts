// The shapes the recommendations api answers, as internal/recs writes
// them. A recs item is a hydrated library card plus what the carousel
// needs: the seed titles behind it and its identity in the run.
import type { LibraryCard } from '@app/components/Library/libraryTypes';

export type RecsMediaType = 'all' | 'movie' | 'tv';

export interface RecsItem extends LibraryCard {
  reasons: string[];
  rank: number;
  runId: number;
  onWatchlist: boolean;
}

export interface RecsPage {
  page: number;
  totalPages: number;
  totalResults: number;
  personal: boolean;
  runId: number;
  results: RecsItem[];
}
