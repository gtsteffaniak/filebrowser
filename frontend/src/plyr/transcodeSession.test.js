import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { TranscodeSessionController } from '@/plyr/transcodeSession.js';

vi.mock('@/api/transcode.js', () => ({
  startTranscodeSession: vi.fn(async () => ({
    id: 'sess-1',
    heartbeatIntervalSec: 12,
    deliveryBaseUrl: '/api/media/transcode/sessions/sess-1',
  })),
  heartbeatTranscodeSession: vi.fn(async () => {}),
  stopTranscodeSession: vi.fn(async () => {}),
}));

import {
  heartbeatTranscodeSession,
  startTranscodeSession,
  stopTranscodeSession,
} from '@/api/transcode.js';

describe('TranscodeSessionController', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('starts session and sends heartbeat ticks', async () => {
    const ctrl = new TranscodeSessionController({
      source: 's',
      path: '/v.mp4',
      viewToken: 'vt',
      profile: 'quality',
      clientId: 'c1',
    });
    await ctrl.start({});
    expect(startTranscodeSession).toHaveBeenCalledOnce();
    ctrl.updatePlayback({ currentTime: 12.5, paused: false });
    vi.advanceTimersByTime(12000);
    expect(heartbeatTranscodeSession).toHaveBeenCalled();
    await ctrl.stop();
    expect(stopTranscodeSession).toHaveBeenCalledWith('sess-1', { keepalive: false });
  });

  it('stops with keepalive on the unload path', async () => {
    const ctrl = new TranscodeSessionController({
      source: 's',
      path: '/v.mp4',
      viewToken: 'vt',
      profile: 'quality',
      clientId: 'c1',
    });
    await ctrl.start({});
    await ctrl.stop({ keepalive: true });
    // keepalive lets the DELETE outlive the document; without it a tab close
    // cancels the request and the session is only reclaimed by the janitor.
    expect(stopTranscodeSession).toHaveBeenCalledWith('sess-1', { keepalive: true });
  });

  it('detach stops heartbeat without server stop', async () => {
    const ctrl = new TranscodeSessionController({
      source: 's',
      path: '/v.mp4',
      viewToken: 'vt',
      profile: 'quality',
    });
    await ctrl.start({});
    ctrl.detach();
    vi.advanceTimersByTime(30000);
    expect(heartbeatTranscodeSession).not.toHaveBeenCalled();
    expect(stopTranscodeSession).not.toHaveBeenCalled();
  });
});
