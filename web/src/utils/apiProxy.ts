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
import { BlockList, isIP } from 'net';
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

// An IPv4 address mapped into IPv6 written plainly, the way the api parses it.
const unmapped = (address: string): string =>
  address.startsWith('::ffff:') ? address.slice(7) : address;

// The address this process saw the request come from.
const peerAddress = (req: IncomingMessage): string =>
  unmapped(req.socket.remoteAddress ?? '');

// The proxies whose forwarding headers this process believes, read from
// WEB_TRUSTED_PROXY_CIDRS (comma-separated addresses or CIDRs). In production
// TLS ends at the ingress in front of web: without trusting it, every browser
// would reach the api as the ingress's one address, sharing a single login
// rate limit, and no session cookie would ever be Secure. Empty trusts nobody.
export const parseTrustedProxies = (value: string | undefined): BlockList => {
  const list = new BlockList();
  for (const raw of (value ?? '').split(',')) {
    const entry = raw.trim();
    if (entry === '') {
      continue;
    }
    const [network, prefixText] = entry.split('/');
    const version = isIP(network);
    const prefix =
      prefixText === undefined
        ? version === 6
          ? 128
          : 32
        : Number(prefixText);
    const maxPrefix = version === 6 ? 128 : 32;
    if (
      version === 0 ||
      !Number.isInteger(prefix) ||
      prefix < 0 ||
      prefix > maxPrefix
    ) {
      throw new Error(
        `WEB_TRUSTED_PROXY_CIDRS: ${entry} is not an address or a CIDR`
      );
    }
    list.addSubnet(network, prefix, version === 6 ? 'ipv6' : 'ipv4');
  }
  return list;
};

const isTrusted = (trusted: BlockList, address: string): boolean => {
  const version = isIP(address);
  return (
    version !== 0 && trusted.check(address, version === 6 ? 'ipv6' : 'ipv4')
  );
};

const listEntries = (value: string | string[] | undefined): string[] =>
  (Array.isArray(value) ? value : [value ?? ''])
    .flatMap((item) => item.split(','))
    .map((item) => item.trim())
    .filter((item) => item !== '');

// The browser's address, by the api's own rule: from a trusted proxy, the
// rightmost X-Forwarded-For entry outside the trusted networks (entries to
// its left were written by the browser and prove nothing); from anyone else,
// the peer itself, whatever the headers say.
const clientAddress = (req: IncomingMessage, trusted: BlockList): string => {
  const peer = peerAddress(req);
  if (!isTrusted(trusted, peer)) {
    return peer;
  }
  const entries = listEntries(req.headers['x-forwarded-for']);
  for (let i = entries.length - 1; i >= 0; i--) {
    const entry = unmapped(entries[i]);
    if (isIP(entry) === 0) {
      return peer;
    }
    if (!isTrusted(trusted, entry) || i === 0) {
      return entry;
    }
  }
  return peer;
};

// https when this connection is TLS, or when a trusted proxy says the
// browser's own connection was.
const clientProtocol = (
  req: IncomingMessage,
  trusted: BlockList
): 'http' | 'https' => {
  if ((req.socket as TLSSocket).encrypted) {
    return 'https';
  }
  if (!isTrusted(trusted, peerAddress(req))) {
    return 'http';
  }
  const entries = listEntries(req.headers['x-forwarded-proto']);
  return entries.length > 0 &&
    entries[entries.length - 1].toLowerCase() === 'https'
    ? 'https'
    : 'http';
};

// The headers sent to the api: the allowed browser headers, then the
// forwarding headers rebuilt by the rules above, replacing any the browser
// sent rather than appending to them.
export const buildApiRequestHeaders = (
  req: IncomingMessage,
  trusted: BlockList = parseTrustedProxies(process.env.WEB_TRUSTED_PROXY_CIDRS)
): IncomingHttpHeaders => {
  const headers: IncomingHttpHeaders = {};
  for (const name of forwardedRequestHeaders) {
    const value = req.headers[name];
    if (value !== undefined) {
      headers[name] = value;
    }
  }
  headers['x-forwarded-for'] = clientAddress(req, trusted);
  if (req.headers.host) {
    headers['x-forwarded-host'] = req.headers.host;
  }
  headers['x-forwarded-proto'] = clientProtocol(req, trusted);
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
    let headers: IncomingHttpHeaders;
    try {
      target = apiTargetFor(req);
      headers = buildApiRequestHeaders(req);
    } catch (e) {
      // The server's log is the only place an operator sees this: a missing
      // api address or an unreadable WEB_TRUSTED_PROXY_CIDRS.
      // eslint-disable-next-line no-console
      console.error(`api proxy: misconfigured: ${e.message}`);
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
        headers,
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
