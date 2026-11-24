// Copied from Seerr `server/interfaces/api/serviceInterfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`).
export interface QualityProfile {
  name: string;
  id: number;
}

export interface RootFolder {
  path: string;
  accessible?: boolean;
  freeSpace?: number;
  id?: number;
}

export interface Tag {
  label: string;
  id: number;
}

export interface LanguageProfile {
  name: string;
  id: number;
}

export interface ServiceCommonServer {
  id: number;
  name: string;
  is4k: boolean;
  isDefault: boolean;
  activeProfileId: number;
  activeDirectory: string;
  activeLanguageProfileId?: number;
  activeAnimeProfileId?: number;
  activeAnimeDirectory?: string;
  activeAnimeLanguageProfileId?: number;
  activeTags: number[];
  activeAnimeTags?: number[];
}

export interface ServiceCommonServerWithDetails {
  server: ServiceCommonServer;
  profiles: QualityProfile[];
  rootFolders: Partial<RootFolder>[];
  languageProfiles?: LanguageProfile[];
  tags: Tag[];
}
