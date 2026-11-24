import {
  buildApiRequestHeaders,
  proxyToApi,
  rawPathAndQuery,
} from '@app/utils/apiProxy';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import type {
  IncomingHttpHeaders,
  IncomingMessage,
  RequestOptions,
  Server,
  ServerResponse,
} from 'node:http';
import http from 'node:http';
import https from 'node:https';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

interface ReceivedByApi {
  method: string;
  url: string;
  headers: IncomingHttpHeaders;
  bodySha256: string;
  bodyBytes: number;
}

// The fake api records what reached it and answers what the test asks for
// through the path.
const received: ReceivedByApi[] = [];

const answerAsFakeApi = (
  req: IncomingMessage,
  body: Buffer,
  res: ServerResponse
): void => {
  const path = (req.url ?? '').split('?')[0];
  if (path === '/api/v1/two-cookies') {
    res.setHeader('set-cookie', ['first=1; Path=/', 'second=2; Path=/']);
  }
  if (path === '/api/v1/auth/local') {
    // The api's own rule, which the Go tests prove: Secure follows the
    // X-Forwarded-Proto this proxy sends.
    const secure = req.headers['x-forwarded-proto'] === 'https';
    res.setHeader(
      'set-cookie',
      `two_tickets_session=token; Path=/; HttpOnly${secure ? '; Secure' : ''}`
    );
  }
  if (path === '/api/v1/auth/me') {
    res.statusCode = 401;
  }
  res.setHeader('content-type', 'application/json; charset=utf-8');
  res.setHeader('x-not-forwarded', 'api-internal');
  res.end(JSON.stringify({ bytes: body.length }));
};

const fakeApi = http.createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on('data', (chunk: Buffer) => chunks.push(chunk));
  req.on('end', () => {
    const body = Buffer.concat(chunks);
    received.push({
      method: req.method ?? '',
      url: req.url ?? '',
      headers: req.headers,
      bodySha256: createHash('sha256').update(body).digest('hex'),
      bodyBytes: body.length,
    });
    answerAsFakeApi(req, body, res);
  });
});

const proxyHandler = (req: IncomingMessage, res: ServerResponse): void => {
  void proxyToApi(req, res);
};

const plainProxy = http.createServer(proxyHandler);
let tlsProxy: Server;
let certDir = '';

const listen = (server: Server): Promise<number> =>
  new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () =>
      resolve((server.address() as AddressInfo).port)
    );
  });

const close = (server: Server): Promise<void> =>
  new Promise((resolve) => server.close(() => resolve()));

let apiPort = 0;
let plainPort = 0;
let tlsPort = 0;

beforeAll(async () => {
  // A throwaway certificate, made for this run only, so no key is committed.
  certDir = mkdtempSync(join(tmpdir(), 'api-proxy-test-'));
  execFileSync(
    'openssl',
    [
      'req',
      '-x509',
      '-newkey',
      'ec',
      '-pkeyopt',
      'ec_paramgen_curve:prime256v1',
      '-nodes',
      '-days',
      '1',
      '-subj',
      '/CN=localhost',
      '-keyout',
      join(certDir, 'key.pem'),
      '-out',
      join(certDir, 'cert.pem'),
    ],
    { stdio: 'ignore' }
  );
  tlsProxy = https.createServer(
    {
      key: readFileSync(join(certDir, 'key.pem')),
      cert: readFileSync(join(certDir, 'cert.pem')),
    },
    proxyHandler
  );
  apiPort = await listen(fakeApi);
  plainPort = await listen(plainProxy);
  tlsPort = await listen(tlsProxy);
  process.env.API_INTERNAL_URL = `http://127.0.0.1:${apiPort}`;
});

afterAll(async () => {
  await Promise.all([close(fakeApi), close(plainProxy), close(tlsProxy)]);
  rmSync(certDir, { recursive: true, force: true });
  delete process.env.API_INTERNAL_URL;
});

afterEach(() => {
  received.length = 0;
});

interface ProxyAnswer {
  status: number;
  headers: IncomingHttpHeaders;
  body: string;
}

