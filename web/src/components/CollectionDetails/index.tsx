import type { Collection } from '@app/compat/models/Collection';
import CachedImage from '@app/components/Common/CachedImage';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import Slider from '@app/components/Slider';
import TitleCard from '@app/components/TitleCard';
import globalMessages from '@app/i18n/globalMessages';
import ErrorPage from '@app/pages/_error';
import defineMessages from '@app/utils/defineMessages';
import { uniq } from 'lodash';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { Fragment } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.CollectionDetails', {
  overview: 'Overview',
  numberofmovies: '{count} Movies',
});

interface CollectionDetailsProps {
  collection?: Collection;
}

const CollectionDetails = ({ collection }: CollectionDetailsProps) => {
  const intl = useIntl();
  const router = useRouter();

  const { data, error } = useSWR<Collection>(
    `/api/v1/collection/${router.query.collectionId}`,
    {
      fallbackData: collection,
      revalidateOnMount: true,
    }
  );

  const { data: genres } =
    useSWR<{ id: number; name: string }[]>(`/api/v1/genres/movie`);

  if (!data && !error) {
    return <LoadingSpinner />;
  }

  if (!data) {
    return <ErrorPage statusCode={404} />;
  }

  const collectionAttributes: React.ReactNode[] = [];

  collectionAttributes.push(
    intl.formatMessage(messages.numberofmovies, {
      count: data.parts.length,
    })
  );

  if (genres && data.parts.some((part) => part.genreIds.length)) {
    collectionAttributes.push(
      uniq(
        data.parts.reduce(
          (genresList: number[], curr) => genresList.concat(curr.genreIds),
          []
        )
      )
        .map((genreId) => (
          <Link
            href={`/discover/movies/genre/${genreId}`}
            key={`genre-${genreId}`}
            className="hover:underline"
          >
            {genres.find((g) => g.id === genreId)?.name}
          </Link>
        ))
        .reduce((prev, curr) => (
          <Fragment key={`${prev.key}-${curr.key}`}>
            {intl.formatMessage(globalMessages.delimitedlist, {
              a: prev,
              b: curr,
            })}
          </Fragment>
        ))
    );
  }

  return (
    <div
      className="media-page"
      style={{
        height: 493,
      }}
    >
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
                'linear-gradient(180deg, rgba(24, 24, 27, 0.47) 0%, rgba(24, 24, 27, 1) 100%)',
            }}
          />
        </div>
      )}
      <PageTitle title={data.name} />

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
          <h1>{data.name}</h1>
          <span className="media-attributes">
            {collectionAttributes.length > 0 &&
              collectionAttributes
                .map((t, k) => <span key={k}>{t}</span>)
                .reduce((prev, curr) => (
                  <Fragment key={`${prev.key}-${curr.key}`}>
                    {prev}
                    <span>|</span>
                    {curr}
                  </Fragment>
                ))}
          </span>
        </div>
      </div>
      {data.overview && (
        <div className="media-overview">
          <div className="flex-1">
            <h2>{intl.formatMessage(messages.overview)}</h2>
            <p>{data.overview}</p>
          </div>
        </div>
      )}
      <div className="slider-header">
        <div className="slider-title">
          <span>{intl.formatMessage(globalMessages.movies)}</span>
        </div>
      </div>
      <Slider
        sliderKey="collection-movies"
        isLoading={false}
        isEmpty={data.parts.length === 0}
        items={data.parts.map((title) => (
          <TitleCard
            key={`collection-movie-${title.id}`}
            id={title.id}
            isAddedToWatchlist={title.mediaInfo?.watchlists?.length ?? 0}
            image={title.posterPath}
            status={title.mediaInfo?.status}
            summary={title.overview}
            title={title.title}
            userScore={title.voteAverage}
            year={title.releaseDate}
            mediaType={title.mediaType}
          />
        ))}
      />
      <div className="extra-bottom-space relative" />
    </div>
  );
};

export default CollectionDetails;
