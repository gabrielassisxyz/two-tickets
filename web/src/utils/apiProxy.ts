// Forwards the browser's /api/v1 calls to the api process. It is a route
// rather than a Next rewrite because a rewrite's destination is compiled into
// the build, and the api's address is only known when the image is deployed.
import { getApiBaseUrl } from '@app/utils/urlHelper';
import type {
  IncomingHttpHeaders,
  IncomingMessage,
  ServerResponse,
} from 'http';
import http from 'http';
import https from 'https';
import type { TLSSocket } from 'tls';

// What the api reads from the browser's request. Nothing else is forwarded:
// in particular no Forwarded or X-Forwarded-* header the browser wrote, since
// the api trusts those headers from this process.
const forwardedRequestHeaders = [
  'content-type',
  'accept',
  'cookie',
  'origin',
  'referer',
] as const;

// What goes back besides the status, the body and every Set-Cookie line.
const returnedResponseHeaders = [
  'content-type',
  'cache-control',
  'retry-after',
  'allow',
  'x-content-type-options',
] as const;

// The browser's address as this process saw it, with an IPv4 address mapped
// into IPv6 written plainly, the way the api parses it.
const peerAddress = (req: IncomingMessage): string => {
  const address = req.socket.remoteAddress ?? '';
  return address.startsWith('::ffff:') ? address.slice(7) : address;
};

const peerProtocol = (req: IncomingMessage): 'http' | 'https' =>
  (req.socket as TLSSocket).encrypted ? 'https' : 'http';

// The headers sent to the api: the allowed browser headers, then the
// forwarding headers built from this connection, replacing any the browser
// sent rather than appending to them.
export const buildApiRequestHeaders = (
  req: IncomingMessage
): IncomingHttpHeaders => {
  const headers: IncomingHttpHeaders = {};
  for (const name of forwardedRequestHeaders) {
    const value = req.headers[name];
    if (value !== undefined) {
      headers[name] = value;
    }
  }
  headers['x-forwarded-for'] = peerAddress(req);
  if (req.headers.host) {
    headers['x-forwarded-host'] = req.headers.host;
  }
  headers['x-forwarded-proto'] = peerProtocol(req);
  return headers;
};

const copyApiResponseHeaders = (
  from: IncomingMessage,
  to: ServerResponse
): void => {
  for (const name of returnedResponseHeaders) {
    const value = from.headers[name];
    if (value !== undefined) {
      to.setHeader(name, value);
    }
  }
  // Node keeps every Set-Cookie line as its own array entry; folding them
  // into one header would lose all but the first cookie in the browser.
  const cookies = from.headers['set-cookie'];
  if (cookies !== undefined) {
    to.setHeader('set-cookie', cookies);
  }
};

const answerBadGateway = (res: ServerResponse): void => {
  if (res.headersSent) {
    res.destroy();
    return;
  }
  res.statusCode = 502;
  res.setHeader('content-type', 'application/json; charset=utf-8');
  res.end(JSON.stringify({ message: 'Bad Gateway', errors: [] }));
};

interface ApiTarget {
  base: URL;
  // The path and query sent to the api: the base's own path, then the path
  // and query exactly as the browser sent them. They never go through a URL
  // parser, so nothing is decoded and re-encoded on the way.
  path: string;
}

const nextRequestMeta = Symbol.for('NextInternalRequestMeta');

// Next rebuilds req.url for an API route from the parsed query, which turns
// %20 into + among other changes. The URL as received survives only in
// Next's request metadata, as initURL, absolute or not depending on how the
// server was started. Outside Next (in the tests) req.url is still raw.
// deploy/web-e2e.sh checks this against the built image.
export const rawPathAndQuery = (req: IncomingMessage): string => {
  const meta = (req as unknown as Record<symbol, { initURL?: unknown }>)[
    nextRequestMeta
  ];
  const initURL = typeof meta?.initURL === 'string' ? meta.initURL : '';
  const raw = initURL.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]*/i, '');
  if (raw.startsWith('/')) {
    return raw;
  }
  return req.url ?? '/';
};

const apiTargetFor = (req: IncomingMessage): ApiTarget => {
  const base = new URL(getApiBaseUrl());
  return {
    base,
    path: base.pathname.replace(/\/+$/, '') + rawPathAndQuery(req),
  };
};

export const proxyToApi = (
  req: IncomingMessage,
  res: ServerResponse
): Promise<void> =>
  new Promise((resolve) => {
    let target: ApiTarget;
    try {
      target = apiTargetFor(req);
    } catch (e) {
      // The server's log is the only place an operator sees this.
      // eslint-disable-next-line no-console
      console.error(`api proxy: no api address: ${e.message}`);
      answerBadGateway(res);
      resolve();
      return;
    }
    const transport = target.base.protocol === 'https:' ? https : http;
    const upstream = transport.request(
      {
        protocol: target.base.protocol,
        // URL keeps an IPv6 literal in brackets; http.request wants it bare.
        hostname: target.base.hostname.replace(/^\[(.*)\]$/, '$1'),
        port: target.base.port,
        method: req.method,
        path: target.path,
        headers: buildApiRequestHeaders(req),
      },
      (apiResponse) => {
        res.statusCode = apiResponse.statusCode ?? 502;
        copyApiResponseHeaders(apiResponse, res);
        apiResponse.pipe(res);
        apiResponse.on('error', () => res.destroy());
      }
    );
    upstream.on('error', (e) => {
      // eslint-disable-next-line no-console
      console.error(`api proxy: ${req.method} ${target.path}: ${e.message}`);
      answerBadGateway(res);
    });
    res.on('close', () => {
      upstream.destroy();
      resolve();
    });
    req.pipe(upstream);
  });
