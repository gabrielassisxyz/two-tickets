import MovieGenreSlider from '@app/components/Discover/MovieGenreSlider';
import NetworkSlider from '@app/components/Discover/NetworkSlider';
import StudioSlider from '@app/components/Discover/StudioSlider';
import TvGenreSlider from '@app/components/Discover/TvGenreSlider';
import { sliderTitles } from '@app/components/Discover/constants';
import MediaSlider from '@app/components/MediaSlider';
import PageTitle from '@app/components/Common/PageTitle';
import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.Discover', {
  discover: 'Descobrir',
});

// The Discover page is a fixed manifest: the same sliders in the same order
// on every load. There is no server-stored customization, no edit UI and no
// request, recently-added or Plex watchlist sliders.
const Discover = () => {
  const intl = useIntl();

  const now = new Date();
  const offset = now.getTimezoneOffset();
  const upcomingDate = new Date(now.getTime() - offset * 60 * 1000)
    .toISOString()
    .split('T')[0];

  return (
    <>
      <PageTitle title={intl.formatMessage(messages.discover)} />
      <MediaSlider
        sliderKey="trending"
        title={intl.formatMessage(sliderTitles.trending)}
        url="/api/v1/discover/trending"
        linkUrl="/discover/trending"
      />
      <MediaSlider
        sliderKey="popular-movies"
        title={intl.formatMessage(sliderTitles.popularmovies)}
        url="/api/v1/discover/movies"
        linkUrl="/discover/movies"
      />
      <MediaSlider
        sliderKey="popular-tv"
        title={intl.formatMessage(sliderTitles.populartv)}
        url="/api/v1/discover/tv"
        linkUrl="/discover/tv"
      />
      <MediaSlider
        sliderKey="upcoming"
        title={intl.formatMessage(sliderTitles.upcoming)}
        linkUrl={`/discover/movies?primaryReleaseDateGte=${upcomingDate}`}
        url="/api/v1/discover/movies"
        extraParams={`primaryReleaseDateGte=${upcomingDate}`}
      />
      <MediaSlider
        sliderKey="upcoming-tv"
        title={intl.formatMessage(sliderTitles.upcomingtv)}
        linkUrl={`/discover/tv?firstAirDateGte=${upcomingDate}`}
        url="/api/v1/discover/tv"
        extraParams={`firstAirDateGte=${upcomingDate}`}
      />
      <MovieGenreSlider />
      <TvGenreSlider />
      <StudioSlider />
      <NetworkSlider />
    </>
  );
};

export default Discover;
