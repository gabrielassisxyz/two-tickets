// Where server-side code reaches the api process. Seerr ran its API inside
// the Next process and called itself at HOST:PORT; here the api is a separate
// process, and HOST, HOSTNAME and PORT keep meaning web's own listen address.
// The address is read at run time, never at build time, so one production
// image works wherever it is deployed.
export function getApiBaseUrl(): string {
  // In the browser every call stays on the page's own origin and goes
  // through the /api/v1 proxy route.
  if (typeof window !== 'undefined') {
    return '';
  }
  const base = process.env.API_INTERNAL_URL;
  if (!base) {
    throw new Error('API_INTERNAL_URL is not set');
  }
  return base.replace(/\/+$/, '');
}
