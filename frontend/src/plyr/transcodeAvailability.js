/** Whether this server can serve transcoded playback right now. */

import { state } from '@/store/index.ts';
import { globalVars } from '@/utils/constants.js';

/**
 * `globalVars` are static page constants, and `mediaAvailable` mirrors ffmpeg
 * availability, which cannot change without a restart — so it gates the feature
 * outright. The transcode toggle itself is read from the live media settings so
 * an admin change applies without a page reload.
 */
export function isTranscodeAvailable() {
  if (globalVars?.mediaAvailable !== true) {
    return false;
  }
  const live = state?.mediaPlayerSettings;
  if (live) {
    return live.enabled === true;
  }
  return globalVars.transcodeEnabled === true;
}
