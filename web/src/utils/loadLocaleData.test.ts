import { loadLocaleData } from '@app/utils/loadLocaleData';
import { describe, expect, it } from 'vitest';

describe('loadLocaleData', () => {
  it('drops empty pt-BR values so the English default renders', async () => {
    const messages = await loadLocaleData();
    expect(Object.keys(messages).length).toBeGreaterThan(0);
    for (const value of Object.values(messages)) {
      expect(value).not.toBe('');
    }
    // Spot-check a key the kept pages use: present and non-empty in pt-BR.
    expect(messages['components.Discover.discover']).toBeTruthy();
  });
});
