// Shape copied from Seerr `server/entity/SeasonRequest.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`).
import type { MediaRequestStatus } from '@app/compat/constants/media';
import type { MediaRequest } from '@app/compat/entity/MediaRequest';

class SeasonRequest {
  id: number;
  seasonNumber: number;
  status: MediaRequestStatus;
  request: MediaRequest;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<SeasonRequest>) {
    Object.assign(this, init);
  }
}

export default SeasonRequest;
