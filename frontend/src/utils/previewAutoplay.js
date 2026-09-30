/**
 * Whether preview should autoplay media.
 * playbackStarted enables autoplay for queue navigation only when autoplayMedia pref is off.
 */
export function shouldAutoPlayPreview(autoplayMedia, playbackStarted, isPlaybackQueueNavMode) {
  return autoplayMedia || (playbackStarted && isPlaybackQueueNavMode);
}
