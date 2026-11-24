// Copied from Seerr `server/models/Search.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the result shapes the frontend renders are kept
// here; the `map*` helpers stay on the server.
import type Media from '@app/compat/entity/Media';

export type MediaType = 'tv' | 'movie' | 'person' | 'collection';

interface SearchResult {
  id: number;
  mediaType: MediaType;
  popularity: number;
  posterPath?: string;
  backdropPath?: string;
  voteCount: number;
  voteAverage: number;
  genreIds: number[];
  overview: string;
  originalLanguage: string;
  mediaInfo?: Media;
}

export interface MovieResult extends SearchResult {
  mediaType: 'movie';
  title: string;
  originalTitle: string;
  releaseDate: string;
  adult: boolean;
  video: boolean;
  mediaInfo?: Media;
}

export interface TvResult extends SearchResult {
  mediaType: 'tv';
  name: string;
  originalName: string;
  originCountry: string[];
  firstAirDate: string;
}

export interface CollectionResult {
  id: number;
  mediaType: 'collection';
  title: string;
  originalTitle: string;
  adult: boolean;
  posterPath?: string;
  backdropPath?: string;
  overview: string;
  originalLanguage: string;
}

export interface PersonResult {
  id: number;
  name: string;
  popularity: number;
  profilePath?: string;
  adult: boolean;
  mediaType: 'person';
  knownFor: (MovieResult | TvResult)[];
}

export type Results = MovieResult | TvResult | PersonResult | CollectionResult;
