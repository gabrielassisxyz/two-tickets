// Shape copied from Seerr `server/entity/Blocklist.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`).
import type { MediaType } from '@app/compat/constants/media';
import type Media from '@app/compat/entity/Media';
import type { User } from '@app/compat/entity/User';
import type { BlocklistItem } from '@app/compat/interfaces/api/blocklistInterfaces';

export class Blocklist implements BlocklistItem {
  id: number;
  mediaType: MediaType;
  title?: string;
  tmdbId: number;
  user?: User;
  media: Media;
  blocklistedTags?: string;
  createdAt: Date;

  constructor(init?: Partial<Blocklist>) {
    Object.assign(this, init);
  }
}

export default Blocklist;
