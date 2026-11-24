// Shape copied from Seerr `server/entity/OverrideRule.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`).
class OverrideRule {
  id: number;
  radarrServiceId?: number;
  sonarrServiceId?: number;
  users?: string;
  genre?: string;
  language?: string;
  keywords?: string;
  profileId?: number;
  rootFolder?: string;
  tags?: string;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<OverrideRule>) {
    Object.assign(this, init);
  }
}

export default OverrideRule;
