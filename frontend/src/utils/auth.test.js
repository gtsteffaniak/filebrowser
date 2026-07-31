import { beforeEach, describe, expect, it, vi } from 'vitest';

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

const COOKIE = 'filebrowser_quantum_jwt';

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
    document.cookie = `${COOKIE}=stale; path=/`;
    // jsdom: make window.location assignable so sessionExpired() can "redirect".
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: { pathname: '/files/', search: '', href: '' },
    });
  });

  it('clears the cookie + user on a non-public 401 with a session cookie', async () => {
    respond(401, 'token is expired');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(false);
    expect(window.location.href).toContain('login');
  });

  it('does NOT log out on a PUBLIC 401 (anonymous share visitor)', async () => {
    respond(401, 'unauthorized');
    await expect(validateLogin(true)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(true);
  });

  it('does NOT log out on a non-public 401 when there is NO session cookie', async () => {
    document.cookie = `${COOKIE}=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/`;
    respond(401, 'unauthorized');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
  });

  it('does NOT log out on a non-401 error (e.g. 500)', async () => {
    respond(500, 'server error');
    await expect(validateLogin(false)).rejects.toThrow();
    expect(storeMock.mutations.setCurrentUser).not.toHaveBeenCalledWith(null);
    expect(document.cookie.includes(`${COOKIE}=stale`)).toBe(true);
  });
});
