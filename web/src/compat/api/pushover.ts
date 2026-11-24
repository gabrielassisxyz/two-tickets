// Copied from Seerr `server/api/pushover.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the sound shape the frontend lists is kept here.
export interface PushoverSound {
  name: string;
  description: string;
}

export const mapSounds = (sounds: {
  [name: string]: string;
}): PushoverSound[] =>
  Object.entries(sounds).map(
    ([name, description]) =>
      ({
        name,
        description,
      }) as PushoverSound
  );
