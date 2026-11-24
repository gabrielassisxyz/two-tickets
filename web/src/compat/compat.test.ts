import { ANIME_KEYWORD_ID } from '@app/compat/api/themoviedb/constants';
import { DISCORD_SNOWFLAKE_REGEX } from '@app/compat/constants/discord';
import { DiscoverSliderType } from '@app/compat/constants/discover';
import { ApiErrorCode } from '@app/compat/constants/error';
import { IssueStatus, IssueType } from '@app/compat/constants/issue';
import {
  MediaRequestStatus,
  MediaStatus,
  MediaType,
} from '@app/compat/constants/media';
import { MediaServerType, ServerType } from '@app/compat/constants/server';
import { UserType } from '@app/compat/constants/user';
import { Permission, hasPermission } from '@app/compat/lib/permissions';
import { describe, expect, it } from 'vitest';

// Every value below mirrors Seerr at the pinned upstream commit
// (`web/UPSTREAM.md`). If upstream changes a value, this test fails and the
// compat copy must be re-examined, not just updated.
describe('compat runtime constants', () => {
  it('pins MediaType', () => {
    expect(MediaType.MOVIE).toBe('movie');
    expect(MediaType.TV).toBe('tv');
  });

  it('pins MediaStatus', () => {
    expect(MediaStatus.UNKNOWN).toBe(1);
    expect(MediaStatus.PENDING).toBe(2);
    expect(MediaStatus.PROCESSING).toBe(3);
    expect(MediaStatus.PARTIALLY_AVAILABLE).toBe(4);
    expect(MediaStatus.AVAILABLE).toBe(5);
    expect(MediaStatus.BLOCKLISTED).toBe(6);
    expect(MediaStatus.DELETED).toBe(7);
  });

  it('pins MediaRequestStatus', () => {
    expect(MediaRequestStatus.PENDING).toBe(1);
    expect(MediaRequestStatus.APPROVED).toBe(2);
    expect(MediaRequestStatus.DECLINED).toBe(3);
    expect(MediaRequestStatus.FAILED).toBe(4);
    expect(MediaRequestStatus.COMPLETED).toBe(5);
  });

  it('pins MediaServerType', () => {
    expect(MediaServerType.PLEX).toBe(1);
    expect(MediaServerType.JELLYFIN).toBe(2);
    expect(MediaServerType.EMBY).toBe(3);
    expect(MediaServerType.NOT_CONFIGURED).toBe(4);
  });

  it('pins ServerType', () => {
    expect(ServerType.JELLYFIN).toBe('Jellyfin');
    expect(ServerType.EMBY).toBe('Emby');
    expect(ServerType.PLEX).toBe('Plex');
  });

  it('pins DiscoverSliderType', () => {
    expect(DiscoverSliderType.RECENTLY_ADDED).toBe(1);
    expect(DiscoverSliderType.RECENT_REQUESTS).toBe(2);
    expect(DiscoverSliderType.PLEX_WATCHLIST).toBe(3);
    expect(DiscoverSliderType.TRENDING).toBe(4);
    expect(DiscoverSliderType.POPULAR_MOVIES).toBe(5);
    expect(DiscoverSliderType.MOVIE_GENRES).toBe(6);
    expect(DiscoverSliderType.UPCOMING_MOVIES).toBe(7);
    expect(DiscoverSliderType.STUDIOS).toBe(8);
    expect(DiscoverSliderType.POPULAR_TV).toBe(9);
    expect(DiscoverSliderType.TV_GENRES).toBe(10);
    expect(DiscoverSliderType.UPCOMING_TV).toBe(11);
    expect(DiscoverSliderType.NETWORKS).toBe(12);
    expect(DiscoverSliderType.TMDB_MOVIE_KEYWORD).toBe(13);
    expect(DiscoverSliderType.TMDB_MOVIE_GENRE).toBe(14);
    expect(DiscoverSliderType.TMDB_TV_KEYWORD).toBe(15);
    expect(DiscoverSliderType.TMDB_TV_GENRE).toBe(16);
    expect(DiscoverSliderType.TMDB_SEARCH).toBe(17);
    expect(DiscoverSliderType.TMDB_STUDIO).toBe(18);
    expect(DiscoverSliderType.TMDB_NETWORK).toBe(19);
    expect(DiscoverSliderType.TMDB_MOVIE_STREAMING_SERVICES).toBe(20);
    expect(DiscoverSliderType.TMDB_TV_STREAMING_SERVICES).toBe(21);
  });

  it('pins Permission', () => {
    expect(Permission.NONE).toBe(0);
    expect(Permission.ADMIN).toBe(2);
    expect(Permission.REQUEST).toBe(32);
    expect(Permission.MANAGE_USERS).toBe(8);
    expect(Permission.VIEW_BLOCKLIST).toBe(1073741824);
  });

  it('keeps hasPermission semantics', () => {
    expect(hasPermission(Permission.ADMIN, Permission.ADMIN)).toBe(true);
    expect(hasPermission(Permission.REQUEST, Permission.ADMIN)).toBe(true);
    expect(hasPermission(Permission.ADMIN, Permission.REQUEST)).toBe(false);
    expect(
      hasPermission(
        [Permission.REQUEST, Permission.MANAGE_REQUESTS],
        Permission.REQUEST
      )
    ).toBe(false);
    expect(
      hasPermission(
        [Permission.REQUEST, Permission.MANAGE_REQUESTS],
        Permission.REQUEST,
        { type: 'or' }
      )
    ).toBe(true);
  });

  it('pins ApiErrorCode', () => {
    expect(ApiErrorCode.InvalidUrl).toBe('INVALID_URL');
    expect(ApiErrorCode.Unauthorized).toBe('UNAUTHORIZED');
    expect(ApiErrorCode.Unknown).toBe('UNKNOWN');
  });

  it('pins IssueType and IssueStatus', () => {
    expect(IssueType.VIDEO).toBe(1);
    expect(IssueType.AUDIO).toBe(2);
    expect(IssueType.SUBTITLES).toBe(3);
    expect(IssueType.OTHER).toBe(4);
    expect(IssueStatus.OPEN).toBe(1);
    expect(IssueStatus.RESOLVED).toBe(2);
  });

  it('pins UserType', () => {
    expect(UserType.PLEX).toBe(1);
    expect(UserType.LOCAL).toBe(2);
    expect(UserType.JELLYFIN).toBe(3);
    expect(UserType.EMBY).toBe(4);
  });

  it('pins the anime keyword id and the discord snowflake pattern', () => {
    expect(ANIME_KEYWORD_ID).toBe(210024);
    expect(DISCORD_SNOWFLAKE_REGEX.test('123456789012345678')).toBe(true);
    expect(DISCORD_SNOWFLAKE_REGEX.test('not-a-snowflake')).toBe(false);
  });
});
