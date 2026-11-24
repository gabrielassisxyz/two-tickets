import Spinner from '@app/assets/spinner.svg';
import type { ProductionCompany, TvNetwork } from '@app/compat/models/common';
import Tag from '@app/components/Common/Tag';
import { BuildingOffice2Icon } from '@heroicons/react/24/outline';
import useSWR from 'swr';

type CompanyTagProps = {
  type: 'studio' | 'network';
  companyId: number;
};

const CompanyTag = ({ companyId, type }: CompanyTagProps) => {
  // One literal per lookup so the static inventory resolves each route.
  const path =
    type === 'studio'
      ? `/api/v1/studio/${companyId}`
      : `/api/v1/network/${companyId}`;
  const { data, error } = useSWR<TvNetwork | ProductionCompany>(path);

  if (!data && !error) {
    return (
      <Tag>
        <Spinner className="h-4 w-4" />
      </Tag>
    );
  }

  return <Tag iconSvg={<BuildingOffice2Icon />}>{data?.name}</Tag>;
};

export default CompanyTag;
