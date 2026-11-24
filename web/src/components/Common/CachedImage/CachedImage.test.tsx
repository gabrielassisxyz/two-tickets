import { IMAGE_PLACEHOLDER } from '@app/compat/imageHosts';
import CachedImage from '@app/components/Common/CachedImage';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

vi.mock('next/image', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  default: ({ src, alt }: any) => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const React = require('react');
    return React.createElement('img', { src, alt });
  },
}));

vi.mock('@app/hooks/useSettings', () => ({
  default: () => ({
    currentSettings: { cacheImages: false },
  }),
}));

const renderSrc = (src: string): string =>
  renderToStaticMarkup(
    <CachedImage src={src} type="tmdb" alt="poster" width={100} height={100} />
  );

describe('CachedImage hostile URLs', () => {
  it('renders the placeholder for a hostile URL', () => {
    const html = renderSrc('https://evil.example/x.jpg');
    expect(html).toContain(IMAGE_PLACEHOLDER);
    expect(html).not.toContain('evil.example');
  });

  it('renders the placeholder for a javascript: path', () => {
    const html = renderSrc('javascript:alert(1)');
    expect(html).toContain(IMAGE_PLACEHOLDER);
    expect(html).not.toContain('javascript:');
  });

  it('renders image.tmdb.org URLs unchanged', () => {
    const src = 'https://image.tmdb.org/t/p/w500/abc.jpg';
    expect(renderSrc(src)).toContain(src);
  });
});
