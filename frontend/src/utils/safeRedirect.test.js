import { describe, expect, it, vi } from 'vitest';
import {
  sanitizeLogoutDestination,
  postLoginRedirectForServer,
  sanitizePostLoginRedirect,
} from './safeRedirect.js';

describe('sanitizePostLoginRedirect', () => {
  it('returns default for missing or unsafe values', () => {
    expect(sanitizePostLoginRedirect(undefined)).toBe('/files/');
    expect(sanitizePostLoginRedirect('//evil.example/')).toBe('/files/');
    expect(sanitizePostLoginRedirect('/\\evil')).toBe('/files/');
    expect(sanitizePostLoginRedirect('https://evil.example/')).toBe('/files/');
  });

  it('allows safe relative paths', () => {
    expect(sanitizePostLoginRedirect('/files/?a=1')).toBe('/files/?a=1');
  });
});

describe('sanitizeLogoutDestination', () => {
  it('rejects cross-origin overrides', () => {
    const fallback = 'http://localhost/login';
    expect(
      sanitizeLogoutDestination('https://evil.example/', fallback)
    ).toBe(fallback);
  });

  it('allows same-origin absolute URLs', () => {
    vi.stubGlobal('location', { origin: 'http://localhost' });
    const dest = sanitizeLogoutDestination(
      'http://localhost/public/share/abc',
      'http://localhost/login'
    );
    expect(dest).toBe('http://localhost/public/share/abc');
    vi.unstubAllGlobals();
  });
});

describe('postLoginRedirectForServer', () => {
  it('prefixes router-relative paths with baseURL', () => {
    expect(postLoginRedirectForServer('/files/', '/app/')).toBe('/app/files/');
  });

  it('leaves paths that already include baseURL unchanged', () => {
    expect(postLoginRedirectForServer('/app/files/', '/app/')).toBe('/app/files/');
  });

  it('returns path unchanged when baseURL is root', () => {
    expect(postLoginRedirectForServer('/files/', '/')).toBe('/files/');
  });
});
