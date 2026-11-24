import { DiscoverSliderType } from '@app/compat/constants/discover';
import type DiscoverSlider from '@app/compat/entity/DiscoverSlider';
import { SliderId } from '@app/compat/sliderIds';
import LoadingSpinner from '@app/components/Common/LoadingSpinner';
import PageTitle from '@app/components/Common/PageTitle';
import MovieGenreSlider from '@app/components/Discover/MovieGenreSlider';
import NetworkSlider from '@app/components/Discover/NetworkSlider';
import StudioSlider from '@app/components/Discover/StudioSlider';
import TvGenreSlider from '@app/components/Discover/TvGenreSlider';
import { sliderTitles } from '@app/components/Discover/constants';
import ForYouSlider from '@app/components/ForYou/ForYouSlider';
import LibraryListSlider from '@app/components/Library/LibraryListSlider';
import MediaSlider from '@app/components/MediaSlider';
import defineMessages from '@app/utils/defineMessages';
import type { IntlShape } from 'react-intl';
import { useIntl } from 'react-intl';
import useSWR from 'swr';

const messages = defineMessages('components.Discover', {
  discover: 'Discover',
  foryou: 'For you',
  myList: 'My list',
  coupleList: 'Couple list',
});

// The api serves a fixed slider manifest from /api/v1/settings/discover: the
// kept built-ins, then this app's own slider types, in the api's order. There
// is no customization and no edit UI. A type this page does not render yet
// shows nothing, so a manifest entry can land before its component does.
const Discover = () => {
  const intl = useIntl();
  const { data: sliders, error } = useSWR<DiscoverSlider[]>(
    '/api/v1/settings/discover'
  );

  if (!sliders && !error) {
    return <LoadingSpinner />;
  }

  const now = new Date();
  const offset = now.getTimezoneOffset();
  const upcomingDate = new Date(now.getTime() - offset * 60 * 1000)
    .toISOString()
    .split('T')[0];

  return (
    <>
      <PageTitle title={intl.formatMessage(messages.discover)} />
      {(sliders ?? [])
        .filter((slider) => slider.enabled)
        .map((slider) => {
          const sliderComponent = discoverSliderFor(slider, intl, upcomingDate);
          if (!sliderComponent) {
            return null;
          }
          return (
            <div key={`discover-slider-${slider.id}`}>{sliderComponent}</div>
          );
        })}
    </>
  );
};

const discoverSliderFor = (
  slider: DiscoverSlider,
  intl: IntlShape,
  upcomingDate: string
): React.ReactNode => {
  // The manifest carries upstream types and this app's own, from 100.
  switch (slider.type as DiscoverSliderType | SliderId) {
    case DiscoverSliderType.TRENDING:
      return (
        <MediaSlider
          sliderKey="trending"
          title={intl.formatMessage(sliderTitles.trending)}
          url="/api/v1/discover/trending"
          linkUrl="/discover/trending"
        />
      );
    case DiscoverSliderType.POPULAR_MOVIES:
      return (
        <MediaSlider
          sliderKey="popular-movies"
          title={intl.formatMessage(sliderTitles.popularmovies)}
          url="/api/v1/discover/movies"
          linkUrl="/discover/movies"
        />
      );
    case DiscoverSliderType.MOVIE_GENRES:
      return <MovieGenreSlider />;
    case DiscoverSliderType.UPCOMING_MOVIES:
      return (
        <MediaSlider
          sliderKey="upcoming"
          title={intl.formatMessage(sliderTitles.upcoming)}
          linkUrl={`/discover/movies?primaryReleaseDateGte=${upcomingDate}`}
          url="/api/v1/discover/movies"
          extraParams={`primaryReleaseDateGte=${upcomingDate}`}
        />
      );
    case DiscoverSliderType.STUDIOS:
      return <StudioSlider />;
    case DiscoverSliderType.POPULAR_TV:
      return (
        <MediaSlider
          sliderKey="popular-tv"
          title={intl.formatMessage(sliderTitles.populartv)}
          url="/api/v1/discover/tv"
          linkUrl="/discover/tv"
        />
      );
    case DiscoverSliderType.TV_GENRES:
      return <TvGenreSlider />;
    case DiscoverSliderType.UPCOMING_TV:
      return (
        <MediaSlider
          sliderKey="upcoming-tv"
          title={intl.formatMessage(sliderTitles.upcomingtv)}
          linkUrl={`/discover/tv?firstAirDateGte=${upcomingDate}`}
          url="/api/v1/discover/tv"
          extraParams={`firstAirDateGte=${upcomingDate}`}
        />
      );
    case DiscoverSliderType.NETWORKS:
      return <NetworkSlider />;
    case SliderId.MY_LIST:
      return (
        <LibraryListSlider
          kind="watchlist"
          sliderKey="my-list"
          title={intl.formatMessage(messages.myList)}
        />
      );
    case SliderId.COUPLE_LIST:
      return (
        <LibraryListSlider
          kind="couple"
          sliderKey="couple-list"
          title={intl.formatMessage(messages.coupleList)}
        />
      );
    case SliderId.FOR_YOU:
      return (
        <ForYouSlider
          sliderKey="for-you"
          title={intl.formatMessage(messages.foryou)}
        />
      );
    default:
      return null;
  }
};

export default Discover;
