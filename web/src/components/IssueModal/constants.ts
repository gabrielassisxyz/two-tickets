import { IssueType } from '@app/compat/constants/issue';
import defineMessages from '@app/utils/defineMessages';
import type { MessageDescriptor } from 'react-intl';

const messages = defineMessages('components.IssueModal', {
  issueAudio: 'Audio',
  issueVideo: 'Video',
  issueSubtitles: 'Subtitle',
  issueOther: 'Other',
});

interface IssueOption {
  name: MessageDescriptor;
  issueType: IssueType;
  mediaType?: 'movie' | 'tv';
}

export const issueOptions: IssueOption[] = [
  {
    name: messages.issueVideo,
    issueType: IssueType.VIDEO,
  },
  {
    name: messages.issueAudio,
    issueType: IssueType.AUDIO,
  },
  {
    name: messages.issueSubtitles,
    issueType: IssueType.SUBTITLES,
  },
  {
    name: messages.issueOther,
    issueType: IssueType.OTHER,
  },
];
