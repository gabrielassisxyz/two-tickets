/**
 * @type {import('next').NextConfig}
 */
const nextConfig = {
  output: 'standalone',
  env: {
    commitTag: process.env.COMMIT_TAG || 'local',
    unsafeDoNotUseDemo: process.env.UNSAFE_DO_NOT_USE_DEMO,
  },
  images: {
    unoptimized: true,
    remotePatterns: [{ hostname: 'image.tmdb.org' }],
  },
  transpilePackages: ['country-flag-icons'],
  turbopack: {
    rules: {
      '*.svg': {
        loaders: ['@svgr/webpack'],
        as: '*.js',
      },
    },
  },
  experimental: {
    scrollRestoration: true,
    largePageDataBytes: 512 * 1000,
  },
};

export default nextConfig;
