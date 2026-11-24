// Copied from Seerr `server/interfaces/api/userInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type Media from '@app/compat/entity/Media';
import type { MediaRequest } from '@app/compat/entity/MediaRequest';
import type { User } from '@app/compat/entity/User';
import type { PaginatedResponse } from '@app/compat/interfaces/api/common';

export interface UserResultsResponse extends PaginatedResponse {
  results: User[];
}

export interface UserRequestsResponse extends PaginatedResponse {
  results: MediaRequest[];
}

export interface QuotaStatus {
  days?: number;
  limit?: number;
  used: number;
  remaining?: number;
  restricted: boolean;
}

export interface QuotaResponse {
  movie: QuotaStatus;
  tv: QuotaStatus;
}

export interface UserWatchDataResponse {
  recentlyWatched: Media[];
  playCount: number;
}
