// An axios adapter that answers upstream's TMDB client from recorded fixtures.
// Nothing leaves the process: a request no fixture matches is an error that
// fails the case, never a fall-through to the network.
import axios, { AxiosError } from 'axios';

const TMDB_BASE_PATH = '/3';

// The request as it would go on the wire, minus the API key: the path under
// /3 and every query parameter axios serialises, as strings.
export const describeTmdbRequest = (config) => {
  const url = new URL(axios.getUri(config));
  if (!url.pathname.startsWith(`${TMDB_BASE_PATH}/`)) {
    throw new Error(`not a TMDB API request: ${url.origin}${url.pathname}`);
  }
  const params = {};
  for (const [key, value] of [...url.searchParams.entries()].sort()) {
    if (key !== 'api_key') {
      params[key] = value;
    }
  }
  return { path: url.pathname.slice(TMDB_BASE_PATH.length), params };
};

const sameRequest = (left, right) =>
  left.path === right.path &&
  JSON.stringify(Object.entries(left.params).sort()) ===
    JSON.stringify(Object.entries(right.params).sort());

const outcomeFor = (fixture, config) => {
  const { response } = fixture;
  if (response.error === 'timeout') {
    return new AxiosError('connect ETIMEDOUT', 'ETIMEDOUT', config);
  }
  const answer = {
    data: structuredClone(response.body),
    status: response.status,
    statusText: '',
    headers: {},
    config,
    request: null,
  };
  if (config.validateStatus(response.status)) {
    return answer;
  }
  return new AxiosError(
    `Request failed with status code ${response.status}`,
    response.status >= 500 ? 'ERR_BAD_RESPONSE' : 'ERR_BAD_REQUEST',
    config,
    null,
    answer
  );
};

// One session per case: the fixtures it may use, and a log of what was asked.
// `onUnmatched` is the recorder's hook: it may answer a request no fixture
// matches with a fixture it has just recorded.
export const createFixtureSession = (fixtures, { onUnmatched } = {}) => {
  const available = [...fixtures];
  const served = new Set();
  const unmatched = [];
  const adapter = async (config) => {
    const request = describeTmdbRequest(config);
    let fixture = available.find((candidate) =>
      sameRequest(candidate.request, request)
    );
    if (!fixture && onUnmatched) {
      fixture = await onUnmatched(request);
      if (fixture) {
        available.push(fixture);
      }
    }
    if (!fixture) {
      unmatched.push(request);
      throw new AxiosError('no fixture matches this TMDB request', 'ENOFIXTURE', config);
    }
    served.add(fixture.id);
    const outcome = outcomeFor(fixture, config);
    if (outcome instanceof Error) {
      throw outcome;
    }
    return outcome;
  };
  return { adapter, served, unmatched };
};
