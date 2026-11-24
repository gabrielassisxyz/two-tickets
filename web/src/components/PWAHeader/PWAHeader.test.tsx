import PWAHeader from '@app/components/PWAHeader';
import { createElement } from 'react';
import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

const render = (props?: { applicationTitle?: string }): string =>
  renderToString(createElement(PWAHeader, props));

describe('PWAHeader', () => {
  it('reads Two Tickets and carries no installable-PWA pieces', () => {
    const html = render();
    expect(html).toContain('Two Tickets');
    expect(html).not.toContain('site.webmanifest');
    expect(html).not.toContain('sw.js');
    expect(html).not.toContain('apple-touch-startup-image');
    expect(html).not.toContain('mobile-web-app-capable');
    expect(html).not.toContain('apple-mobile-web-app');
    expect(html).not.toContain('offline.html');
  });

  it('keeps the browser metadata: icons, description and theme', () => {
    const html = render({ applicationTitle: 'Two Tickets' });
    expect(html).toContain('/favicon-32x32.png');
    expect(html).toContain('/apple-touch-icon.png');
    expect(html).toContain('name="description"');
    expect(html).toContain('name="theme-color"');
    expect(html).toContain('name="application-name"');
  });
});
