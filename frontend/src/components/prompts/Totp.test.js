import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp } from 'vue';

const { store, router, notify } = vi.hoisted(() => {
  const state = { route: { query: {} }, user: null, sessionId: 'test-session' };
  return {
    store: {
      state,
      getters: { isShare: () => false, isLoggedIn: () => state.user !== null },
      mutations: {
        showPrompt: vi.fn(),
        closeTopPrompt: vi.fn(),
        updateCurrentUser: vi.fn(),
        setCurrentUser: vi.fn((user) => { state.user = user; }),
      },
    },
    router: { push: vi.fn() },
    notify: { showError: vi.fn(), showSuccessToast: vi.fn() },
  };
});

vi.mock('@/store', () => store);
vi.mock('@/router', () => ({ default: router, router }));
vi.mock('@/notify', () => ({ notify }));
vi.mock('@/utils/constants', () => ({ globalVars: { baseURL: '/', recaptcha: false } }));
vi.mock('@/api/utils', () => ({ fetchJSON: vi.fn(), fetchURL: vi.fn() }));
vi.mock('@/api', async () => ({ authApi: await import('@/api/auth.js') }));
vi.mock('@/components/prompts/Prompts.vue', () => ({ default: {} }));
vi.mock('@/components/Tooltip.vue', () => ({ default: {} }));
vi.mock('@/components/LoadingSpinner.vue', () => ({ default: {} }));
vi.mock('qrcode.vue', () => ({ default: { render: () => null } }));

import Login from '@/views/Login.vue';
import Totp from './Totp.vue';

const password = 'test password &+%';
const user = { username: 'admin', loginMethod: 'password', otpEnabled: true };
let app;
let fetchMock;

function response(status, body = {}) {
  return { status, ok: status === 200, text: async () => JSON.stringify(body), json: async () => body };
}

async function submitLogin() {
  const form = { ...Login.data(), username: user.username, password, $t: (key) => key };
  await Login.methods.submit.call(form, { preventDefault() {}, stopPropagation() {} });
  return form;
}

function mountPrompt(props) {
  app = createApp(Totp, { username: user.username, password, ...props });
  app.config.globalProperties.$t = (key) => key;
  app.config.globalProperties.$router = router;
  app.directive('focus', {});
  return app.mount(document.createElement('div'));
}

function paths() {
  return fetchMock.mock.calls.map(([url]) => new URL(url, 'http://localhost').pathname);
}

