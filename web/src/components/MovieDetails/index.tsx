import type { MovieDetails as MovieDetailsType } from '@app/compat/models/Movie';
import JustWatchCredit from '@app/components/About/JustWatchCredit';
import CachedImage from '@app/components/Common/CachedImage';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import Tooltip from '@app/components/Common/Tooltip';
import ExternalLinkBlock from '@app/components/ExternalLinkBlock';
import TitlePanel from '@app/components/Library/TitlePanel';
import TitleWatchlistButton from '@app/components/Library/TitleWatchlistButton';
import MediaSlider from '@app/components/MediaSlider';
import PersonCard from '@app/components/PersonCard';
import Slider from '@app/components/Slider';
import { UserType, useUser } from '@app/hooks/useUser';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { useRouter } from 'next/router';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.MovieDetails', {
  overview: 'Overview',
  cast: 'Cast',
  recommendations: 'Recommendations',
  similar: 'Similar Titles',
  overviewunavailable: 'Overview unavailable.',
  streamingproviders: 'Currently Streaming On',
});

interface MovieDetailsProps {
  movie?: MovieDetailsType;
}

// The title page shows the catalog record, the cast and the two related
// sliders. Request, issue, manage, blocklist, status, deep-link and external
// rating controls were removed with the request-era features.
const MovieDetails = ({ movie }: MovieDetailsProps) => {
  const { user } = useUser();
  const router = useRouter();
  const intl = useIntl();

  const { data, error } = useSWR<MovieDetailsType>(
    `/api/v1/movie/${router.query.movieId}`,
    {
      fallbackData: movie,
      revalidateOnMount: true,
    }
  );

  if (!data && !error) {
    return <LoadingSpinner />;
  }

  if (!data) {
    return <ErrorPage statusCode={404} />;
  }

  // The display locale is always pt-BR and the region is always BR.
  const streamingProviders =
    data.watchProviders?.find((provider) => provider.iso_3166_1 === 'BR')
      ?.flatrate ?? [];

  return (
    <div className="media-page">
      {data.backdropPath && (
        <div className="media-page-bg-image">
          <CachedImage
            type="tmdb"
            alt=""
            src={`https://image.tmdb.org/t/p/w1920_and_h800_multi_faces/${data.backdropPath}`}
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            fill
            priority
          />
          <div
            className="absolute inset-0"
            style={{
              backgroundImage:
                'linear-gradient(180deg, rgba(17, 24, 39, 0.47) 0%, rgba(17, 24, 39, 1) 100%)',
            }}
          />
        </div>
      )}
      <PageTitle title={data.title} />
      <div className="media-header">
        <div className="media-poster">
          <CachedImage
            type="tmdb"
            src={
              data.posterPath
                ? `https://image.tmdb.org/t/p/w600_and_h900_bestv2${data.posterPath}`
                : '/images/seerr_poster_not_found.png'
            }
            alt=""
            sizes="100vw"
            style={{ width: '100%', height: 'auto' }}
            width={600}
            height={900}
            priority
          />
        </div>
        <div className="media-title">
          <h1 data-testid="media-title">{data.title}</h1>
          {data.tagline && <p className="tagline">{data.tagline}</p>}
        </div>
        <div className="media-actions">
          {user?.userType !== UserType.PLEX && (
            <TitleWatchlistButton
              mediaType="movie"
              tmdbId={data.id}
              title={data.title}
              onWatchlist={!!data.onUserWatchlist}
            />
          )}
        </div>
      </div>
      <TitlePanel mediaType="movie" tmdbId={data.id} />
      <div className="media-overview">
        <div className="flex-1">
          <h2>{intl.formatMessage(messages.overview)}</h2>
          <p>
            {data.overview || intl.formatMessage(messages.overviewunavailable)}
          </p>
        </div>
      </div>
      <ExternalLinkBlock
        mediaType="movie"
        tmdbId={data.id}
        imdbId={data.imdbId}
      />
      {streamingProviders.length > 0 && (
        <div className="media-overview">
          <div className="flex-1">
            <h2>{intl.formatMessage(messages.streamingproviders)}</h2>
            <div className="flex flex-row flex-wrap gap-3">
              {streamingProviders.map((provider) => (
                <Tooltip
                  content={provider.name}
                  key={`provider-${provider.id}`}
                >
                  <span className="opacity-50 transition duration-300 hover:opacity-100">
                    <CachedImage
                      type="tmdb"
                      src={
                        provider.logoPath
                          ? `https://image.tmdb.org/t/p/w45${provider.logoPath}`
                          : '/images/seerr_poster_not_found.png'
                      }
                      alt={provider.name}
                      width={32}
                      height={32}
                      className="rounded-md"
                    />
                  </span>
                </Tooltip>
              ))}
            </div>
            <div className="mt-2">
              <JustWatchCredit />
            </div>
          </div>
        </div>
      )}
      {data.credits.cast.length > 0 && (
        <>
          <div className="slider-header">
            <div className="slider-title">
              <span>{intl.formatMessage(messages.cast)}</span>
            </div>
          </div>
          <Slider
            sliderKey="movie-cast"
            isLoading={false}
            isEmpty={data.credits.cast.length === 0}
            items={data.credits.cast.map((person) => (
              <PersonCard
                key={person.id}
                personId={person.id}
                name={person.name}
                subName={person.character}
                profilePath={person.profilePath}
              />
            ))}
          />
        </>
      )}
      <MediaSlider
        sliderKey="movie-recommendations"
        title={intl.formatMessage(messages.recommendations)}
        url={`/api/v1/movie/${router.query.movieId}/recommendations`}
      />
      <MediaSlider
        sliderKey="movie-similar"
        title={intl.formatMessage(messages.similar)}
        url={`/api/v1/movie/${router.query.movieId}/similar`}
      />
    </div>
  );
};

export default MovieDetails;
