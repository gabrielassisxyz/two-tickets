// Shape copied from Seerr `server/entity/MediaRequest.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`). Request-creation logic stays on the server; the
// frontend only reads these fields.
import type {
  MediaRequestStatus,
  MediaType,
} from '@app/compat/constants/media';
import type Media from '@app/compat/entity/Media';
import type SeasonRequest from '@app/compat/entity/SeasonRequest';
import type { User } from '@app/compat/entity/User';

export class MediaRequest {
  id: number;
  status: MediaRequestStatus;
  media: Media;
  requestedBy: User;
  modifiedBy?: User;
  createdAt: Date;
  updatedAt: Date;
  type: MediaType;
  seasonCount: number;
  seasons: SeasonRequest[];
  is4k: boolean;
  serverId: number;
  profileId: number;
  rootFolder: string;
  languageProfileId: number;
  tags?: number[];
  isAutoRequest: boolean;
  ignoreQuota: boolean;

  constructor(init?: Partial<MediaRequest>) {
    Object.assign(this, init);
  }
}

export default MediaRequest;
