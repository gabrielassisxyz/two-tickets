// Copied from Seerr `server/interfaces/api/issueInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type { IssueType } from '@app/compat/constants/issue';
import type Issue from '@app/compat/entity/Issue';
import type { PaginatedResponse } from '@app/compat/interfaces/api/common';

export interface IssueResultsResponse extends PaginatedResponse {
  results: Issue[];
}

export type IssueRequestBody = {
  message: string;
  mediaId: number;
  issueType: IssueType;
  problemSeason?: number;
  problemEpisode?: number;
  userId?: number;
};
