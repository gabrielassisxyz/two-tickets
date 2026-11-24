// Host policy for artwork. The pod's root filesystem is read-only, so Next's
// image optimizer cannot run, and artwork loads straight from `image.tmdb.org`.
// Every other remote host becomes the local placeholder, `javascript:` URLs
// included.

export const IMAGE_PLACEHOLDER = '/images/seerr_poster_not_found.png';

const TMDB_HOST = 'image.tmdb.org';

// A local asset is a path that resolves to this page's own origin. Testing
// only for a leading slash would let `//evil.example/x.jpg` and
// `/\evil.example/x.jpg` through: browsers read both as another host.
const LOCAL_BASE = 'http://local.invalid';

const isRelativePath = (src: string): boolean => {
  if (!src.startsWith('/')) {
    return false;
  }
  try {
    return new URL(src, LOCAL_BASE).origin === LOCAL_BASE;
  } catch {
    return false;
  }
};

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
