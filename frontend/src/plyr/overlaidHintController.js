const FADE_MS = 400;
const REVEAL_AFTER_PLAY_MS = 1000;

const CLASS_SHOWN = 'fb-overlaid--shown';
const CLASS_FADE_IN = 'fb-overlaid--fade-in';
const CLASS_FADE_OUT = 'fb-overlaid--fade-out';

/**
 * Overlaid play/pause hint while video is playing.
 * Visibility follows Plyr control show/hide; icon swaps on play/pause only.
 * The overlaid button stays non-interactive — gestures handle taps/clicks.
 */
export function enableOverlaidHintController({
  player,
  hasStartedPlayback,
  baseUrl,
  isPlaying = false,
}) {
  if (!player?.elements?.container) {
    return {
      cleanup: () => {},
      syncPlayback: () => {},
      reset: () => {},
    };
  }

  let currentlyPlaying = false;
  let fadeTimer = null;
  let fadeInRaf = null;
  let suppressUntil = 0;

  const getPlyrEl = () => player.elements.container;

  const getOverlaidButton = () => {
    const buttons = player.elements?.buttons?.play;
    if (Array.isArray(buttons)) {
      const overlaid = buttons.find((btn) => btn.classList?.contains('plyr__control--overlaid'));
      if (overlaid) {
        return overlaid;
      }
    }
    return player.elements.container.querySelector('.plyr__control--overlaid');
  };

  const applyIcon = () => {
    const btn = getOverlaidButton();
    const use = btn?.querySelector('use');
    if (!use) return;
    const isPlaying = currentlyPlaying ? 'pause' : 'play';
    const href = `${baseUrl}public/static/img/plyr.svg#plyr-${isPlaying}`;
    use.setAttribute('href', href);
    use.setAttribute('xlink:href', href);
  };

  const clearTimers = () => {
    if (fadeTimer) {
      clearTimeout(fadeTimer);
      fadeTimer = null;
    }
    if (fadeInRaf !== null) {
      cancelAnimationFrame(fadeInRaf);
      fadeInRaf = null;
    }
  };

  const isSuppressed = () => suppressUntil > Date.now();
  const fadeOut = () => {
    const plyrEl = getPlyrEl();
    if (!plyrEl) {
      return;
    }
    clearTimers();
    plyrEl.classList.remove(CLASS_FADE_IN, CLASS_FADE_OUT);
    plyrEl.classList.add(CLASS_SHOWN);
    fadeInRaf = requestAnimationFrame(() => {
      fadeInRaf = null;
      plyrEl.classList.add(CLASS_FADE_OUT);
    });
    fadeTimer = setTimeout(() => {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_OUT, CLASS_FADE_IN);
      fadeTimer = null;
    }, FADE_MS);
  };

  // Instant hide on pause, plyr own paused will take place
  const hideInstantly = () => {
    clearTimers();
    const plyrEl = getPlyrEl();
    if (plyrEl) {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_IN, CLASS_FADE_OUT);
    }
  };

  const show = () => {
    if (!currentlyPlaying || !hasStartedPlayback() || isSuppressed()) {
      return;
    }
    const plyrEl = getPlyrEl();
    if (!plyrEl) {
      return;
    }
    clearTimers();
    plyrEl.classList.remove(CLASS_FADE_OUT);
    plyrEl.classList.add(CLASS_SHOWN, CLASS_FADE_IN);
    fadeTimer = setTimeout(() => {
      plyrEl.classList.remove(CLASS_FADE_IN);
      fadeTimer = null;
    }, FADE_MS);
  };

  const onControlsShown = () => {
    suppressUntil = 0;
    show();
  };

  const onControlsHidden = () => {
    fadeOut();
  };

  player.on('controlsshown', onControlsShown);
  player.on('controlshidden', onControlsHidden);

  const syncPlayback = (playing) => {
    currentlyPlaying = !!playing;
    applyIcon();
    if (currentlyPlaying) {
      suppressUntil = Date.now() + REVEAL_AFTER_PLAY_MS;
      fadeOut();
    } else {
      suppressUntil = 0;
      hideInstantly();
    }
  };

  const reset = () => syncPlayback(false);

  const cleanup = () => {
    player.off('controlsshown', onControlsShown);
    player.off('controlshidden', onControlsHidden);
    reset();
  };

  syncPlayback(isPlaying);

  return {
    cleanup,
    syncPlayback,
    reset,
  };
}
