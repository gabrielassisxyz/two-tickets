// Shape copied from Seerr `server/entity/UserPushSubscription.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type { User } from '@app/compat/entity/User';

export class UserPushSubscription {
  id: number;
  user: User;
  endpoint: string;
  p256dh: string;
  auth: string;
  userAgent: string;
  createdAt: Date;

  constructor(init?: Partial<UserPushSubscription>) {
    Object.assign(this, init);
  }
}

export default UserPushSubscription;
