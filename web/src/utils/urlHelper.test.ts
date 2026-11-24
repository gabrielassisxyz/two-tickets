import { getApiBaseUrl } from '@app/utils/urlHelper';
import { afterEach, describe, expect, it, vi } from 'vitest';

describe('getApiBaseUrl', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it('is API_INTERNAL_URL on the server, read when called', () => {
    vi.stubEnv('API_INTERNAL_URL', 'http://api.internal:8080');
    expect(getApiBaseUrl()).toBe('http://api.internal:8080');
    vi.stubEnv('API_INTERNAL_URL', 'http://other.internal:9090/');
    expect(getApiBaseUrl()).toBe('http://other.internal:9090');
  });

  it('never falls back to web’s own HOST and PORT', () => {
    vi.stubEnv('API_INTERNAL_URL', '');
    vi.stubEnv('HOST', 'tickets.example.com');
    vi.stubEnv('PORT', '3000');
    expect(() => getApiBaseUrl()).toThrow('API_INTERNAL_URL is not set');
  });

  it('is the page’s own origin in the browser', () => {
    vi.stubEnv('API_INTERNAL_URL', 'http://api.internal:8080');
    vi.stubGlobal('window', {});
    expect(getApiBaseUrl()).toBe('');
  });
});
