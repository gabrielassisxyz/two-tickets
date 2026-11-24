// Copied from Seerr `server/constants/server.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the values the frontend uses are kept here.
export enum MediaServerType {
  PLEX = 1,
  JELLYFIN,
  EMBY,
  NOT_CONFIGURED,
}

export enum ServerType {
  JELLYFIN = 'Jellyfin',
  EMBY = 'Emby',
  PLEX = 'Plex',
}
