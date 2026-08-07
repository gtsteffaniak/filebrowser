/**
 * Show a loading indicator while the native video element is fetching or buffering.
 */
export function enablePlyrVideoLoadingIndicator(player, onLoadingChange) {
  if (!player?.media || typeof onLoadingChange !== 'function') {
    return () => {};
  }

  const media = player.media;
  let waiting = false;
  let seeking = false;
  let loading = false;
  /** True after play is requested; cleared when paused and not loading/buffering. */
  let playbackExpected = false;

  const isLoadingOrBuffering = () => (
    waiting
    || seeking
    || loading
    || (
      media.networkState === HTMLMediaElement.NETWORK_LOADING
      && media.readyState < HTMLMediaElement.HAVE_FUTURE_DATA
    )
  );

  const sync = () => {
    const hasSource = Boolean(media.currentSrc || media.src);
    const show = playbackExpected && hasSource && isLoadingOrBuffering();
    onLoadingChange(show);
  };

  const onPlay = () => {
    playbackExpected = true;
    sync();
  };

  const onPause = () => {
    const stillLoadingForPlayback = media.readyState < HTMLMediaElement.HAVE_FUTURE_DATA
      || isLoadingOrBuffering();
    if (!stillLoadingForPlayback) {
      playbackExpected = false;
    }
    sync();
  };

  const onWaiting = () => {
    waiting = true;
    sync();
  };
  const onPlaying = () => {
    waiting = false;
    loading = false;
    sync();
  };
  const onLoadStart = () => {
    loading = true;
    sync();
  };
  const onCanPlay = () => {
    loading = false;
    sync();
  };
  const onSeeking = () => {
    seeking = true;
    sync();
  };
  const onSeeked = () => {
    seeking = false;
    sync();
  };
  const onStalled = () => {
    waiting = true;
    sync();
  };
  const onError = () => {
    waiting = false;
    seeking = false;
    loading = false;
    playbackExpected = false;
    sync();
  };
  const onEmptied = () => {
    waiting = false;
    seeking = false;
    loading = false;
    playbackExpected = false;
    sync();
  };

  const handlers = {
    play: onPlay,
    pause: onPause,
    loadstart: onLoadStart,
    waiting: onWaiting,
    playing: onPlaying,
    canplay: onCanPlay,
    seeking: onSeeking,
    seeked: onSeeked,
    stalled: onStalled,
    error: onError,
    emptied: onEmptied,
  };

  Object.entries(handlers).forEach(([evt, fn]) => {
    player.on(evt, fn);
  });

  return () => {
    Object.entries(handlers).forEach(([evt, fn]) => {
      player.off(evt, fn);
    });
    onLoadingChange(false);
  };
}
