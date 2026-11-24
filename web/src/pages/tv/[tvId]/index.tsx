import type { TvDetails as TvDetailsType } from '@app/compat/models/Tv';
import TvDetails from '@app/components/TvDetails';
import { getApiBaseUrl } from '@app/utils/urlHelper';
import axios from 'axios';
import type { GetServerSideProps, NextPage } from 'next';

interface TvPageProps {
  tv?: TvDetailsType;
}

const TvPage: NextPage<TvPageProps> = ({ tv }) => {
  return <TvDetails key={tv?.id} tv={tv} />;
};

export const getServerSideProps: GetServerSideProps<TvPageProps> = async (
  ctx
) => {
  const response = await axios.get<TvDetailsType>(
    `${getApiBaseUrl()}/api/v1/tv/${ctx.query.tvId}`,
    {
      headers: ctx.req?.headers?.cookie
        ? { cookie: ctx.req.headers.cookie }
        : undefined,
    }
  );

  return {
    props: {
      tv: response.data,
    },
  };
};

export default TvPage;
