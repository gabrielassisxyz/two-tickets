import Spinner from '@app/assets/spinner.svg';
import type { Keyword } from '@app/compat/models/common';
import Tag from '@app/components/Common/Tag';
import useSWR from 'swr';

type KeywordTagProps = {
  keywordId: number;
};

const KeywordTag = ({ keywordId }: KeywordTagProps) => {
  const { data, error } = useSWR<Keyword>(`/api/v1/keyword/${keywordId}`);

  if (!data && !error) {
    return (
      <Tag>
        <Spinner className="h-4 w-4" />
      </Tag>
    );
  }

  return <Tag>{data?.name}</Tag>;
};

export default KeywordTag;
