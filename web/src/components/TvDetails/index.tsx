import TmdbLogo from '@app/assets/services/tmdb.svg';
import { ANIME_KEYWORD_ID } from '@app/compat/api/themoviedb/constants';
import type { TvDetails as TvDetailsType } from '@app/compat/models/Tv';
import type { Crew } from '@app/compat/models/common';
import JustWatchCredit from '@app/components/About/JustWatchCredit';
import CachedImage from '@app/components/Common/CachedImage';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import type { PlayButtonLink } from '@app/components/Common/PlayButton';
import PlayButton from '@app/components/Common/PlayButton';
import Tag from '@app/components/Common/Tag';
import Tooltip from '@app/components/Common/Tooltip';
import ExternalLinkBlock from '@app/components/ExternalLinkBlock';
import SeasonRows from '@app/components/Library/SeasonRows';
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
import { ArrowRightCircleIcon, FilmIcon } from '@heroicons/react/24/solid';
import { countries } from 'country-flag-icons';
import 'country-flag-icons/3x2/flags.css';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { Fragment, useMemo } from 'react';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.TvDetails', {
  firstAirDate: 'First Air Date',
  nextAirDate: 'Next Air Date',
  originallanguage: 'Original Language',
  overview: 'Overview',
  cast: 'Cast',
  recommendations: 'Recommendations',
  similar: 'Similar Series',
  watchtrailer: 'Watch Trailer',
  overviewunavailable: 'Overview unavailable.',
  originaltitle: 'Original Title',
  showtype: 'Series Type',
  anime: 'Anime',
  network: '{networkCount, plural, one {Network} other {Networks}}',
  viewfullcrew: 'View Full Crew',
  seasons: '{seasonCount, plural, one {# Season} other {# Seasons}}',
  episodeRuntime: 'Episode Runtime',
  episodeRuntimeMinutes: '{runtime} minutes',
  streamingproviders: 'Currently Streaming On',
  productioncountries:
    'Production {countryCount, plural, one {Country} other {Countries}}',
  seasonstitle: 'Seasons',
  tmdbuserscore: 'TMDB User Score',
});

interface TvDetailsProps {
  tv?: TvDetailsType;
}

// The display locale is always pt-BR and the region is always BR, so the
// content rating and the providers are Brazil's.
const seriesRegion = 'BR';

