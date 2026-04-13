import { state, mutations } from '@/store';
import { url } from '@/utils';

/**
 * Advance preview by one playback-queue step and sync the router.
 * Call only when `getters.isPreviewPlaybackQueueNavMode()` and the corresponding
 * `playbackQueueCanGo*` guard already passed (e.g. from NextPrevious or Preview).
 *
 * @param {{ replace: (loc: object) => Promise<unknown> }} router
 * @param {-1|1} delta
 * @returns {boolean} Whether navigation ran
 */
export function replaceRouteForPlaybackQueueStep(router, delta) {
  const item = mutations.navigatePlaybackQueueRelative(delta);
  if (!item) {
    return false;
  }
  const itemUrl = url.buildItemUrl(item.source || state.req.source, item.path);
  router.replace({ path: itemUrl }).catch((err) => {
    if (err.name !== 'NavigationDuplicated') {
      // Silently ignore navigation errors
    }
  });
  return true;
}
