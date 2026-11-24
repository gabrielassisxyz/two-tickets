// Copied from Seerr `server/lib/overrideRules.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the result shape the frontend reads is kept here.
export type OverrideRulesResult = {
  rootFolder: string | null;
  profileId: number | null;
  tags: number[] | null;
};
