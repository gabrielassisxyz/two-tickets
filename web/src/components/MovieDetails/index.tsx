import TmdbLogo from '@app/assets/services/tmdb.svg';
import type { MovieDetails as MovieDetailsType } from '@app/compat/models/Movie';
import JustWatchCredit from '@app/components/About/JustWatchCredit';
import Button from '@app/components/Common/Button';
import CachedImage from '@app/components/Common/CachedImage';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import type { PlayButtonLink } from '@app/components/Common/PlayButton';
import PlayButton from '@app/components/Common/PlayButton';
import Tag from '@app/components/Common/Tag';
import Tooltip from '@app/components/Common/Tooltip';
import ExternalLinkBlock from '@app/components/ExternalLinkBlock';
import TitlePanel from '@app/components/Library/TitlePanel';
import TitleWatchlistButton from '@app/components/Library/TitleWatchlistButton';
import MediaSlider from '@app/components/MediaSlider';
import PersonCard from '@app/components/PersonCard';
import Slider from '@app/components/Slider';
import useLocale from '@app/hooks/useLocale';
import useSettings from '@app/hooks/useSettings';
import { UserType, useUser } from '@app/hooks/useUser';
import globalMessages from '@app/i18n/globalMessages';
import ErrorPage from '@app/pages/_error';
import { sortCrewPriority } from '@app/utils/creditHelpers';
import defineMessages from '@app/utils/defineMessages';
import {
  ArrowRightCircleIcon,
  CloudIcon,
  FilmIcon,
  TicketIcon,
} from '@heroicons/react/24/outline';
import {
  ChevronDoubleDownIcon,
  ChevronDoubleUpIcon,
} from '@heroicons/react/24/solid';
import { countries } from 'country-flag-icons';
import 'country-flag-icons/3x2/flags.css';
import { uniqBy } from 'lodash';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { Fragment, useMemo, useState } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.MovieDetails', {
  originaltitle: 'Original Title',
  releasedate:
    '{releaseCount, plural, one {Release Date} other {Release Dates}}',
  revenue: 'Revenue',
  budget: 'Budget',
  watchtrailer: 'Watch Trailer',
  originallanguage: 'Original Language',
  overview: 'Overview',
  runtime: '{minutes} minutes',
  cast: 'Cast',
  recommendations: 'Recommendations',
  similar: 'Similar Titles',
  overviewunavailable: 'Overview unavailable.',
  studio: '{studioCount, plural, one {Studio} other {Studios}}',
  viewfullcrew: 'View Full Crew',
  showmore: 'Show More',
  showless: 'Show Less',
  streamingproviders: 'Currently Streaming On',
  productioncountries:
    'Production {countryCount, plural, one {Country} other {Countries}}',
  theatricalrelease: 'Theatrical Release',
  digitalrelease: 'Digital Release',
  physicalrelease: 'Physical Release',
  tmdbuserscore: 'TMDB User Score',
});

interface MovieDetailsProps {
  movie?: MovieDetailsType;
}

// The display locale is always pt-BR and the region is always BR, so the
// release dates, the certification and the providers are Brazil's.
const titleRegion = 'BR';

