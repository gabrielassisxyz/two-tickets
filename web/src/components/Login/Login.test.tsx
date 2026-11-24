import type { PublicSettingsResponse } from '@app/compat/interfaces/api/settingsInterfaces';
import { SettingsProvider } from '@app/context/SettingsContext';
import ptBR from '@app/i18n/locale/pt_BR.json';
import LoginPage from '@app/pages/login';
import axios from 'axios';
import type * as FormikModule from 'formik';
import type { FormikHelpers, FormikValues } from 'formik';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { NextRouter } from 'next/dist/shared/lib/router/router';
import { readFileSync } from 'node:fs';
import { createElement } from 'react';
import { renderToString } from 'react-dom/server';
import { IntlProvider } from 'react-intl';
import { SWRConfig } from 'swr';
import { afterEach, describe, expect, it, vi } from 'vitest';

// The login form's submit handler, captured from the Formik it renders.
const captured = vi.hoisted(() => ({
  onSubmit: undefined as
    | ((values: FormikValues, helpers: FormikHelpers<FormikValues>) => unknown)
    | undefined,
}));

vi.mock('formik', async (importOriginal) => {
  const actual = await importOriginal<typeof FormikModule>();
  const Formik = (props: Parameters<typeof actual.Formik>[0]) => {
    captured.onSubmit = props.onSubmit as typeof captured.onSubmit;
    return createElement(actual.Formik, props);
  };
  return { ...actual, Formik };
});

const contractFile = (name: string) =>
  readFileSync(
    new URL(`../../../../testdata/contract/${name}`, import.meta.url),
    'utf8'
  );

// The fixture api serves, shared with the Go tests.
const settingsFixture: PublicSettingsResponse = JSON.parse(
  contractFile('settings-public.json')
);

// Every field of the upstream type, copied from Seerr's
// server/interfaces/api/settingsInterfaces.ts.
const upstreamFields = contractFile('settings-public.fields')
  .split('\n')
  .map((line) => line.trim())
  .filter((line) => line !== '' && !line.startsWith('#'));

// The type-checker holds this object to exactly the keys of the compat copy
// of the type, so the list above, the copy and the fixture cannot drift
// apart without one of the checks below failing.
const compatTypeFields = {
  jellyfinHost: true,
  jellyfinExternalHost: true,
  jellyfinServerName: true,
  jellyfinForgotPasswordUrl: true,
  initialized: true,
  applicationTitle: true,
  applicationUrl: true,
  hideAvailable: true,
  hideBlocklisted: true,
  hideRequested: true,
  localLogin: true,
  mediaServerLogin: true,
  movie4kEnabled: true,
  series4kEnabled: true,
  discoverRegion: true,
  streamingRegion: true,
  originalLanguage: true,
  mediaServerType: true,
  partialRequestsEnabled: true,
  enableSpecialEpisodes: true,
  cacheImages: true,
  vapidPublic: true,
  enablePushRegistration: true,
  locale: true,
  emailEnabled: true,
  newPlexLogin: true,
  youtubeUrl: true,
  versionCheck: true,
  plexClientIdentifier: true,
} satisfies Record<keyof PublicSettingsResponse, true>;

const loginRouter = {
  pathname: '/login',
  route: '/login',
  asPath: '/login',
  query: {},
  basePath: '',
  isReady: true,
  isFallback: false,
  isPreview: false,
  isLocaleDomain: false,
  push: vi.fn(),
  replace: vi.fn(),
  reload: vi.fn(),
  back: vi.fn(),
  forward: vi.fn(),
  prefetch: vi.fn(),
  beforePopState: vi.fn(),
  events: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
} as unknown as NextRouter;

// The login page the way _app renders it on the server: the fixture as the
// settings, pt-BR messages, and no session.
const renderLoginPage = (settings: PublicSettingsResponse): string =>
  renderToString(
    createElement(
      RouterContext.Provider,
      { value: loginRouter },
      createElement(
        SWRConfig,
        { value: { provider: () => new Map(), fetcher: () => undefined } },
        createElement(
          SettingsProvider,
          { currentSettings: settings },
          createElement(
            IntlProvider,
            { locale: 'pt-BR', defaultLocale: 'en', messages: ptBR },
            createElement(LoginPage)
          )
        )
      )
    )
  );

const sorted = (keys: string[]) => [...keys].sort();

describe('the settings/public fixture', () => {
  it('sets exactly the fields of the upstream type', () => {
    expect(sorted(Object.keys(settingsFixture))).toEqual(
      sorted(upstreamFields)
    );
  });

  it('matches the compat copy of the type', () => {
    expect(sorted(Object.keys(compatTypeFields))).toEqual(
      sorted(upstreamFields)
    );
  });
});

describe('the login page against the fixture', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    captured.onSubmit = undefined;
  });

  it('renders the local form in Portuguese under the app title', () => {
    const html = renderLoginPage(settingsFixture);
    expect(html).toContain('Conecte-se com Two Tickets');
    expect(html).toMatch(/<input[^>]*name="email"[^>]*>/);
    expect(html).toMatch(/<input[^>]*name="password"[^>]*>/);
    expect(html).toContain('placeholder="Nome de usuário"');
  });

  it('shows no language picker and no media server sign-in', () => {
    const html = renderLoginPage(settingsFixture);
    expect(html).not.toContain('Language Picker');
    expect(html).not.toContain('id="language"');
    expect(html).not.toMatch(/plex|jellyfin|emby/i);
  });

  it('posts {email, password} to /api/v1/auth/local, the username in email', async () => {
    renderLoginPage(settingsFixture);
    expect(captured.onSubmit).toBeDefined();
    const post = vi.spyOn(axios, 'post').mockResolvedValue({ data: {} });
    await captured.onSubmit?.(
      { email: 'alice', password: 'sentinelsentinelsentinel' },
      {} as FormikHelpers<FormikValues>
    );
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith('/api/v1/auth/local', {
      email: 'alice',
      password: 'sentinelsentinelsentinel',
    });
  });
});
