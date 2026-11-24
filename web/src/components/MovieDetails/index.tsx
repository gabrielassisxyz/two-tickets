import type { MovieDetails as MovieDetailsType } from '@app/compat/models/Movie';
import { MediaType } from '@app/compat/constants/media';
import Button from '@app/components/Common/Button';
import CachedImage from '@app/components/Common/CachedImage';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import ExternalLinkBlock from '@app/components/ExternalLinkBlock';
import MediaSlider from '@app/components/MediaSlider';
import PersonCard from '@app/components/PersonCard';
import Slider from '@app/components/Slider';
import useToasts from '@app/hooks/useToasts';
import { UserType, useUser } from '@app/hooks/useUser';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { MinusCircleIcon, StarIcon } from '@heroicons/react/24/outline';
import axios from 'axios';
import { useRouter } from 'next/router';
import { useState } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.MovieDetails', {
  overview: 'Overview',
  cast: 'Cast',
  recommendations: 'Recommendations',
  similar: 'Similar Titles',
  overviewunavailable: 'Overview unavailable.',
  watchlistSuccess: '<strong>{title}</strong> added to watchlist successfully!',
  watchlistDeleted:
    '<strong>{title}</strong> Removed from watchlist successfully!',
  watchlistError: 'Something went wrong. Please try again.',
  removefromwatchlist: 'Remove From Watchlist',
  addtowatchlist: 'Quero ver',
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
  const [isUpdating, setIsUpdating] = useState<boolean>(false);
  const [toggleWatchlist, setToggleWatchlist] = useState<boolean>(
    !movie?.onUserWatchlist
  );
  const { addToast } = useToasts();

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

  const onClickWatchlistBtn = async (): Promise<void> => {
    setIsUpdating(true);
    try {
      const response = await axios.post('/api/v1/watchlist', {
        tmdbId: data.id,
        mediaType: MediaType.MOVIE,
        title: data.title,
      });
      if (response.data) {
        addToast(
          <span>
            {intl.formatMessage(messages.watchlistSuccess, {
              title: data.title,
              strong: (msg: React.ReactNode) => <strong>{msg}</strong>,
            })}
          </span>,
          { appearance: 'success', autoDismiss: true }
        );
      }
    } catch {
      addToast(intl.formatMessage(messages.watchlistError), {
        appearance: 'error',
        autoDismiss: true,
      });
    } finally {
      setIsUpdating(false);
      setToggleWatchlist((prevState) => !prevState);
    }
  };

  const onClickDeleteWatchlistBtn = async (): Promise<void> => {
    setIsUpdating(true);
    try {
      const response = await axios.delete(
        `/api/v1/watchlist/${data.id}?mediaType=${MediaType.MOVIE}`
      );

      if (response.status === 204) {
        addToast(
          <span>
            {intl.formatMessage(messages.watchlistDeleted, {
              title: data.title,
              strong: (msg: React.ReactNode) => <strong>{msg}</strong>,
            })}
          </span>,
          { appearance: 'info', autoDismiss: true }
        );
      }
    } catch {
      addToast(intl.formatMessage(messages.watchlistError), {
        appearance: 'error',
        autoDismiss: true,
      });
    } finally {
      setIsUpdating(false);
      setToggleWatchlist((prevState) => !prevState);
    }
  };

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
          {user?.userType !== UserType.PLEX &&
            (toggleWatchlist ? (
              <Button
                buttonType="primary"
                onClick={onClickWatchlistBtn}
                disabled={isUpdating}
                title={intl.formatMessage(messages.addtowatchlist)}
              >
                <StarIcon />
                <span>{intl.formatMessage(messages.addtowatchlist)}</span>
              </Button>
            ) : (
              <Button
                buttonType="ghost"
                onClick={onClickDeleteWatchlistBtn}
                disabled={isUpdating}
                title={intl.formatMessage(messages.removefromwatchlist)}
              >
                <MinusCircleIcon />
                <span>{intl.formatMessage(messages.removefromwatchlist)}</span>
              </Button>
            ))}
        </div>
      </div>
      <div className="media-overview">
        <div className="flex-1">
          <h2>{intl.formatMessage(messages.overview)}</h2>
          <p>{data.overview || intl.formatMessage(messages.overviewunavailable)}</p>
        </div>
      </div>
      <ExternalLinkBlock
        mediaType="movie"
        tmdbId={data.id}
        imdbId={data.imdbId}
      />
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
