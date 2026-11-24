// Shape copied from Seerr `server/entity/IssueComment.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`).
import type Issue from '@app/compat/entity/Issue';
import type { User } from '@app/compat/entity/User';

class IssueComment {
  id: number;
  user: User;
  issue: Issue;
  message: string;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<IssueComment>) {
    Object.assign(this, init);
  }
}

export default IssueComment;
