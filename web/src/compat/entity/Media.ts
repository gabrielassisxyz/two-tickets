// Shape copied from Seerr `server/entity/Media.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Persistence decorators and server-only helpers are left
// out; the frontend only reads these fields from API responses.
import type { MediaStatus, MediaType } from '@app/compat/constants/media';
import type { DownloadingItem } from '@app/compat/lib/downloadtracker';

import type { Blocklist } from '@app/compat/entity/Blocklist';
import type Issue from '@app/compat/entity/Issue';
import type { MediaRequest } from '@app/compat/entity/MediaRequest';
import type Season from '@app/compat/entity/Season';
import type { User } from '@app/compat/entity/User';
import type { Watchlist } from '@app/compat/entity/Watchlist';

class Media {
  id: number;
  mediaType: MediaType;
  tmdbId: number;
  tvdbId?: number;
  imdbId?: string;
  status: MediaStatus;
  status4k: MediaStatus;
  requests: MediaRequest[];
  watchlists: Watchlist[];
  seasons: Season[];
  issues: Issue[];
  blocklist: Blocklist;
  createdAt: Date;
  updatedAt: Date;
  lastSeasonChange: Date;
  mediaAddedAt: Date;
  serviceId?: number | null;
  serviceId4k?: number | null;
  externalServiceId?: number | null;
  externalServiceId4k?: number | null;
  externalServiceSlug?: string | null;
  externalServiceSlug4k?: string | null;
  ratingKey?: string | null;
  ratingKey4k?: string | null;
  jellyfinMediaId?: string | null;
  jellyfinMediaId4k?: string | null;
  serviceUrl?: string;
  serviceUrl4k?: string;
  hasActiveRequest?: boolean;
  downloadStatus?: DownloadingItem[];
  downloadStatus4k?: DownloadingItem[];
  mediaUrl?: string;
  mediaUrl4k?: string;
  iOSPlexUrl?: string;
  iOSPlexUrl4k?: string;
  tautulliUrl?: string;
  tautulliUrl4k?: string;

  constructor(init?: Partial<Media>) {
    Object.assign(this, init);
  }
}

export default Media;
export type { User };
