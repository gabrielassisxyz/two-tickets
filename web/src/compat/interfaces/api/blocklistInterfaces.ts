// Copied from Seerr `server/interfaces/api/blocklistInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type { User } from '@app/compat/entity/User';

import type { PaginatedResponse } from '@app/compat/interfaces/api/common';

export interface BlocklistItem {
  tmdbId: number;
  mediaType: 'movie' | 'tv';
  title?: string;
  createdAt?: Date;
  user?: User;
  blocklistedTags?: string;
}

export interface BlocklistResultsResponse extends PaginatedResponse {
  results: BlocklistItem[];
}
