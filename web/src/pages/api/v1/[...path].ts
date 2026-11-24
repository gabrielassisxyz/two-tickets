import { proxyToApi } from '@app/utils/apiProxy';
import type { NextApiRequest, NextApiResponse } from 'next';

// The body is streamed to the api untouched, so Next must not read or size it.
export const config = {
  api: {
    bodyParser: false,
    externalResolver: true,
    responseLimit: false,
  },
};

export default function apiV1Proxy(
  req: NextApiRequest,
  res: NextApiResponse
): Promise<void> {
  return proxyToApi(req, res);
}
