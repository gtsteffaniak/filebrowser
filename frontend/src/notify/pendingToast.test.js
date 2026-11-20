import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/i18n/index.ts', () => ({ default: { global: { t: (key) => key } } }));
vi.mock('@/store/index.ts', () => ({ mutations: {}, state: {} }));
vi.mock('@/utils/notificationActions.js', () => ({ resolveHistoryNotificationButtons: vi.fn() }));

import { closeToast, getToasts, queueToastForNextPage, showPendingToast } from './message.js';

describe('pending toast across full page loads', () => {
  afterEach(() => {
    for (const toast of getToasts()) {
      closeToast(toast.id);
    }
    sessionStorage.clear();
  });

  it('shows a queued toast once on the next page', () => {
    queueToastForNextPage('success', 'Logged in');
    showPendingToast();
    expect(getToasts().map(({ type, message }) => ({ type, message }))).toEqual([
      { type: 'success', message: 'Logged in' },
    ]);
    expect(sessionStorage.getItem('pendingToast')).toBeNull();

    showPendingToast();
    expect(getToasts()).toHaveLength(1);
  });

  it('ignores malformed or unknown entries', () => {
    sessionStorage.setItem('pendingToast', '{not json');
    showPendingToast();
    sessionStorage.setItem('pendingToast', JSON.stringify({ type: 'bogus', message: 'x' }));
    showPendingToast();
    expect(getToasts()).toHaveLength(0);
    expect(sessionStorage.getItem('pendingToast')).toBeNull();
  });
});
