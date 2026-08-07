import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

const pushMock = vi.fn(() => Promise.resolve());

vi.mock('@/router', () => ({
  default: { push: (...args) => pushMock(...args) },
}));

vi.mock('@/store', () => ({
  state: {
    route: { path: '/files/source/vid.mp4' },
    req: null,
    sources: { current: 'source' },
  },
}));

vi.mock('@/utils/url.js', () => ({
  buildItemUrl: (source, path) => `/files/${source}${path}`,
}));

describe('pipSession', () => {
  beforeEach(async () => {
    vi.resetModules();
    pushMock.mockClear();
    document.body.innerHTML = '';
    HTMLVideoElement.prototype.pause = vi.fn();
    HTMLVideoElement.prototype.load = vi.fn();
    Object.defineProperty(document, 'pictureInPictureElement', {
      configurable: true,
      writable: true,
      value: null,
    });
    Object.defineProperty(document, 'exitPictureInPicture', {
      configurable: true,
      writable: true,
      value: vi.fn(() => Promise.resolve()),
    });
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'visible',
    });
    Object.defineProperty(document, 'hasFocus', {
      configurable: true,
      value: () => true,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  async function loadModule() {
    return import('@/plyr/pipSession.js');
  }

  function dispatchLeavePip(media) {
    const event = new Event('leavepictureinpicture', { bubbles: true });
    Object.defineProperty(event, 'target', { value: media });
    media.dispatchEvent(event);
  }

  function dispatchLeavePipOnDocument(media) {
    const event = new Event('leavepictureinpicture', { bubbles: true });
    Object.defineProperty(event, 'target', { value: media });
    document.dispatchEvent(event);
  }

  async function flushHandoffNavigation() {
    for (let i = 0; i < 4; i++) {
      await Promise.resolve();
      await new Promise((r) => requestAnimationFrame(r));
    }
    await new Promise((r) => setTimeout(r, 10));
  }

  it('registerSession adopts media into a hidden document host', async () => {
    const mod = await loadModule();
    const wrapper = document.createElement('div');
    const media = document.createElement('video');
    wrapper.appendChild(media);
    document.body.appendChild(wrapper);
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });
    expect(mod.hasActiveSession('src', '/a.mp4')).toBe(true);
    expect(mod.hasActiveSession('src', 'a.mp4')).toBe(true);
    expect(mod.hasActiveSession('src', '/b.mp4')).toBe(false);
    expect(document.body.contains(media)).toBe(true);
    expect(document.getElementById('pip-session-host')?.contains(media)).toBe(true);
    expect(wrapper.contains(media)).toBe(false);
  });

  it('takeSessionSnapshot prefers pictureInPictureElement currentTime', async () => {
    const mod = await loadModule();
    const media = document.createElement('video');
    media.setAttribute('src', 'blob:test');
    media.pause = vi.fn();
    media.load = vi.fn();
    Object.defineProperty(media, 'currentTime', { value: 0, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    const pipMedia = document.createElement('video');
    Object.defineProperty(pipMedia, 'currentTime', { value: 42, writable: true });
    Object.defineProperty(pipMedia, 'paused', { value: false, writable: true });
    document.pictureInPictureElement = pipMedia;
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const snap = await mod.takeSessionSnapshot('src', '/a.mp4');
    expect(snap).toEqual({
      source: 'src',
      path: '/a.mp4',
      currentTime: 42,
      wasPlaying: true,
      wasFullscreen: false,
    });
  });

  it('appendMediaFragment adds #t= seek hint to stream URL', async () => {
    const mod = await loadModule();
    expect(mod.appendMediaFragment('https://example.com/v.mp4', 90)).toBe(
      'https://example.com/v.mp4#t=90',
    );
    expect(mod.appendMediaFragment('https://example.com/v.mp4#t=1', 90)).toBe(
      'https://example.com/v.mp4#t=90',
    );
    expect(mod.appendMediaFragment('https://example.com/v.mp4', 0)).toBe(
      'https://example.com/v.mp4',
    );
  });

  it('isPipBlockedForPreview when another PiP or session is active', async () => {
    const mod = await loadModule();
    const mediaA = document.createElement('video');
    const mediaB = document.createElement('video');

    expect(mod.isPipBlockedForPreview('src', '/a.mp4', mediaB)).toBe(false);

    mod.registerSession(mediaA, { source: 'src', path: '/a.mp4' });
    expect(mod.isPipBlockedForPreview('src', '/b.mp4', mediaB)).toBe(true);
    expect(mod.isPipBlockedForPreview('src', '/a.mp4', mediaA)).toBe(false);

    document.pictureInPictureElement = mediaA;
    expect(mod.isPipBlockedForPreview('src', '/b.mp4', mediaB)).toBe(true);
    expect(mod.isPipBlockedForPreview('src', '/a.mp4', mediaA)).toBe(true);

    document.pictureInPictureElement = null;
    await mod.takeSessionSnapshot('src', '/a.mp4');
    expect(mod.isPipBlockedForPreview('src', '/b.mp4', mediaB)).toBe(false);
  });

  it('shouldDeferVideoStreamAttach when session or pending resume exists', async () => {
    const mod = await loadModule();
    expect(mod.shouldDeferVideoStreamAttach('src', '/a.mp4')).toBe(false);

    const media = document.createElement('video');
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });
    expect(mod.shouldDeferVideoStreamAttach('src', '/a.mp4')).toBe(true);
    expect(mod.shouldDeferVideoStreamAttach('src', '/other.mp4')).toBe(false);

    await mod.takeSessionSnapshot('src', '/a.mp4');
    mod.setPendingInlineResume({
      source: 'src',
      path: '/a.mp4',
      currentTime: 1,
      wasPlaying: true,
      wasFullscreen: true,
      timestamp: Date.now(),
    });
    expect(mod.shouldDeferVideoStreamAttach('src', '/a.mp4')).toBe(true);
    expect(mod.shouldDeferVideoStreamAttach('src', '/other.mp4')).toBe(false);
    mod.clearPendingInlineResume();
    expect(mod.shouldDeferVideoStreamAttach('src', '/a.mp4')).toBe(false);
  });

  it('leavepictureinpicture navigates when document has focus (back-to-tab heuristic)', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 10, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const { state } = await import('@/store');
    state.route.path = '/files/source/';

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 10,
      wasPlaying: false,
      wasFullscreen: false,
    });
    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
  });

  it('leavepictureinpicture while another preview is open navigates and resumes pip video', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 12, writable: true });
    Object.defineProperty(media, 'paused', { value: false, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const { state } = await import('@/store');
    state.route.path = '/files/src/b.mp4';
    state.req = { source: 'src', path: '/b.mp4', type: 'video/mp4' };

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 12,
      wasPlaying: true,
    });
    expect(mod.hasActiveSession('src', '/a.mp4')).toBe(false);
    expect(media.pause).toHaveBeenCalled();
  });

  it('leavepictureinpicture always navigates even without focus', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 5, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    Object.defineProperty(document, 'hasFocus', {
      configurable: true,
      value: () => false,
    });

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 5,
    });
    expect(mod.hasActiveSession('src', '/a.mp4')).toBe(false);
    expect(media.pause).toHaveBeenCalled();
  });

  it('registerSession media listener handles leave without initPipSession', async () => {
    const mod = await loadModule();

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 3, writable: true });
    Object.defineProperty(media, 'paused', { value: false, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const { state } = await import('@/store');
    state.route.path = '/files/source/';

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 3,
      wasPlaying: true,
    });
  });

  it('document leave listener fallback handles detached media events', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 8, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    dispatchLeavePipOnDocument(media);

    await flushHandoffNavigation();

    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 8,
    });
  });

  it('onPendingInlineResume notifies subscribers on same-route back-to-tab handoff', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const { state } = await import('@/store');
    state.route.path = '/files/src/a.mp4';

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 7, writable: true });
    Object.defineProperty(media, 'paused', { value: false, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const listener = vi.fn();
    mod.onPendingInlineResume(listener);

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(listener).toHaveBeenCalledWith(expect.objectContaining({
      source: 'src',
      path: '/a.mp4',
      currentTime: 7,
      wasPlaying: true,
    }));
  });

  it('cross-route back-to-tab keeps pending for target preview mount', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const { state } = await import('@/store');
    state.route.path = '/files/src/b.mp4';

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 9, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const listener = vi.fn();
    mod.onPendingInlineResume(listener);

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(pushMock).toHaveBeenCalledWith('/files/src/a.mp4');
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 9,
    });
    expect(listener).not.toHaveBeenCalled();
  });

  it('navigateToPipVideo skips push when already on target route but still notifies', async () => {
    const mod = await loadModule();
    mod.initPipSession();

    const { state } = await import('@/store');
    state.route.path = '/files/src/a.mp4';

    const media = document.createElement('video');
    Object.defineProperty(media, 'currentTime', { value: 15, writable: true });
    Object.defineProperty(media, 'paused', { value: true, writable: true });
    mod.registerSession(media, { source: 'src', path: '/a.mp4' });

    const listener = vi.fn();
    mod.onPendingInlineResume(listener);

    dispatchLeavePip(media);

    await flushHandoffNavigation();

    expect(pushMock).not.toHaveBeenCalled();
    expect(mod.getPendingInlineResume()).toMatchObject({
      source: 'src',
      path: '/a.mp4',
      currentTime: 15,
    });
    expect(listener).toHaveBeenCalledWith(expect.objectContaining({
      source: 'src',
      path: '/a.mp4',
      currentTime: 15,
    }));
  });
});
