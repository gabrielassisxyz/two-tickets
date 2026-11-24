import { Permission } from '@app/compat/lib/permissions';
import Blocklist from '@app/components/Blocklist';
import useRouteGuard from '@app/hooks/useRouteGuard';
import type { NextPage } from 'next';

const BlocklistPage: NextPage = () => {
  useRouteGuard([Permission.MANAGE_BLOCKLIST, Permission.VIEW_BLOCKLIST], {
    type: 'or',
  });
  return <Blocklist />;
};

export default BlocklistPage;
