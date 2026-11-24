// The bundle's entry: upstream's own API router mounted the way upstream's
// server/index.ts mounts it, with TMDB answered by fixtures. Requests are
// injected in process, so the harness opens no socket at all.
import './frozen-clock.mjs';
import routes from '@server/routes/index';
import axios from 'axios';
import express from 'express';
import * as OpenApiValidator from 'express-openapi-validator';
import inject from 'light-my-request';
import { createFixtureSession } from './fixture-adapter.mjs';
import { GOLDENS_USER } from '@server/datasource';

const refuseOutsideCase = async () => {
  throw new Error('goldens harness: a TMDB request arrived outside a case');
};
let activeAdapter = refuseOutsideCase;
axios.defaults.adapter = (config) => activeAdapter(config);

// The middleware below mirrors server/index.ts at the pinned commit, minus what
// a catalog response cannot observe (cookies, CSRF, client IP, demo mode, the
// Next.js handler). cookie-parser and express-session are replaced by the
// session cookie the OpenAPI security check looks for and a session already
// holding the one user, which is what upstream's checkUser reads.
const createApp = (apiSpecPath) => {
  const server = express();
  server.use(express.json());
  server.use(express.urlencoded({ extended: true }));
  server.use('/api', (req, _res, next) => {
    req.cookies = { 'connect.sid': 'goldens-session' };
    req.session = { userId: GOLDENS_USER.id };
    next();
  });
  server.use(
    OpenApiValidator.middleware({
      apiSpec: apiSpecPath,
      validateRequests: true,
    })
  );
  server.use((_req, res, next) => {
    const original = res.json;
    res.json = function jsonp(json) {
      return original.call(this, JSON.parse(JSON.stringify(json)));
    };
    next();
  });
  server.use('/api/v1', routes);
  server.use((err, _req, res, next) => {
    if (res.headersSent) {
      return next(err);
    }
    res.status(err.status || 500).json({
      message: err.message,
      errors: err.errors,
    });
  });
  return server;
};

const parseBody = (response) => {
  const type = response.headers['content-type'] ?? '';
  if (!type.startsWith('application/json')) {
    return { contentType: type, text: response.payload };
  }
  return JSON.parse(response.payload);
};

export const createRunner = (apiSpecPath) => {
  const app = createApp(apiSpecPath);
  return async (url, fixtures, options = {}) => {
    const session = createFixtureSession(fixtures, options);
    activeAdapter = session.adapter;
    try {
      const response = await inject(app, { method: 'GET', url });
      return {
        status: response.statusCode,
        body: parseBody(response),
        served: [...session.served],
        unmatched: session.unmatched,
      };
    } finally {
      activeAdapter = refuseOutsideCase;
    }
  };
};
