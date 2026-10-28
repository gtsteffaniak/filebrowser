import { describe, expect, it, vi, beforeEach } from 'vitest';

vi.mock('@/api/settings', () => ({
  patchMediaSettings: vi.fn(async (partial) => partial),
}));

import { patchMediaSettings } from '@/api/settings.js';
import {
  enabledTranscodeProfileIds,
  mergeMediaSettings,
  normalizeMediaPlayerSettings,
  saveMediaSettingsPatch,
  stripLockedMediaPaths,
} from '@/utils/mediaSettings.js';

describe('mediaSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('stripLockedMediaPaths', () => {
    it('returns the patch untouched when nothing is locked', () => {
      const patch = { hardwareAcceleration: true, transcode: { enabled: true } };
      expect(stripLockedMediaPaths(patch, [])).toEqual(patch);
    });

    it('removes a locked top-level field but keeps its siblings', () => {
      const stripped = stripLockedMediaPaths(
        { hardwareAcceleration: true, debug: false, transcode: { enabled: true } },
        ['hardwareAcceleration'],
      );
      expect(stripped).toEqual({ debug: false, transcode: { enabled: true } });
    });

    it('removes a locked nested path without dropping sibling branches', () => {
      const stripped = stripLockedMediaPaths(
        { transcode: { enabled: true, maxConcurrent: 3, profiles: { quality: { maxResolution: 0 } } } },
        ['transcode.enabled'],
      );
      expect(stripped).toEqual({
        transcode: { maxConcurrent: 3, profiles: { quality: { maxResolution: 0 } } },
      });
    });

    it('does not mutate the caller patch', () => {
      const patch = { hardwareAcceleration: true, debug: true };
      stripLockedMediaPaths(patch, ['hardwareAcceleration']);
      expect(patch).toEqual({ hardwareAcceleration: true, debug: true });
    });

    it('ignores locked paths that are absent from the patch', () => {
      const stripped = stripLockedMediaPaths({ debug: true }, ['transcode.profiles.quality.enabled']);
      expect(stripped).toEqual({ debug: true });
    });
  });

  describe('saveMediaSettingsPatch', () => {
    it('saves only the unlocked paths', async () => {
      const result = await saveMediaSettingsPatch(
        { hardwareAcceleration: true, debug: false },
        ['hardwareAcceleration'],
      );
      expect(result.skipped).toBe(false);
      expect(patchMediaSettings).toHaveBeenCalledWith({ debug: false });
    });

    it('skips the request entirely when every path is locked', async () => {
      const result = await saveMediaSettingsPatch(
        { transcode: { enabled: true } },
        ['transcode.enabled'],
      );
      expect(result.skipped).toBe(true);
      expect(patchMediaSettings).not.toHaveBeenCalled();
    });
  });

  describe('mergeMediaSettings', () => {
    it('defaults accelDevice and encoderBackend fields when absent', () => {
      const merged = mergeMediaSettings(undefined);
      expect(merged.accelDevice).toBe('auto');
      expect(merged.transcode.encoderBackend).toBe('');
      expect(merged.transcode.profiles.datasaver.encoderBackend).toBe('');
    });

    it('defaults the producer rate limit to 2x realtime when unset or zero', () => {
      expect(mergeMediaSettings(undefined).transcode.readRate).toBe(2);
      expect(mergeMediaSettings({ transcode: { readRate: 0 } }).transcode.readRate).toBe(2);
      expect(mergeMediaSettings({ transcode: { readRate: 1.5 } }).transcode.readRate).toBe(1.5);
      expect(mergeMediaSettings({ transcode: { readRate: 4 } }).transcode.readRate).toBe(4);
    });

    it('merges accelDevice and per-preset encoderBackend overrides', () => {
      const merged = mergeMediaSettings({
        accelDevice: 'renderD128',
        transcode: {
          encoderBackend: 'vaapi',
          profiles: { balanced: { encoderBackend: 'cpu' } },
        },
      });
      expect(merged.accelDevice).toBe('renderD128');
      expect(merged.transcode.encoderBackend).toBe('vaapi');
      expect(merged.transcode.profiles.balanced.encoderBackend).toBe('cpu');
      expect(merged.transcode.profiles.quality.encoderBackend).toBe('');
    });
  });

  describe('normalizeMediaPlayerSettings', () => {
    it('derives enabled profiles in menu order from the admin settings shape', () => {
      const media = {
        transcode: {
          enabled: true,
          maxConcurrentPerUser: 2,
          profiles: {
            quality: { enabled: true },
            balanced: { enabled: false },
            datasaver: { enabled: true },
          },
        },
        hardwareAcceleration: true,
      };
      expect(enabledTranscodeProfileIds(media)).toEqual(['quality', 'datasaver']);
      expect(normalizeMediaPlayerSettings(media)).toEqual({
        enabled: true,
        profiles: ['quality', 'datasaver'],
        maxConcurrentPerUser: 2,
        hardwareAcceleration: true,
      });
    });

    it('accepts the player-only shape returned to non-admin users', () => {
      expect(normalizeMediaPlayerSettings({
        enabled: false,
        profiles: ['quality'],
        maxConcurrentPerUser: 3,
        hardwareAcceleration: false,
      })).toEqual({
        enabled: false,
        profiles: ['quality'],
        maxConcurrentPerUser: 3,
        hardwareAcceleration: false,
      });
    });

    it('falls back to a per-user limit of one and disabled when values are absent', () => {
      expect(normalizeMediaPlayerSettings(undefined)).toEqual({
        enabled: false,
        profiles: [],
        maxConcurrentPerUser: 1,
        hardwareAcceleration: false,
      });
    });
  });
});
