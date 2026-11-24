// Copied from Seerr `server/models/Tv.ts` at the pinned upstream commit
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
import type { Video } from '@app/compat/models/Movie';

export interface Episode {
  id: number;
  airDate: string | null;
  episodeNumber: number;
  name: string;
  overview: string;
  productionCode: string;
  seasonNumber: number;
  showId: number;
  stillPath?: string;
  voteAverage: number;
  voteCount: number;
}

export interface Season {
  airDate: string;
  id: number;
  episodeCount: number;
  name: string;
  overview: string;
  posterPath?: string;
  seasonNumber: number;
}

export interface SeasonWithEpisodes extends Omit<Season, 'episodeCount'> {
  episodes: Episode[];
  externalIds: ExternalIds;
}

interface SpokenLanguage {
  englishName: string;
  iso_639_1: string;
  name: string;
}

export interface TvDetails {
  id: number;
  adult?: boolean;
  backdropPath?: string;
  posterPath?: string;
  contentRatings: {
    results: {
      iso_3166_1: string;
      rating: string;
    }[];
  };
  createdBy: {
    id: number;
    creditId?: string;
    name: string;
    gender: number;
    profilePath?: string;
  }[];
  episodeRunTime: number[];
  firstAirDate?: string;
  genres: Genre[];
  homepage: string;
  inProduction: boolean;
  relatedVideos?: Video[];
  languages: string[];
  lastAirDate: string;
  lastEpisodeToAir?: Episode;
  name: string;
  nextEpisodeToAir?: Episode;
  networks: {
    id: number;
    name: string;
    logoPath?: string;
    originCountry?: string;
    headquarters?: string;
    homepage?: string;
  }[];
  numberOfEpisodes: number;
  numberOfSeasons: number;
  originCountry: string[];
  originalLanguage: string;
  originalName: string;
  overview: string;
  popularity: number;
  productionCompanies: ProductionCompany[];
  productionCountries: {
    iso_3166_1: string;
    name: string;
  }[];
  spokenLanguages: SpokenLanguage[];
  seasons: Season[];
  status: string;
  tagline?: string;
  type: string;
  voteAverage: number;
  voteCount: number;
  credits: {
    cast: Cast[];
    crew: Crew[];
  };
  externalIds: ExternalIds;
  keywords: Keyword[];
  mediaInfo?: Media;
  watchProviders?: WatchProviders[];
  onUserWatchlist?: boolean;
}
