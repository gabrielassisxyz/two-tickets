// Copied from Seerr `server/interfaces/api/requestInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type { MediaType } from '@app/compat/constants/media';
import type { MediaRequest } from '@app/compat/entity/MediaRequest';
import type {
  NonFunctionProperties,
  PaginatedResponse,
} from '@app/compat/interfaces/api/common';

export interface RequestResultsResponse extends PaginatedResponse {
  results: (NonFunctionProperties<MediaRequest> & {
    profileName?: string;
    canRemove?: boolean;
  })[];
  serviceErrors: {
    radarr: { id: number; name: string }[];
    sonarr: { id: number; name: string }[];
  };
}

export type MediaRequestBody = {
  mediaType: MediaType;
  mediaId: number;
  tvdbId?: number;
  seasons?: number[] | 'all';
  is4k?: boolean;
  serverId?: number;
  profileId?: number;
  profileName?: string;
  rootFolder?: string;
  languageProfileId?: number;
  userId?: number;
  tags?: number[];
  ignoreQuota?: boolean;
};