// Upstream's series page with everything tied to requests, issues, media
// servers, *arr and the Rotten Tomatoes score taken out. The library panels
// stand where the strip put them: the watchlist control in the header
// actions, the title panel between the header and the overview, and the
// season rows where upstream's season list stood.
const TvDetails = ({ tv }: TvDetailsProps) => {
  const settings = useSettings();
  const { user } = useUser();
  const router = useRouter();
  const intl = useIntl();
  const { locale } = useLocale();

  const { data, error } = useSWR<TvDetailsType>(
    `/api/v1/tv/${router.query.tvId}`,
    {
      fallbackData: tv,
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
  const seasons = data.seasons ?? [];
  const keywords = data.keywords ?? [];
  const createdBy = data.createdBy ?? [];
  const networks = data.networks ?? [];
  const productionCountries = data.productionCountries ?? [];
  const episodeRunTime = data.episodeRunTime ?? [];

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

  const seriesAttributes: React.ReactNode[] = [];

  const contentRating = data.contentRatings?.results.find(
    (r) => r.iso_3166_1 === seriesRegion
  )?.rating;
  if (contentRating) {
    seriesAttributes.push(
      <span className="rounded-md border p-0.5 py-0">{contentRating}</span>
    );
  }

  // Does NOT include "Specials"
  const seasonCount = seasons.filter(
    (season) => season.seasonNumber !== 0 && season.episodeCount !== 0
  ).length;

  if (seasonCount) {
    seriesAttributes.push(
      intl.formatMessage(messages.seasons, { seasonCount: seasonCount })
    );
  }

  if (data.genres.length) {
    seriesAttributes.push(
      data.genres
        .map((g) => (
          <Link
            href={`/discover/tv?genre=${g.id}`}
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
    data.watchProviders?.find(
      (provider) => provider.iso_3166_1 === seriesRegion
    )?.flatrate ?? [];

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
          <h1 data-testid="media-title">
            {data.name}{' '}
            {data.firstAirDate && (
              <span className="media-year">
                ({data.firstAirDate.slice(0, 4)})
              </span>
            )}
          </h1>
          <span className="media-attributes">
            {seriesAttributes.length > 0 &&
              seriesAttributes
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
                mediaType="tv"
                tmdbId={data.id}
                title={data.name}
                onWatchlist={!!data.onUserWatchlist}
              />
            </div>
          )}
          <div className="z-20">
            <PlayButton links={mediaLinks} />
          </div>
        </div>
      </div>
      <TitlePanel mediaType="tv" tmdbId={data.id} />
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
                {(createdBy.length > 0
                  ? [
                      ...createdBy.map(
                        (person): Partial<Crew> => ({
                          id: person.id,
                          job: 'Creator',
                          name: person.name,
                        })
                      ),
                      ...sortedCrew,
                    ]
                  : sortedCrew
                )
                  .slice(0, 6)
                  .map((person) => (
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
                  href={`/tv/${data.id}/crew`}
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
                  href={`/discover/tv?keywords=${keyword.id}`}
                  key={`keyword-id-${keyword.id}`}
                  className="mb-2 mr-2 inline-flex last:mr-0"
                >
                  <Tag>{keyword.name}</Tag>
                </Link>
              ))}
            </div>
          )}
          {seasons.length > 0 && (
            <>
              <h2 className="py-4">
                {intl.formatMessage(messages.seasonstitle)}
              </h2>
              <SeasonRows tvId={data.id} seasons={seasons} />
            </>
          )}
        </div>
        <div className="media-overview-right">
          <div className="media-facts">
            {!!data.voteCount && (
              <div className="media-ratings">
                <Tooltip content={intl.formatMessage(messages.tmdbuserscore)}>
                  <a
                    href={`https://www.themoviedb.org/tv/${data.id}?language=${locale}`}
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
            {data.originalName &&
              data.originalLanguage !== locale.slice(0, 2) && (
                <div className="media-fact">
                  <span>{intl.formatMessage(messages.originaltitle)}</span>
                  <span className="media-fact-value">{data.originalName}</span>
                </div>
              )}
            {keywords.some((keyword) => keyword.id === ANIME_KEYWORD_ID) && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.showtype)}</span>
                <span className="media-fact-value">
                  {intl.formatMessage(messages.anime)}
                </span>
              </div>
            )}
            <div className="media-fact">
              <span>{intl.formatMessage(globalMessages.status)}</span>
              <span className="media-fact-value">{data.status}</span>
            </div>
            {data.firstAirDate && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.firstAirDate)}</span>
                <span className="media-fact-value">
                  {intl.formatDate(data.firstAirDate, {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                    timeZone: 'UTC',
                  })}
                </span>
              </div>
            )}
            {data.nextEpisodeToAir &&
              data.nextEpisodeToAir.airDate &&
              data.nextEpisodeToAir.airDate !== data.firstAirDate && (
                <div className="media-fact">
                  <span>{intl.formatMessage(messages.nextAirDate)}</span>
                  <span className="media-fact-value">
                    {intl.formatDate(data.nextEpisodeToAir.airDate, {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                      timeZone: 'UTC',
                    })}
                  </span>
                </div>
              )}
            {episodeRunTime.length > 0 && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.episodeRuntime)}</span>
                <span className="media-fact-value">
                  {intl.formatMessage(messages.episodeRuntimeMinutes, {
                    runtime: episodeRunTime[0],
                  })}
                </span>
              </div>
            )}
            {data.originalLanguage && (
              <div className="media-fact">
                <span>{intl.formatMessage(messages.originallanguage)}</span>
                <span className="media-fact-value">
                  <Link href={`/discover/tv/language/${data.originalLanguage}`}>
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
            {networks.length > 0 && (
              <div className="media-fact">
                <span>
                  {intl.formatMessage(messages.network, {
                    networkCount: networks.length,
                  })}
                </span>
                <span className="media-fact-value">
                  {networks
                    .map((n) => (
                      <Link
                        href={`/discover/tv/network/${n.id}`}
                        key={`network-${n.id}`}
                      >
                        {n.name}
                      </Link>
                    ))
                    .reduce((prev, curr) => (
                      <Fragment key={`${prev.key}-${curr.key}`}>
                        {intl.formatMessage(globalMessages.delimitedlist, {
                          a: prev,
                          b: curr,
                        })}
                      </Fragment>
                    ))}
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
                mediaType="tv"
                tmdbId={data.id}
                tvdbId={data.externalIds?.tvdbId}
                imdbId={data.externalIds?.imdbId}
              />
            </div>
          </div>
        </div>
      </div>
      {data.credits.cast.length > 0 && (
        <>
          <div className="slider-header">
            <Link
              href="/tv/[tvId]/cast"
              as={`/tv/${data.id}/cast`}
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
        url={`/api/v1/tv/${router.query.tvId}/recommendations`}
        linkUrl={`/tv/${data.id}/recommendations`}
        hideWhenEmpty
      />
      <MediaSlider
        sliderKey="similar"
        title={intl.formatMessage(messages.similar)}
        url={`/api/v1/tv/${router.query.tvId}/similar`}
        linkUrl={`/tv/${data.id}/similar`}
        hideWhenEmpty
      />
      <div className="extra-bottom-space relative" />
    </div>
  );
};

export default TvDetails;
