// Shape copied from Seerr `server/entity/DiscoverSlider.ts` at the pinned upstream
// commit (`web/UPSTREAM.md`). The bootstrap routine stays on the server.
import type { DiscoverSliderType } from '@app/compat/constants/discover';

class DiscoverSlider {
  id: number;
  type: DiscoverSliderType;
  order: number;
  isBuiltIn: boolean;
  enabled: boolean;
  title?: string;
  data?: string;
  createdAt: Date;
  updatedAt: Date;

  constructor(init?: Partial<DiscoverSlider>) {
    Object.assign(this, init);
  }
}

export default DiscoverSlider;
