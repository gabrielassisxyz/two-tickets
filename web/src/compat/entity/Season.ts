// Shape copied from Seerr `server/entity/Season.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`).
import type { MediaStatus } from '@app/compat/constants/media';
import type Media from '@app/compat/entity/Media';

class Season {
  id: number;
  seasonNumber: number;
  status: MediaStatus;
  status4k: MediaStatus;
  media: Media;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<Season>) {
    Object.assign(this, init);
  }
}

export default Season;
