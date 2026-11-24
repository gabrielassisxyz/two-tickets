// Copied from Seerr `server/interfaces/api/personInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
import type {
  PersonCreditCast,
  PersonCreditCrew,
} from '@app/compat/models/Person';

export interface PersonCombinedCreditsResponse {
  id: number;
  cast: PersonCreditCast[];
  crew: PersonCreditCrew[];
}
