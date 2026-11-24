// Copied from Seerr `server/api/themoviedb/interfaces.ts` at the pinned
// upstream commit (`web/UPSTREAM.md`). Only the shapes the frontend passes
// around (genres, keywords, company and keyword search responses) are kept
// here.
interface TmdbPaginatedResponse {
  page: number;
  total_results: number;
  total_pages: number;
}

export interface TmdbGenre {
  id: number;
  name: string;
}

export interface TmdbKeyword {
  id: number;
  name: string;
}

export interface TmdbCompany {
  id: number;
  logo_path?: string;
  name: string;
}

export interface TmdbCompanySearchResponse extends TmdbPaginatedResponse {
  results: TmdbCompany[];
}

export interface TmdbKeywordSearchResponse extends TmdbPaginatedResponse {
  results: TmdbKeyword[];
}