// Upstream's title page with everything tied to requests, issues, media
// servers, *arr and the Rotten Tomatoes and IMDb scores taken out. The
// library panels stand where the strip put them: the watchlist control in
// the header actions, the title panel between the header and the overview.
const MovieDetails = ({ movie }: MovieDetailsProps) => {
  const settings = useSettings();
  const { user } = useUser();
  const router = useRouter();
  const intl = useIntl();
  const { locale } = useLocale();
  const minStudios = 3;
  const [showMoreStudios, setShowMoreStudios] = useState(false);

  const { data, error } = useSWR<MovieDetailsType>(
    `/api/v1/movie/${router.query.movieId}`,
    {
      fallbackData: movie,
      revalidateOnMount: true,
    }
  );

  const sortedCrew = useMemo(
    () => sortCrewPriority(data?.credits.crew ?? []),
    [data]
  );

  if (!data && !error) {
    return <LoadingSpinner />;
  }

  if (!data) {
    return <ErrorPage statusCode={404} />;
  }

  // The api always sends these lists; a partial record still renders the
  // facts it has instead of taking the page down.
  const productionCompanies = data.productionCompanies ?? [];
  const productionCountries = data.productionCountries ?? [];
  const keywords = data.keywords ?? [];

  const showAllStudios = productionCompanies.length <= minStudios + 1;
  const mediaLinks: PlayButtonLink[] = [];

  const trailerVideo = data.relatedVideos
    ?.filter((r) => r.type === 'Trailer')
    .sort((a, b) => a.size - b.size)
    .pop();
  const trailerUrl =
    trailerVideo?.site === 'YouTube' &&
    settings.currentSettings.youtubeUrl != ''
      ? `${settings.currentSettings.youtubeUrl}${trailerVideo?.key}`
      : trailerVideo?.url;

  if (trailerUrl) {
    mediaLinks.push({
      text: intl.formatMessage(messages.watchtrailer),
      url: trailerUrl,
      svg: <FilmIcon />,
    });
  }

  const releases = data.releases?.results.find(
    (r) => r.iso_3166_1 === titleRegion
  )?.release_dates;

  // Release date types:
  // 1. Premiere
  // 2. Theatrical (limited)
  // 3. Theatrical
  // 4. Digital
  // 5. Physical
  // 6. TV
  const filteredReleases = uniqBy(
    releases?.filter((r) => r.type > 2 && r.type < 6),
    'type'
  );

  const movieAttributes: React.ReactNode[] = [];

  const certification = releases?.find((r) => r.certification)?.certification;
  if (certification) {
    movieAttributes.push(
      <span className="rounded-md border p-0.5 py-0">{certification}</span>
    );
  }

  if (data.runtime) {
    movieAttributes.push(
      intl.formatMessage(messages.runtime, { minutes: data.runtime })
    );
  }

  if (data.genres.length) {
    movieAttributes.push(
      data.genres
        .map((g) => (
          <Link
            href={`/discover/movies?genre=${g.id}`}
            key={`genre-${g.id}`}
            className="hover:underline"
          >
            {g.name}
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

  const streamingProviders =
    data.watchProviders?.find((provider) => provider.iso_3166_1 === titleRegion)
      ?.flatrate ?? [];

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
          <div className="absolute inset-0 bg-gradient-to-b from-gray-900/[.47] to-gray-900" />
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
          <h1 data-testid="media-title">
            {data.title}{' '}
            {data.releaseDate && (
              <span className="media-year">
                ({data.releaseDate.slice(0, 4)})
              </span>
            )}
          </h1>
          <span className="media-attributes">
            {movieAttributes.length > 0 &&
              movieAttributes
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
        <div className="media-actions">
          {user?.userType !== UserType.PLEX && (
            <div className="z-40 mr-2">
              <TitleWatchlistButton
                mediaType="movie"
                tmdbId={data.id}
                title={data.title}
                onWatchlist={!!data.onUserWatchlist}
              />
            </div>
          )}
          <div className="z-20">
            <PlayButton links={mediaLinks} />
          </div>
        </div>
      </div>
      <TitlePanel mediaType="movie" tmdbId={data.id} />
      <div className="media-overview">
        <div className="media-overview-left">
          {data.tagline && <div className="tagline">{data.tagline}</div>}
          <h2>{intl.formatMessage(messages.overview)}</h2>
          <p>
            {data.overview
              ? data.overview
              : intl.formatMessage(messages.overviewunavailable)}
          </p>
          {sortedCrew.length > 0 && (
            <>
              <ul className="media-crew">
                {sortedCrew.slice(0, 6).map((person) => (
                  <li key={`crew-${person.job}-${person.id}`}>
                    <span>{person.job}</span>
                    <Link href={`/person/${person.id}`} className="crew-name">
                      {person.name}
                    </Link>
                  </li>
                ))}
              </ul>
              <div className="mt-4 flex justify-end">
                <Link
                  href={`/movie/${data.id}/crew`}
                  className="flex items-center text-gray-400 transition duration-300 hover:text-gray-100"
                >
                  <span>{intl.formatMessage(messages.viewfullcrew)}</span>
                  <ArrowRightCircleIcon className="ml-1.5 inline-block h-5 w-5" />
                </Link>
              </div>
            </>
          )}
          {keywords.length > 0 && (
            <div className="mt-6">
              {keywords.map((keyword) => (
                <Link
                  href={`/discover/movies?keywords=${keyword.id}`}
                  key={`keyword-id-${keyword.id}`}
                  className="mb-2 mr-2 inline-flex last:mr-0"
                >
                  <Tag>{keyword.name}</Tag>
                </Link>
              ))}
            </div>
          )}
        </div>
        <div className="media-overview-right">
          {data.collection && (
            <div className="mb-6">
              <Link href={`/collection/${data.collection.id}`}>
                <div className="group relative z-0 scale-100 transform-gpu cursor-pointer overflow-hidden rounded-lg bg-gray-800 bg-cover bg-center shadow-md ring-1 ring-gray-700 transition duration-300 hover:scale-105 hover:ring-gray-500">
                  <div className="absolute inset-0 z-0">
                    <CachedImage
                      type="tmdb"
                      src={`https://image.tmdb.org/t/p/w1440_and_h320_multi_faces/${data.collection.backdropPath}`}
                      alt=""
                      style={{
                        width: '100%',
                        height: '100%',
                        objectFit: 'cover',
                      }}
                      fill
                    />
                    <div className="absolute inset-0 bg-gradient-to-b from-gray-800/[.47] to-gray-800/80" />
                  </div>
                  <div className="relative z-10 flex h-full items-center justify-between p-4 text-gray-200 transition duration-300 group-hover:text-white">
                    <div>{data.collection.name}</div>
                    <Button buttonSize="sm">
                      {intl.formatMessage(globalMessages.view)}
                    </Button>
                  </div>
                </div>
              </Link>
            </div>
          )}
          <div className="media-facts">
            {!!data.voteCount && (
              <div className="media-ratings">
                <Tooltip content={intl.formatMessage(messages.tmdbuserscore)}>
                  <a
                    href={`https://www.themoviedb.org/movie/${data.id}?language=${locale}`}
                    className="media-rating"
                    target="_blank"
                    rel="noreferrer"
                  >
                    <TmdbLogo className="mr-1 w-6" />
                    <span>{Math.round(data.voteAverage * 10)}%</span>
                  </a>
                </Tooltip>
              </div>
            )}
            {data.originalTitle &&
              data.originalLanguage !== locale.slice(0, 2) && (
                <div className="media-fact">
                  <span>{intl.formatMessage(messages.originaltitle)}</span>
                  <span className="media-fact-value">{data.originalTitle}</span>
                </div>
              )}
            <div className="media-fact">
              <span>{intl.formatMessage(globalMessages.status)}</span>
              <span className="media-fact-value">{data.status}</span>
            </div>
            {filteredReleases && filteredReleases.length > 0 ? (
              <div className="media-fact">
                <span>
                  {intl.formatMessage(messages.releasedate, {
                    releaseCount: filteredReleases.length,
                  })}
                </span>
                <span className="media-fact-value">
                  {filteredReleases.map((r, i) => (
                    <span
                      className="flex items-center justify-end"
                      key={`release-date-${i}`}
                    >
                      {r.type === 3 ? (
                        // Theatrical
                        <Tooltip
                          content={intl.formatMessage(
                            messages.theatricalrelease
                          )}
                        >
                          <TicketIcon className="h-4 w-4" />
                        </Tooltip>
                      ) : r.type === 4 ? (
                        // Digital
                        <Tooltip
                          content={intl.formatMessage(messages.digitalrelease)}
                        >
                          <CloudIcon className="h-4 w-4" />
                        </Tooltip>
                      ) : (
                        // Physical
                        <Tooltip
                          content={intl.formatMessage(messages.physicalrelease)}
                        >
                          <svg
                            className="h-4 w-4"
                            viewBox="0 0 24 24"
                            xmlns="http://www.w3.org/2000/svg"
                          >
                            <path
                              d="m12 2c-5.5242 0-10 4.4758-10 10 0 5.5242 4.4758 10 10 10 5.5242 0 10-4.4758 10-10 0-5.5242-4.4758-10-10-10zm0 18.065c-4.4476 0-8.0645-3.6169-8.0645-8.0645 0-4.4476 3.6169-8.0645 8.0645-8.0645 4.4476 0 8.0645 3.6169 8.0645 8.0645 0 4.4476-3.6169 8.0645-8.0645 8.0645zm0-14.516c-3.5565 0-6.4516 2.8952-6.4516 6.4516h1.2903c0-2.8468 2.3145-5.1613 5.1613-5.1613zm0 2.9032c-1.9597 0-3.5484 1.5887-3.5484 3.5484s1.5887 3.5484 3.5484 3.5484 3.5484-1.5887 3.5484-3.5484-1.5887-3.5484-3.5484-3.5484zm0 4.8387c-0.71371 0-1.2903-0.57661-1.2903-1.2903s0.57661-1.2903 1.2903-1.2903 1.2903 0.57661 1.2903 1.2903-0.57661 1.2903-1.2903 1.2903z"
                              fill="currentColor"
                            />
                          </svg>
                        </Tooltip>
                      )}
                      <span className="ml-1.5">
                        {intl.formatDate(r.release_date, {
                          year: 'numeric',
                          month: 'long',
                          day: 'numeric',
                          timeZone: 'UTC',
                        })}
                      </span>
                    </span>
                  ))}
                </span>
              </div>
            ) : (
              data.releaseDate && (
                <div className="media-fact">
                  <span>
                    {intl.formatMessage(messages.releasedate, {
                      releaseCount: 1,
                    })}
                  </span>
                  <span className="media-fact-value">
                    {intl.formatDate(data.releaseDate, {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                      timeZone: 'UTC',
                    })}
                  </span>
                </div>
              )
            )}
            {data.revenue > 0 && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.revenue)}</span>
                <span className="media-fact-value">
                  {intl.formatNumber(data.revenue, {
                    currency: 'USD',
                    style: 'currency',
                  })}
                </span>
              </div>
            )}
            {data.budget > 0 && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.budget)}</span>
                <span className="media-fact-value">
                  {intl.formatNumber(data.budget, {
                    currency: 'USD',
                    style: 'currency',
                  })}
                </span>
              </div>
            )}
            {data.originalLanguage && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.originallanguage)}</span>
                <span className="media-fact-value">
                  <Link
                    href={`/discover/movies/language/${data.originalLanguage}`}
                  >
                    {intl.formatDisplayName(data.originalLanguage, {
                      type: 'language',
                      fallback: 'none',
                    }) ??
                      data.spokenLanguages?.find(
                        (lng) => lng.iso_639_1 === data.originalLanguage
                      )?.name}
                  </Link>
                </span>
              </div>
            )}
            {productionCountries.length > 0 && (
              <div className="media-fact">
                <span>
                  {intl.formatMessage(messages.productioncountries, {
                    countryCount: productionCountries.length,
                  })}
                </span>
                <span className="media-fact-value">
                  {productionCountries.map((c) => {
                    return (
                      <span
                        className="flex items-center justify-end"
                        key={`prodcountry-${c.iso_3166_1}`}
                      >
                        {countries.includes(c.iso_3166_1) && (
                          <span
                            className={`mr-1.5 text-xs leading-5 flag:${c.iso_3166_1}`}
                          />
                        )}
                        <span>
                          {intl.formatDisplayName(c.iso_3166_1, {
                            type: 'region',
                            fallback: 'none',
                          }) ?? c.name}
                        </span>
                      </span>
                    );
                  })}
                </span>
              </div>
            )}
            {productionCompanies.length > 0 && (
              <div className="media-fact">
                <span>
                  {intl.formatMessage(messages.studio, {
                    studioCount: productionCompanies.length,
                  })}
                </span>
                <span className="media-fact-value">
                  {productionCompanies
                    .slice(
                      0,
                      showAllStudios || showMoreStudios
                        ? productionCompanies.length
                        : minStudios
                    )
                    .map((s) => {
                      return (
                        <Link
                          href={`/discover/movies/studio/${s.id}`}
                          key={`studio-${s.id}`}
                          className="block"
                        >
                          {s.name}
                        </Link>
                      );
                    })}
                  {!showAllStudios && (
                    <button
                      type="button"
                      onClick={() => {
                        setShowMoreStudios(!showMoreStudios);
                      }}
                    >
                      <span className="flex items-center">
                        {intl.formatMessage(
                          !showMoreStudios
                            ? messages.showmore
                            : messages.showless
                        )}
                        {!showMoreStudios ? (
                          <ChevronDoubleDownIcon className="ml-1 h-4 w-4" />
                        ) : (
                          <ChevronDoubleUpIcon className="ml-1 h-4 w-4" />
                        )}
                      </span>
                    </button>
                  )}
                </span>
              </div>
            )}
            {!!streamingProviders.length && (
              <div className="media-fact flex-col gap-1">
                <span>{intl.formatMessage(messages.streamingproviders)}</span>
                <span className="media-fact-value flex flex-row flex-wrap gap-5">
                  {streamingProviders.map((p) => {
                    return (
                      <Tooltip content={p.name} key={`tooltip-${p.id}`}>
                        <span
                          className="opacity-50 transition duration-300 hover:opacity-100"
                          key={`provider-${p.id}`}
                        >
                          <CachedImage
                            type="tmdb"
                            src={
                              p.logoPath
                                ? `https://image.tmdb.org/t/p/w45${p.logoPath}`
                                : '/images/seerr_poster_not_found.png'
                            }
                            alt={p.name}
                            width={32}
                            height={32}
                            className="rounded-md"
                          />
                        </span>
                      </Tooltip>
                    );
                  })}
                </span>
                <JustWatchCredit />
              </div>
            )}
            <div className="media-fact">
              <ExternalLinkBlock
                mediaType="movie"
                tmdbId={data.id}
                tvdbId={data.externalIds?.tvdbId}
                imdbId={data.externalIds?.imdbId ?? data.imdbId}
              />
            </div>
          </div>
        </div>
      </div>
      {data.credits.cast.length > 0 && (
        <>
          <div className="slider-header">
            <Link
              href="/movie/[movieId]/cast"
              as={`/movie/${data.id}/cast`}
              className="slider-title"
            >
              <span>{intl.formatMessage(messages.cast)}</span>
              <ArrowRightCircleIcon />
            </Link>
          </div>
          <Slider
            sliderKey="cast"
            isLoading={false}
            isEmpty={false}
            items={data.credits.cast.slice(0, 20).map((person) => (
              <PersonCard
                key={`cast-item-${person.id}`}
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
        sliderKey="recommendations"
        title={intl.formatMessage(messages.recommendations)}
        url={`/api/v1/movie/${router.query.movieId}/recommendations`}
        linkUrl={`/movie/${data.id}/recommendations`}
        hideWhenEmpty
      />
      <MediaSlider
        sliderKey="similar"
        title={intl.formatMessage(messages.similar)}
        url={`/api/v1/movie/${router.query.movieId}/similar`}
        linkUrl={`/movie/${data.id}/similar`}
        hideWhenEmpty
      />
      <div className="extra-bottom-space relative" />
    </div>
  );
};

export default MovieDetails;
