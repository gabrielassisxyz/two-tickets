import JustWatchCredit from '@app/components/About/JustWatchCredit';
import Header from '@app/components/Common/Header';
import PageTitle from '@app/components/Common/PageTitle';
import defineMessages from '@app/utils/defineMessages';
import Image from 'next/image';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.About', {
  title: 'About',
  tmdbNotice:
    'This product uses the TMDB API but is not endorsed or certified by TMDB.',
  seerrCredit:
    'Interface built on Seerr, an open-source project under the MIT licence.',
  posterHosting:
    'Posters and images load directly from the TMDB image servers. If they are unreachable, artwork may be missing.',
});

// The credits page: who provides the posters, the descriptions and the
// watch-provider data, and the open-source project the interface is built on.
const About = () => {
  const intl = useIntl();
  const title = intl.formatMessage(messages.title);

  return (
    <>
      <PageTitle title={title} />
      <Header>{title}</Header>
      <div className="mt-6 flex flex-col gap-8">
        <section className="flex flex-col gap-3">
          <div className="relative h-12 w-32">
            <Image
              src="/images/tmdb-logo.svg"
              alt="The Movie Database"
              fill
              sizes="128px"
              style={{ objectFit: 'contain' }}
            />
          </div>
          <p className="text-gray-300">
            {intl.formatMessage(messages.tmdbNotice)}
          </p>
          <p className="text-gray-300">
            {intl.formatMessage(messages.posterHosting)}
          </p>
        </section>
        <section className="flex flex-col gap-3">
          <p className="text-gray-300">
            {intl.formatMessage(messages.seerrCredit)}{' '}
            <a
              href="https://github.com/seerr-team/seerr"
              target="_blank"
              rel="noreferrer"
              className="text-indigo-400 hover:underline"
            >
              seerr-team/seerr
            </a>
          </p>
        </section>
        <section className="flex flex-col gap-3">
          <JustWatchCredit />
        </section>
      </div>
    </>
  );
};

export default About;
