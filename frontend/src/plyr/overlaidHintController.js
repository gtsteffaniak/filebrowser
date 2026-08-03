const DWELL_MS = 320;
const IDLE_MS = 2000;
const FADE_IN_MS = 200;
const FADE_OUT_MS = 450;
const REVEAL_AFTER_PLAY_MS = 1000;

const CLASS_SHOWN = 'fb-overlaid--shown';
const CLASS_FADE_IN = 'fb-overlaid--fade-in';
const CLASS_FADE_OUT = 'fb-overlaid--fade-out';

/**
 * Desktop overlaid play/pause hint while video is playing.
 * Icon swaps on play/pause only; visibility uses fade in/out.
 */
export function enableOverlaidHintController({
  player,
  surface,
  isHoverCapable,
  hasStartedPlayback,
  baseUrl,
}) {
  if (!player?.elements?.container || !surface) {
    return {
      cleanup: () => {},
      onPlaybackToggle: () => {},
      reset: () => {},
      setInitialIcon: () => {},
    };
  }

  let state = 'hidden';
  let pointerInside = false;
  let lastIconState = null;
  let dwellTimer = null;
  let idleTimer = null;
  let fadeTimer = null;
  let suppressUntilMove = false;
  let suppressStartTime = 0;

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

  const clearDwellTimer = () => {
    if (dwellTimer) {
      clearTimeout(dwellTimer);
      dwellTimer = null;
    }
  };

  const clearIdleTimer = () => {
    if (idleTimer) {
      clearTimeout(idleTimer);
      idleTimer = null;
    }
  };

  const clearFadeTimer = () => {
    if (fadeTimer) {
      clearTimeout(fadeTimer);
      fadeTimer = null;
    }
  };

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
    btn.setAttribute('aria-label', which === 'pause' ? 'Pause' : 'Play');
  };

  const resetIdleTimer = () => {
    clearIdleTimer();
    if (state !== 'shown' || !player.playing) {
      return;
    }
    idleTimer = setTimeout(() => {
      idleTimer = null;
      if (pointerInside && player.playing) {
        fadeOut();
      }
    }, IDLE_MS);
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
    clearIdleTimer();
    clearDwellTimer();
    clearFadeTimer();
    plyrEl.classList.remove(CLASS_FADE_IN);
    plyrEl.classList.add(CLASS_SHOWN);
    plyrEl.classList.remove(CLASS_FADE_OUT);
    requestAnimationFrame(() => {
      plyrEl.classList.add(CLASS_FADE_OUT);
    });
    fadeTimer = setTimeout(() => {
      plyrEl.classList.remove(CLASS_SHOWN, CLASS_FADE_OUT, CLASS_FADE_IN);
      state = 'hidden';
      fadeTimer = null;
    }, FADE_OUT_MS);
  };

  const show = () => {
    if (!player.playing || suppressUntilMove) {
      return;
    }
    const plyrEl = getPlyrEl();
    if (!plyrEl) {
      return;
    }
    clearFadeTimer();
    clearDwellTimer();
    state = 'shown';
    plyrEl.classList.remove(CLASS_FADE_OUT);
    plyrEl.classList.add(CLASS_SHOWN, CLASS_FADE_IN);
    fadeTimer = setTimeout(() => {
      plyrEl.classList.remove(CLASS_FADE_IN);
      fadeTimer = null;
    }, FADE_IN_MS);
    resetIdleTimer();
  };

  const scheduleShow = () => {
    if (!hasStartedPlayback() || !player.playing || suppressUntilMove) {
      return;
    }
    if (state === 'shown' || state === 'fadeOut') {
      if (state === 'shown') {
        resetIdleTimer();
      }
      return;
    }
    if (state === 'dwelling') {
      return;
    }
    state = 'dwelling';
    clearDwellTimer();
    dwellTimer = setTimeout(() => {
      dwellTimer = null;
      if (pointerInside && hasStartedPlayback() && player.playing && !suppressUntilMove) {
        show();
      } else {
        state = 'hidden';
      }
    }, DWELL_MS);
  };

  const onPointerEnter = (event) => {
    if (!isHoverCapable() || event.pointerType === 'touch') {
      return;
    }
    pointerInside = true;
    maybeClearPlaySuppress();
    scheduleShow();
  };

  const onPointerMove = (event) => {
    if (!isHoverCapable() || event.pointerType === 'touch') {
      return;
    }
    pointerInside = true;
    maybeClearPlaySuppress();
    scheduleShow();
  };

  const onPointerLeave = (event) => {
    if (!isHoverCapable() || event.pointerType === 'touch') {
      return;
    }
    pointerInside = false;
    clearDwellTimer();
    if (state === 'dwelling') {
      state = 'hidden';
    }
    if (state === 'shown') {
      fadeOut();
    }
  };

  const maybeClearPlaySuppress = () => {
    if (!suppressUntilMove) {
      return;
    }
    if (Date.now() - suppressStartTime >= REVEAL_AFTER_PLAY_MS) {
      suppressUntilMove = false;
    }
  };

  surface.addEventListener('pointerenter', onPointerEnter);
  surface.addEventListener('pointermove', onPointerMove);
  surface.addEventListener('pointerleave', onPointerLeave);

  const onPlaybackToggle = (isPlaying) => {
    if (isPlaying) {
      setIcon('pause');
      suppressUntilMove = true;
      suppressStartTime = Date.now();
      fadeOut({ force: true });
      return;
    }
    setIcon('play');
    suppressUntilMove = false;
  };

  const reset = () => {
    pointerInside = false;
    suppressUntilMove = false;
    suppressStartTime = 0;
    clearDwellTimer();
    clearIdleTimer();
    clearFadeTimer();
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
    surface.removeEventListener('pointerenter', onPointerEnter);
    surface.removeEventListener('pointermove', onPointerMove);
    surface.removeEventListener('pointerleave', onPointerLeave);
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