// Sends one request through the proxy. chunks are written one by one, with a
// pause between them, so the body really arrives as a stream.
const throughProxy = (
  options: RequestOptions & { tls?: boolean },
  chunks: Buffer[] = []
): Promise<ProxyAnswer> =>
  new Promise((resolve, reject) => {
    const transport = options.tls ? https : http;
    const req = transport.request(
      {
        host: '127.0.0.1',
        port: options.tls ? tlsPort : plainPort,
        rejectUnauthorized: false,
        ...options,
      },
      (res) => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (chunk: string) => (body += chunk));
        res.on('end', () =>
          resolve({ status: res.statusCode ?? 0, headers: res.headers, body })
        );
      }
    );
    req.on('error', reject);
    const writeNext = (i: number): void => {
      if (i === chunks.length) {
        req.end();
        return;
      }
      req.write(chunks[i]);
      setTimeout(() => writeNext(i + 1), 2);
    };
    writeNext(0);
  });

const fakeRequest = (
  headers: IncomingHttpHeaders,
  socket: { remoteAddress?: string; encrypted?: boolean }
): IncomingMessage => ({ headers, socket }) as unknown as IncomingMessage;

describe('buildApiRequestHeaders', () => {
  it('forwards only the browser headers the api reads', () => {
    const headers = buildApiRequestHeaders(
      fakeRequest(
        {
          host: 'tickets.example.com',
          'content-type': 'application/json',
          accept: 'application/json',
          cookie: 'two_tickets_session=abc',
          origin: 'https://tickets.example.com',
          referer: 'https://tickets.example.com/login',
          authorization: 'Bearer nothing',
          'x-real-ip': '1.2.3.4',
          connection: 'keep-alive',
        },
        { remoteAddress: '192.0.2.7' }
      )
    );
    expect(headers).toEqual({
      'content-type': 'application/json',
      accept: 'application/json',
      cookie: 'two_tickets_session=abc',
      origin: 'https://tickets.example.com',
      referer: 'https://tickets.example.com/login',
      'x-forwarded-for': '192.0.2.7',
      'x-forwarded-host': 'tickets.example.com',
      'x-forwarded-proto': 'http',
    });
  });

  it('replaces inbound forwarding headers instead of appending to them', () => {
    const headers = buildApiRequestHeaders(
      fakeRequest(
        {
          host: 'tickets.example.com',
          'x-forwarded-for': '1.2.3.4',
          'x-forwarded-host': 'evil.example',
          'x-forwarded-proto': 'https',
          'x-forwarded-port': '443',
          forwarded: 'for=1.2.3.4;proto=https',
        },
        { remoteAddress: '192.0.2.7' }
      )
    );
    expect(headers['x-forwarded-for']).toBe('192.0.2.7');
    expect(headers['x-forwarded-host']).toBe('tickets.example.com');
    expect(headers['x-forwarded-proto']).toBe('http');
    expect(headers['x-forwarded-port']).toBeUndefined();
    expect(headers.forwarded).toBeUndefined();
  });

  it('gives each client its own address, so the api limits them apart', () => {
    const addressOf = (remoteAddress: string) =>
      buildApiRequestHeaders(fakeRequest({}, { remoteAddress }))[
        'x-forwarded-for'
      ];
    expect(addressOf('192.0.2.1')).toBe('192.0.2.1');
    expect(addressOf('192.0.2.2')).toBe('192.0.2.2');
    expect(addressOf('::ffff:192.0.2.3')).toBe('192.0.2.3');
    expect(addressOf('2001:db8::1')).toBe('2001:db8::1');
  });

  it('says https only when the browser connection is TLS', () => {
    const protoOf = (encrypted: boolean) =>
      buildApiRequestHeaders(
        fakeRequest({}, { remoteAddress: '192.0.2.1', encrypted })
      )['x-forwarded-proto'];
    expect(protoOf(true)).toBe('https');
    expect(protoOf(false)).toBe('http');
  });
});

