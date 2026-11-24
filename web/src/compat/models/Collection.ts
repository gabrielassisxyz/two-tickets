// Copied from Seerr `server/models/Collection.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the DTO shape is kept here.
import type { MovieResult } from '@app/compat/models/Search';

export interface Collection {
  id: number;
  name: string;
  overview?: string;
  posterPath?: string;
  backdropPath?: string;
  parts: MovieResult[];
}
