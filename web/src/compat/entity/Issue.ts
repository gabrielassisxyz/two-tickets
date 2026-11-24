// Shape copied from Seerr `server/entity/Issue.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`).
import type { IssueStatus, IssueType } from '@app/compat/constants/issue';
import type IssueComment from '@app/compat/entity/IssueComment';
import type Media from '@app/compat/entity/Media';
import type { User } from '@app/compat/entity/User';

class Issue {
  id: number;
  issueType: IssueType;
  status: IssueStatus;
  problemSeason: number;
  problemEpisode: number;
  media: Media;
  createdBy: User;
  modifiedBy?: User;
  comments: IssueComment[];
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<Issue>) {
    Object.assign(this, init);
  }
}

export default Issue;
