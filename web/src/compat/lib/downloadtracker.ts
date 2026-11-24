// Copied from Seerr `server/lib/downloadtracker.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the item shape the frontend reads is kept here; the
// tracker class itself stays on the server.
import type { MediaType } from '@app/compat/constants/media';

export interface DownloadingItem {
  mediaType: MediaType;
  externalId: number;
  size: number;
  sizeLeft: number;
  status: string;
  timeLeft: string;
  estimatedCompletionTime: Date;
  title: string;
  downloadId: string;
  episode?: {
    seasonNumber: number;
    episodeNumber: number;
    absoluteEpisodeNumber: number;
    id: number;
  };
}
