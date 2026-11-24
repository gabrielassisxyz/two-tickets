// Copied from Seerr `server/interfaces/api/mediaInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type Media from '@app/compat/entity/Media';
import type { User } from '@app/compat/entity/User';
import type { PaginatedResponse } from '@app/compat/interfaces/api/common';

export interface MediaResultsResponse extends PaginatedResponse {
  results: Media[];
}

export interface MediaWatchDataResponse {
  data?: {
    users: User[];
    playCount: number;
    playCount7Days: number;
    playCount30Days: number;
  };
  data4k?: {
    users: User[];
    playCount: number;
    playCount7Days: number;
    playCount30Days: number;
  };
}
