import { PLYR_CONTROLS_TRANSITION_MS } from '@/plyr/pipSession.js';

const FADE_MS = PLYR_CONTROLS_TRANSITION_MS;
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
}) {
  if (!player?.elements?.container) {
    return {
      cleanup: () => {},
      onPlaybackToggle: () => {},
      reset: () => {},
      setInitialIcon: () => {},
    };
  }

  let state = 'hidden';
  let lastIconState = null;
  let fadeTimer = null;
  let fadeOutRaf = null;
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

  const clearFadeTimer = () => {
    if (fadeTimer) {
      clearTimeout(fadeTimer);
      fadeTimer = null;
    }
  };

  const clearFadeOutRaf = () => {
    if (fadeOutRaf !== null) {
      cancelAnimationFrame(fadeOutRaf);
      fadeOutRaf = null;
    }
  };

  const isSuppressed = () => suppressUntil > Date.now();

  const setIcon = (which) => {
    if (lastIconState === which) {
      return;
    }
    lastIconState = which;
    const btn = getOverlaidButton();
    if (!btn) {
      return;
    }
    const href = `${baseUrl}public/static/img/plyr.svg#plyr-${which}`;
    const use = btn.querySelector('use');
    if (use) {
      use.setAttribute('href', href);
      use.setAttribute('xlink:href', href);
    }
  };

  const fadeOut = ({ force = false } = {}) => {
    if (!force && (state === 'hidden' || state === 'fadeOut')) {
      return;
    }
    const plyrEl = getPlyrEl();
    if (!plyrEl) {
      state = 'hidden';
      return;
    }
    state = 'fadeOut';
    clearFadeTimer();
    clearFadeOutRaf();
    plyrEl.classList.remove(CLASS_FADE_IN);
    plyrEl.classList.add(CLASS_SHOWN);
    plyrEl.classList.remove(CLASS_FADE_OUT);
    fadeOutRaf = requestAnimationFrame(() => {
      fadeOutRaf = null;
      plyrEl.classList.add(CLASS_FADE_OUT);
    });
    fadeTimer = setTimeout(() => {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_OUT, CLASS_FADE_IN);
      state = 'hidden';
      fadeTimer = null;
    }, FADE_MS);
  };

  const show = () => {
    if (!player.playing || !hasStartedPlayback() || isSuppressed()) {
      return;
    }
    const plyrEl = getPlyrEl();
    if (!plyrEl) {
      return;
    }
    clearFadeTimer();
    clearFadeOutRaf();
    state = 'shown';
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
    fadeOut({ force: true });
  };

  player.on('controlsshown', onControlsShown);
  player.on('controlshidden', onControlsHidden);

  const onPlaybackToggle = (isPlaying) => {
    if (isPlaying) {
      setIcon('pause');
      suppressUntil = Date.now() + REVEAL_AFTER_PLAY_MS;
      fadeOut({ force: true });
      return;
    }
    setIcon('play');
    suppressUntil = 0;
    clearFadeTimer();
    clearFadeOutRaf();
    state = 'hidden';
    const plyrEl = getPlyrEl();
    if (plyrEl) {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_IN, CLASS_FADE_OUT);
    }
  };

  const reset = () => {
    suppressUntil = 0;
    clearFadeTimer();
    clearFadeOutRaf();
    state = 'hidden';
    const plyrEl = getPlyrEl();
    if (plyrEl) {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_IN, CLASS_FADE_OUT);
    }
    lastIconState = null;
    setIcon('play');
  };

  const setInitialIcon = () => {
    setIcon('play');
  };

  const cleanup = () => {
    player.off('controlsshown', onControlsShown);
    player.off('controlshidden', onControlsHidden);
    reset();
  };

  setInitialIcon();

  return {
    cleanup,
    onPlaybackToggle,
    reset,
    setInitialIcon,
  };
}
