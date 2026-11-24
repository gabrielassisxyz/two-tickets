// Shape copied from Seerr `server/entity/User.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Password handling and quota computation stay on the
// server; the frontend only reads these fields.
import type { UserType } from '@app/compat/constants/user';
import type Issue from '@app/compat/entity/Issue';
import type { MediaRequest } from '@app/compat/entity/MediaRequest';
import type { UserPushSubscription } from '@app/compat/entity/UserPushSubscription';
import type { Watchlist } from '@app/compat/entity/Watchlist';

export class User {
  id: number;
  displayName: string;
  email: string;
  plexUsername?: string | null;
  jellyfinUsername?: string | null;
  username?: string;
  userType: UserType;
  plexId?: number | null;
  jellyfinUserId?: string | null;
  avatar: string;
  avatarETag?: string | null;
  avatarVersion?: string | null;
  requestCount: number;
  requests: MediaRequest[];
  watchlists: Watchlist[];
  movieQuotaLimit?: number;
  movieQuotaDays?: number;
  tvQuotaLimit?: number;
  tvQuotaDays?: number;
  pushSubscriptions: UserPushSubscription[];
  createdIssues: Issue[];
  createdAt: Date;
  updatedAt: Date;
  permissions = 0;
  warnings: string[] = [];

  constructor(init?: Partial<User>) {
    Object.assign(this, init);
  }
}

export default User;
