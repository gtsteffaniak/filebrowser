import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Hoisted mocks referenced by the vi.mock factories below.
const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: { sessionId: 'sess', user: null },
    getters: {
      isLoggedIn: vi.fn(() => true),
      isShare: vi.fn(() => false),
    },
    mutations: { setCurrentUser: vi.fn(), setSession: vi.fn(), syncEnforcedUserDefaults: vi.fn(), syncSidebarLinkDefaultsPolicy: vi.fn() },
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
  renew,
  SESSION_REFRESH_BEFORE_MS,
  shouldRefreshBeforeExpiry,
  startSessionKeepAlive,
  stopSessionKeepAlive,
  validateLogin,
  VIEW_REFRESH_BEFORE_MS,
} from './auth.js';

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
    storeMock.getters.isLoggedIn.mockReturnValue(true);
    stopSessionKeepAlive();
    vi.stubGlobal('location', { pathname: '/files/', search: '?a=1', href: '' });
  });

  afterEach(() => {
    stopSessionKeepAlive();
    vi.unstubAllGlobals();
  });

  it('clears the user and redirects (with encoded return path) on a non-public 401 while logged in', async () => {
    respond(401, 'token is expired');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).toHaveBeenCalledWith(null);
    expect(window.location.href).toBe(
      `/login?redirect=${encodeURIComponent('/files/?a=1')}`
    );
  });

  it('does NOT log out or redirect on a PUBLIC 401 (anonymous share visitor)', async () => {
    respond(401, 'unauthorized');
    await expect(validateLogin(true)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(window.location.href).toBe('');
  });

  it('does NOT log out or redirect on a non-public 401 when the app is not logged in', async () => {
    storeMock.getters.isLoggedIn.mockReturnValue(false);
    respond(401, 'unauthorized');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(window.location.href).toBe('');
  });

  it('does NOT log out or redirect on a non-401 error (e.g. 500)', async () => {
    respond(500, 'server error');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
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
  });

  it('tracks exp from the renew response JWT', async () => {
    const exp = Math.floor(Date.now() / 1000) + 3600;
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => makeJwt(exp),
    });
    await renew();
    expect(getSessionJwtExpiresAt()).toBe(exp);
  });

  it('ensureSessionFresh renews when JWT is within the refresh window', async () => {
    const exp = Math.floor(Date.now() / 1000) + 60; // 1 minute left
    global.fetch = vi.fn()
      .mockResolvedValueOnce({ status: 200, text: async () => makeJwt(exp) })
      .mockResolvedValueOnce({ status: 200, text: async () => makeJwt(exp + 3600) });
    await renew();
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
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => makeJwt(exp),
    });
    await renew();
    global.fetch = vi.fn();
    const renewed = await ensureSessionFresh(SESSION_REFRESH_BEFORE_MS);
    expect(renewed).toBe(false);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it('ensureSessionFresh returns false for joined callers when renew fails', async () => {
    const exp = Math.floor(Date.now() / 1000) + 60;
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => makeJwt(exp),
    });
    await renew();
    let release;
    const barrier = new Promise((resolve) => {
      release = resolve;
    });
    global.fetch = vi.fn().mockImplementation(async () => {
      await barrier;
      return { status: 500, text: async () => 'renew failed' };
    });

    const first = ensureSessionFresh(SESSION_REFRESH_BEFORE_MS);
    const second = ensureSessionFresh(SESSION_REFRESH_BEFORE_MS);
    release();
    expect(await first).toBe(false);
    expect(await second).toBe(false);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('renew single-flights concurrent callers', async () => {
    let release;
    const barrier = new Promise((resolve) => {
      release = resolve;
    });
    global.fetch = vi.fn().mockImplementation(async () => {
      await barrier;
      return { status: 200, text: async () => 'ok' };
    });

    const first = renew();
    const second = renew();
    release();
    await Promise.all([first, second]);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('startSessionKeepAlive ticks ensureSessionFresh on an interval', async () => {
    const exp = Math.floor(Date.now() / 1000) + 60;
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: async () => makeJwt(exp),
    });
    startSessionKeepAlive();
    expect(global.fetch).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(global.fetch.mock.calls.length).toBeGreaterThanOrEqual(2);
    stopSessionKeepAlive();
  });
});
