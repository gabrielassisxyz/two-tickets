// Shape copied from Seerr `server/entity/Watchlist.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`). Watchlist mutation helpers stay on the server.
import type { MediaType } from '@app/compat/constants/media';
import type Media from '@app/compat/entity/Media';
import type { User } from '@app/compat/entity/User';
import type { WatchlistItem } from '@app/compat/interfaces/api/discoverInterfaces';

export class Watchlist implements WatchlistItem {
  id: number;
  ratingKey = '';
  mediaType: MediaType;
  title = '';
  tmdbId: number;
  requestedBy: User;
  media: Media;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<Watchlist>) {
    Object.assign(this, init);
  }
}

export default Watchlist;
