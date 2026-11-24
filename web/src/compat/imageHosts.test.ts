import {
  IMAGE_PLACEHOLDER,
  isAllowedImageSrc,
  resolveImageSrc,
} from '@app/compat/imageHosts';
import { describe, expect, it } from 'vitest';

describe('image host policy', () => {
  it('renders image.tmdb.org URLs unchanged', () => {
    const src = 'https://image.tmdb.org/t/p/w500/abc.jpg';
    expect(isAllowedImageSrc(src)).toBe(true);
    expect(resolveImageSrc(src)).toBe(src);
  });

  it('keeps relative local assets unchanged', () => {
    expect(resolveImageSrc('/logo_full.png')).toBe('/logo_full.png');
    expect(resolveImageSrc('/images/seerr_poster_not_found.png')).toBe(
      '/images/seerr_poster_not_found.png'
    );
  });

  it('renders a hostile URL as the placeholder', () => {
    expect(isAllowedImageSrc('https://evil.example/x.jpg')).toBe(false);
    expect(resolveImageSrc('https://evil.example/x.jpg')).toBe(
      IMAGE_PLACEHOLDER
    );
  });

  it('renders protocol-relative URLs to another host as the placeholder', () => {
    for (const src of [
      '//evil.example/x.jpg',
      '/\\evil.example/x.jpg',
      '//image.tmdb.org.evil.example/x.jpg',
    ]) {
      expect(isAllowedImageSrc(src)).toBe(false);
      expect(resolveImageSrc(src)).toBe(IMAGE_PLACEHOLDER);
    }
  });

  it('renders a javascript: path as the placeholder', () => {
    expect(isAllowedImageSrc('javascript:alert(1)')).toBe(false);
    expect(resolveImageSrc('javascript:alert(1)')).toBe(IMAGE_PLACEHOLDER);
  });

  it('renders other remote hosts as the placeholder', () => {
    for (const src of [
      'https://gravatar.com/avatar/abc',
      'https://artworks.thetvdb.com/banners/x.jpg',
      'https://plex.tv/pms/resources/x.png',
      'data:image/png;base64,abc',
    ]) {
      expect(resolveImageSrc(src)).toBe(IMAGE_PLACEHOLDER);
    }
  });
});