describe('TOTP login and enrollment', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.state.user = null;
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    vi.spyOn(console, 'log').mockImplementation(() => {});
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    app?.unmount();
    app = undefined;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('retries password login with the existing OTP and initializes the authenticated user', async () => {
    fetchMock
      .mockResolvedValueOnce(response(403, { message: 'OTP code is required for user' }))
      .mockResolvedValueOnce(response(200))
      .mockResolvedValueOnce(response(200, user));

    await submitLogin();
    expect(store.mutations.showPrompt).toHaveBeenCalledOnce();
    const prompt = mountPrompt(store.mutations.showPrompt.mock.calls[0][0].props);
    prompt.code = '123456';
    await prompt.verifyCode();

    expect(paths()).toEqual(['/api/auth/login', '/api/auth/login', '/api/users']);
    for (const [index, [url, options]] of fetchMock.mock.calls.slice(0, 2).entries()) {
      expect(new URL(url, 'http://localhost').searchParams.get('username')).toBe(user.username);
      expect(options).toMatchObject({
        method: 'POST', credentials: 'same-origin',
        headers: { 'X-Password': encodeURIComponent(password), 'X-Secret': index === 0 ? '' : '123456' },
      });
    }
    expect(store.mutations.setCurrentUser).toHaveBeenCalledWith(user);
    expect(router.push).toHaveBeenCalledWith('/files/');
    expect(prompt.succeeded).toBe(true);
    expect(store.mutations.closeTopPrompt).toHaveBeenCalledOnce();
  });

  it('passes the captcha token rather than the redirect to the login endpoint', async () => {
    fetchMock.mockResolvedValueOnce(response(200)).mockResolvedValueOnce(response(200, user));
    const prompt = mountPrompt({ redirect: '/files/', recaptcha: 'captcha-token' });
    prompt.code = '123456';
    await prompt.verifyCode();

    const query = new URL(fetchMock.mock.calls[0][0], 'http://localhost').searchParams;
    expect(paths()[0]).toBe('/api/auth/login');
    expect(query.get('recaptcha')).toBe('captcha-token');
  });

  it('keeps an invalid OTP on the login prompt and allows a retry', async () => {
    fetchMock
      .mockResolvedValueOnce(response(403, { message: 'invalid OTP token' }))
      .mockResolvedValueOnce(response(200))
      .mockResolvedValueOnce(response(200, user));
    const prompt = mountPrompt({ redirect: '/files/' });
    prompt.code = '000000';
    await prompt.verifyCode();

    expect(prompt.error).toBe('otp.verificationFailed');
    expect(prompt.succeeded).toBe(false);
    expect(prompt.verifyInFlight).toBe(false);
    expect(store.state.user).toBeNull();
    expect(router.push).not.toHaveBeenCalled();
    expect(store.mutations.closeTopPrompt).not.toHaveBeenCalled();

    prompt.code = '123456';
    await prompt.verifyCode();
    expect(paths()).toEqual(['/api/auth/login', '/api/auth/login', '/api/users']);
    expect(prompt.succeeded).toBe(true);
  });

  it('rejects an empty OTP without sending a request', async () => {
    const prompt = mountPrompt({ redirect: '/files/' });
    await prompt.verifyCode();
    expect(prompt.error).toBe('otp.invalidCodeType');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('keeps invalid credentials in the password form without opening an OTP prompt', async () => {
    fetchMock.mockResolvedValueOnce(response(403, { message: 'invalid credentials' }));
    const form = await submitLogin();
    expect(form.error).toBe('invalid credentials');
    expect(store.mutations.showPrompt).not.toHaveBeenCalled();
    expect(store.state.user).toBeNull();
  });

  it('logs in without a prompt when the user has no TOTP', async () => {
    const noOtpUser = { ...user, otpEnabled: false };
    fetchMock.mockResolvedValueOnce(response(200)).mockResolvedValueOnce(response(200, noOtpUser));
    await submitLogin();
    expect(paths()).toEqual(['/api/auth/login', '/api/users']);
    expect(store.mutations.showPrompt).not.toHaveBeenCalled();
    expect(store.state.user).toEqual(noOtpUser);
    expect(router.push).toHaveBeenCalledWith({ path: '/files/' });
  });

  it('uses generate and verify before logging in after first-time enforced enrollment', async () => {
    fetchMock
      .mockResolvedValueOnce(response(403, { message: 'OTP is enforced, but user is not configured' }))
      .mockResolvedValueOnce(response(200, { url: 'otpauth://totp/test?secret=TEST' }))
      .mockResolvedValueOnce(response(200))
      .mockResolvedValueOnce(response(200))
      .mockResolvedValueOnce(response(200, user));

    await submitLogin();
    const prompt = mountPrompt(store.mutations.showPrompt.mock.calls[0][0].props);
    await vi.waitFor(() => expect(prompt.url).not.toBe(''));
    prompt.code = '123456';
    await prompt.verifyCode();

    expect(paths()).toEqual([
      '/api/auth/login', '/api/auth/otp/generate', '/api/auth/otp/verify', '/api/auth/login', '/api/users',
    ]);
    expect(prompt.succeeded).toBe(true);
    expect(store.state.user).toEqual(user);
  });

  it('keeps enrollment and reset from settings on the dedicated OTP endpoints', async () => {
    store.state.user = user;
    fetchMock
      .mockResolvedValueOnce(response(200, { url: 'otpauth://totp/test?secret=TEST' }))
      .mockResolvedValueOnce(response(200));
    const prompt = mountPrompt({ generate: true });
    await vi.waitFor(() => expect(prompt.url).not.toBe(''));
    prompt.code = '123456';
    await prompt.verifyCode();

    expect(paths()).toEqual(['/api/auth/otp/generate', '/api/auth/otp/verify']);
    expect(router.push).not.toHaveBeenCalled();
    expect(prompt.succeeded).toBe(true);
  });
});
