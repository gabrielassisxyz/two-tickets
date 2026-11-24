// Copied from Seerr `server/models/Movie.ts` at the pinned upstream commit
// (`web/UPSTREAM.md`). Only the details shape is kept here.
import type Media from '@app/compat/entity/Media';
import type {
  Cast,
  Crew,
  ExternalIds,
  Genre,
  Keyword,
  ProductionCompany,
  WatchProviders,
} from '@app/compat/models/common';

export interface Video {
  url?: string;
  site: 'YouTube';
  key: string;
  name: string;
  size: number;
  type:
    | 'Clip'
    | 'Teaser'
    | 'Trailer'
    | 'Featurette'
    | 'Opening Credits'
    | 'Behind the Scenes'
    | 'Bloopers';
}

export interface MovieRelease {
  certification: string;
  iso_639_1?: string;
  note?: string;
  release_date: string;
  type: number;
}

export interface MovieDetails {
  id: number;
  imdbId?: string;
  adult: boolean;
  backdropPath?: string;
  budget: number;
  genres: Genre[];
  homepage?: string;
  originalLanguage: string;
  originalTitle: string;
  overview?: string;
  popularity: number;
  relatedVideos?: Video[];
  posterPath?: string;
  productionCompanies: ProductionCompany[];
  productionCountries: {
    iso_3166_1: string;
    name: string;
  }[];
  releaseDate: string;
  releases: {
    results: {
      iso_3166_1: string;
      release_dates: MovieRelease[];
    }[];
  };
  revenue: number;
  runtime?: number;
  spokenLanguages: {
    iso_639_1: string;
    name: string;
  }[];
  status: string;
  tagline?: string;
  title: string;
  video: boolean;
  voteAverage: number;
  voteCount: number;
  credits: {
    cast: Cast[];
    crew: Crew[];
  };
  collection?: {
    id: number;
    name: string;
    posterPath?: string;
    backdropPath?: string;
  };
  mediaInfo?: Media;
  externalIds: ExternalIds;
  mediaUrl?: string;
  watchProviders?: WatchProviders[];
  keywords: Keyword[];
  onUserWatchlist?: boolean;
}