describe('rawPathAndQuery', () => {
  const withMeta = (url: string, initURL?: string): IncomingMessage =>
    ({
      url,
      [Symbol.for('NextInternalRequestMeta')]:
        initURL === undefined ? undefined : { initURL },
    }) as unknown as IncomingMessage;

  it('prefers the URL Next received over the req.url it rebuilt', () => {
    expect(
      rawPathAndQuery(
        withMeta(
          '/api/v1/search?query=a+b',
          'http://0.0.0.0:3000/api/v1/search?query=a%20b'
        )
      )
    ).toBe('/api/v1/search?query=a%20b');
    expect(
      rawPathAndQuery(
        withMeta('/api/v1/search?query=a+b', '/api/v1/search?query=a%20b')
      )
    ).toBe('/api/v1/search?query=a%20b');
  });

  it('falls back to req.url outside Next', () => {
    expect(rawPathAndQuery(withMeta('/api/v1/x?q=%20'))).toBe(
      '/api/v1/x?q=%20'
    );
  });
});

describe('proxyToApi against a fake api', () => {
  it('returns every Set-Cookie line separately', async () => {
    const answer = await throughProxy({ path: '/api/v1/two-cookies' });
    expect(answer.status).toBe(200);
    expect(answer.headers['set-cookie']).toEqual([
      'first=1; Path=/',
      'second=2; Path=/',
    ]);
  });

  it('streams a request body to the api intact', async () => {
    const chunks = Array.from({ length: 64 }, (_, i) =>
      Buffer.alloc(32 * 1024, i)
    );
    const whole = Buffer.concat(chunks);
    const answer = await throughProxy(
      {
        method: 'POST',
        path: '/api/v1/upload',
        headers: { 'content-type': 'application/octet-stream' },
      },
      chunks
    );
    expect(answer.status).toBe(200);
    expect(received).toHaveLength(1);
    expect(received[0].bodyBytes).toBe(whole.length);
    expect(received[0].bodySha256).toBe(
      createHash('sha256').update(whole).digest('hex')
    );
  });

  it('passes an encoded query string through byte for byte', async () => {
    const path =
      '/api/v1/search?query=%C3%A9%20a%2Bb%26c%3Dd&page=1&x=%2F%3F%23+y&empty=';
    await throughProxy({ path });
    expect(received.map((r) => r.url)).toEqual([path]);
  });

  it('replaces an inbound X-Forwarded-For with the peer address', async () => {
    await throughProxy({
      path: '/api/v1/settings/public',
      headers: { 'x-forwarded-for': '1.2.3.4', forwarded: 'for=1.2.3.4' },
    });
    expect(received[0].headers['x-forwarded-for']).toBe('127.0.0.1');
    expect(received[0].headers.forwarded).toBeUndefined();
  });

  it('forwards Origin and Referer unchanged, for the api to judge', async () => {
    await throughProxy({
      method: 'POST',
      path: '/api/v1/auth/logout',
      headers: {
        origin: 'https://elsewhere.example',
        referer: 'https://elsewhere.example/page',
      },
    });
    expect(received[0].headers.origin).toBe('https://elsewhere.example');
    expect(received[0].headers.referer).toBe('https://elsewhere.example/page');
  });

  it('brings back a Secure cookie over HTTPS and a plain one over HTTP', async () => {
    const overTls = await throughProxy(
      { method: 'POST', path: '/api/v1/auth/local', tls: true },
      [Buffer.from('{}')]
    );
    const overPlain = await throughProxy(
      { method: 'POST', path: '/api/v1/auth/local' },
      [Buffer.from('{}')]
    );
    expect(received.map((r) => r.headers['x-forwarded-proto'])).toEqual([
      'https',
      'http',
    ]);
    expect(overTls.headers['set-cookie']?.[0]).toMatch(/; Secure$/);
    expect(overPlain.headers['set-cookie']?.[0]).not.toMatch(/Secure/);
  });

  it('returns the status and body, and no header outside the list', async () => {
    const answer = await throughProxy({ path: '/api/v1/auth/me' });
    expect(answer.status).toBe(401);
    expect(JSON.parse(answer.body)).toEqual({ bytes: 0 });
    expect(answer.headers['content-type']).toBe(
      'application/json; charset=utf-8'
    );
    expect(answer.headers['x-not-forwarded']).toBeUndefined();
  });

  it('answers 502 when no api address is configured', async () => {
    const saved = process.env.API_INTERNAL_URL;
    delete process.env.API_INTERNAL_URL;
    try {
      const answer = await throughProxy({ path: '/api/v1/auth/me' });
      expect(answer.status).toBe(502);
      expect(received).toHaveLength(0);
    } finally {
      process.env.API_INTERNAL_URL = saved;
    }
  });
});
