import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Hoisted mocks referenced by the vi.mock factories below.
const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: { sessionId: 'sess', user: null },
    getters: {
      isLoggedIn: vi.fn(() => true),
      isShare: vi.fn(() => false),
    },
    mutations: { setCurrentUser: vi.fn(), setSession: vi.fn(), syncEnforcedUserDefaults: vi.fn() },
  },
}));

vi.mock('@/store', () => storeMock);
vi.mock('@/utils/constants', () => ({ globalVars: { baseURL: '/', recaptcha: false } }));
vi.mock('@/utils/url.js', () => ({
  getApiPath: (p, params = {}) =>
    `/api/${p}${params.username ? `?username=${params.username}` : ''}`,
}));

import {
  ensureSessionFresh,
  getSessionJwtExpiresAt,
  msUntilRefresh,
  SESSION_COOKIE_NAME,
  SESSION_REFRESH_BEFORE_MS,
  shouldRefreshBeforeExpiry,
  startSessionKeepAlive,
  stopSessionKeepAlive,
  validateLogin,
  VIEW_REFRESH_BEFORE_MS,
} from './auth.js';

const COOKIE = SESSION_COOKIE_NAME;

function respond(status, body = '') {
  global.fetch = vi.fn().mockResolvedValue({
    status,
    text: async () => body,
    json: async () => ({}),
  });
}

function makeJwt(expSeconds) {
  const header = btoa(JSON.stringify({ alg: 'none', typ: 'JWT' }))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  const payload = btoa(JSON.stringify({ exp: expSeconds }))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `${header}.${payload}.sig`;
}

describe('validateLogin session-expiry handling', () => {
  beforeEach(() => {
    storeMock.mutations.setCurrentUser.mockClear();
    stopSessionKeepAlive();
    document.cookie = `${COOKIE}=stale; path=/`;
    // Stub navigation via Vitest's global-stub API (jsdom doesn't implement
    // real navigation). pathname+search feed the redirect; href captures it.
    vi.stubGlobal('location', { pathname: '/files/', search: '?a=1', href: '' });
  });

  afterEach(() => {
    stopSessionKeepAlive();
    vi.unstubAllGlobals();
  });

  it('clears the cookie + user and redirects (with encoded return path) on a non-public 401 with a session cookie', async () => {
    respond(401, 'token is expired');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(false);
    // Full redirect contract: login route + correctly encoded current path+query.
    expect(window.location.href).toBe(
      `/login?redirect=${encodeURIComponent('/files/?a=1')}`
    );
  });

  it('does NOT log out or redirect on a PUBLIC 401 (anonymous share visitor)', async () => {
    respond(401, 'unauthorized');
    await expect(validateLogin(true)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(true);
    expect(window.location.href).toBe('');
  });

  it('does NOT log out or redirect on a non-public 401 when there is NO session cookie', async () => {
    document.cookie = `${COOKIE}=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/`;
    respond(401, 'unauthorized');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(window.location.href).toBe('');
  });

  it('does NOT log out or redirect on a non-401 error (e.g. 500)', async () => {
    respond(500, 'server error');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(true);
    expect(window.location.href).toBe('');
  });
});

describe('tokenRefresh helpers', () => {
  it('msUntilRefresh and shouldRefreshBeforeExpiry agree near expiry', () => {
    const soon = Math.floor(Date.now() / 1000) + 60;
    expect(shouldRefreshBeforeExpiry(soon, SESSION_REFRESH_BEFORE_MS)).toBe(true);
    expect(msUntilRefresh(soon, SESSION_REFRESH_BEFORE_MS)).toBe(0);

    const later = Math.floor(Date.now() / 1000) + 60 * 60;
    expect(shouldRefreshBeforeExpiry(later, VIEW_REFRESH_BEFORE_MS)).toBe(false);
    expect(msUntilRefresh(later, VIEW_REFRESH_BEFORE_MS)).toBeGreaterThan(0);
  });

  it('treats missing expiry as needing refresh', () => {
    expect(shouldRefreshBeforeExpiry(null, SESSION_REFRESH_BEFORE_MS)).toBe(true);
    expect(msUntilRefresh(null, SESSION_REFRESH_BEFORE_MS)).toBe(0);
  });
});

describe('session JWT keep-alive', () => {
  beforeEach(() => {
    stopSessionKeepAlive();
    storeMock.mutations.setSession.mockClear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    stopSessionKeepAlive();
    vi.useRealTimers();
    document.cookie = `${COOKIE}=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/`;
  });

  it('parses exp from the session cookie JWT', () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    document.cookie = `${COOKIE}=${makeJwt(exp)}; path=/`;
    expect(getSessionJwtExpiresAt()).toBe(exp);
  });

  it('ensureSessionFresh renews when JWT is within the refresh window', async () => {
    const exp = Math.floor(Date.now() / 1000) + 60; // 1 minute left
    document.cookie = `${COOKIE}=${makeJwt(exp)}; path=/`;
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => 'new-token',
    });
    const renewed = await ensureSessionFresh(SESSION_REFRESH_BEFORE_MS);
    expect(renewed).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/auth/renew',
      expect.objectContaining({ method: 'POST' })
    );
    expect(storeMock.mutations.setSession).toHaveBeenCalled();
  });

  it('ensureSessionFresh skips renew when JWT is still fresh', async () => {
    const exp = Math.floor(Date.now() / 1000) + 2 * 60 * 60; // 2 hours left
    document.cookie = `${COOKIE}=${makeJwt(exp)}; path=/`;
    global.fetch = vi.fn();
    const renewed = await ensureSessionFresh(SESSION_REFRESH_BEFORE_MS);
    expect(renewed).toBe(false);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it('startSessionKeepAlive ticks ensureSessionFresh on an interval', async () => {
    const exp = Math.floor(Date.now() / 1000) + 60;
    document.cookie = `${COOKIE}=${makeJwt(exp)}; path=/`;
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => 'new-token',
    });
    startSessionKeepAlive();
    expect(global.fetch).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(global.fetch.mock.calls.length).toBeGreaterThanOrEqual(2);
    stopSessionKeepAlive();
  });
});
