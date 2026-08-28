import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Hoisted mocks referenced by the vi.mock factories below.
const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: { sessionId: 'sess', user: null },
    getters: { isLoggedIn: vi.fn(() => true) },
    mutations: { setCurrentUser: vi.fn(), setSession: vi.fn() },
  },
}));

vi.mock('@/store', () => storeMock);
vi.mock('@/utils/constants', () => ({ globalVars: { baseURL: '/', recaptcha: false } }));
vi.mock('@/utils/url.js', () => ({
  getApiPath: (p, params = {}) =>
    `/api/${p}${params.id ? `?id=${params.id}` : ''}`,
}));

import { validateLogin } from './auth.js';

function respond(status, body = '') {
  global.fetch = vi.fn().mockResolvedValue({
    status,
    text: async () => body,
    json: async () => ({}),
  });
}

describe('validateLogin session-expiry handling', () => {
  beforeEach(() => {
    storeMock.mutations.setCurrentUser.mockClear();
    storeMock.getters.isLoggedIn.mockReturnValue(true);
    // Stub navigation via Vitest's global-stub API (jsdom doesn't implement
    // real navigation). pathname+search feed the redirect; href captures it.
    vi.stubGlobal('location', { pathname: '/files/', search: '?a=1', href: '' });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('clears user state and redirects (with encoded return path) on a non-public 401 while logged in', async () => {
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

  it('does NOT log out or redirect on a non-public 401 when the user is NOT logged in', async () => {
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
