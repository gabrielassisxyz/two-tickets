import defineMessages from '@app/utils/defineMessages';
import { useIntl } from 'react-intl';

const messages = defineMessages('components.About.JustWatchCredit', {
  justwatchcredit: 'Streaming provider data provided by JustWatch.',
});

// A single line credit for the watch-provider data, rendered on the About
// page and next to every provider list on the title pages.
const JustWatchCredit = () => {
  const intl = useIntl();

  return (
    <p className="text-sm text-gray-400">
      <a
        href="https://www.justwatch.com"
        target="_blank"
        rel="noreferrer"
        className="hover:underline"
      >
        {intl.formatMessage(messages.justwatchcredit)}
      </a>
    </p>
  );
};

export default JustWatchCredit;
