// A stand-in for the api process, for the production-image check in
// deploy/web-e2e.sh. It answers the boot calls from the committed contract
// bodies and prints one JSON line per request it receives, with the port it
// arrived on and the Host it named, so the script can tell which address
// server-side rendering actually called.
//
// It listens on the api port and on the ports a call to web's own or public
// name would reach, so a call to the wrong address is logged instead of
// failing silently.
import { readFileSync } from 'node:fs';
import http from 'node:http';

const contractDir = process.env.CONTRACT_DIR ?? '/e2e/contract';
const ports = (process.env.FAKE_API_PORTS ?? '8080,3000,80')
  .split(',')
  .map(Number);

const contract = (name) => readFileSync(`${contractDir}/${name}`, 'utf8');
const settings = contract('settings-public.json');
const user = contract('auth-me-first.json');

const answer = (req, res) => {
  const path = (req.url ?? '').split('?')[0];
  res.setHeader('content-type', 'application/json; charset=utf-8');
  if (path === '/api/v1/settings/public') {
    res.end(settings);
    return;
  }
  if (path === '/api/v1/auth/me') {
    if (!(req.headers.cookie ?? '').includes('two_tickets_session=')) {
      res.statusCode = 401;
      res.end('{"message":"Unauthorized","errors":[]}');
      return;
    }
    res.end(user);
    return;
  }
  if (path === '/api/v1/two-cookies') {
    res.setHeader('set-cookie', ['first=1; Path=/', 'second=2; Path=/']);
  }
  res.end('{}');
};

for (const port of ports) {
  http
    .createServer((req, res) => {
      req.resume();
      req.on('end', () => {
        process.stdout.write(
          `${JSON.stringify({
            port,
            host: req.headers.host ?? '',
            method: req.method,
            url: req.url,
            cookie: req.headers.cookie ?? '',
            forwardedFor: req.headers['x-forwarded-for'] ?? '',
          })}\n`
        );
        answer(req, res);
      });
    })
    .listen(port, '0.0.0.0');
}
process.stdout.write(`${JSON.stringify({ listening: ports })}\n`);
