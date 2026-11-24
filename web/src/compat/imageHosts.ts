// Host policy for artwork. The pod's root filesystem is read-only, so Next's
// image optimizer cannot run, and artwork loads straight from `image.tmdb.org`.
// Every other remote host becomes the local placeholder, `javascript:` URLs
// included.

export const IMAGE_PLACEHOLDER = '/images/seerr_poster_not_found.png';

const TMDB_HOST = 'image.tmdb.org';

const isRelativePath = (src: string): boolean => src.startsWith('/');

const isHttpUrl = (src: string): boolean =>
  src.startsWith('http://') || src.startsWith('https://');

export const isAllowedImageSrc = (src: string): boolean => {
  if (isRelativePath(src)) {
    return true;
  }

  if (!isHttpUrl(src)) {
    return false;
  }

  try {
    const host = new URL(src).hostname.toLowerCase();
    return host === TMDB_HOST;
  } catch {
    return false;
  }
};

export const resolveImageSrc = (src: string): string =>
  isAllowedImageSrc(src) ? src : IMAGE_PLACEHOLDER;
