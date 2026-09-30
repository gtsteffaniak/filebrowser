import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { enableOverlaidHintController } from '@/plyr/overlaidHintController.js';

function createMockPlayer({ playing = true } = {}) {
  const container = document.createElement('div');
  container.className = 'plyr plyr--video plyr--playing plyr--hide-controls';

  const overlaid = document.createElement('button');
  overlaid.className = 'plyr__control plyr__control--overlaid';
  overlaid.innerHTML = '<svg><use href=""></use></svg>';
  container.appendChild(overlaid);

  const listeners = new Map();

  return {
    playing,
    elements: {
      container,
      buttons: { play: [overlaid] },
    },
    on(event, fn) {
      if (!listeners.has(event)) listeners.set(event, []);
      listeners.get(event).push(fn);
    },
    off(event, fn) {
      const list = listeners.get(event);
      if (!list) return;
      const idx = list.indexOf(fn);
      if (idx !== -1) list.splice(idx, 1);
    },
    emit(event) {
      (listeners.get(event) || []).forEach((fn) => fn());
    },
  };
}

describe('overlaidHintController', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows overlaid hint on controlsshown even when post-play suppress is active', () => {
    const player = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player,
      hasStartedPlayback: () => true,
      baseUrl: '/files/',
    });

    api.syncPlayback(true);
    vi.advanceTimersByTime(400);
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(false);

    player.emit('controlsshown');
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(true);

    api.cleanup();
  });

  it('hides overlaid hint on controlshidden while playing', () => {
    const player = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player,
      hasStartedPlayback: () => true,
      baseUrl: '/files/',
      isPlaying: player.playing,
    });

    player.emit('controlsshown');
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(true);

    player.emit('controlshidden');
    vi.advanceTimersByTime(400);
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(false);

    api.cleanup();
  });

  it('clears overlaid hint classes when playback pauses', () => {
    const player = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player,
      hasStartedPlayback: () => true,
      baseUrl: '/files/',
      isPlaying: player.playing,
    });

    player.emit('controlsshown');
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(true);

    api.syncPlayback(false);
    expect(player.elements.container.classList.contains('fb-overlaid--shown')).toBe(false);
    expect(player.elements.container.classList.contains('fb-overlaid--fade-in')).toBe(false);
    expect(player.elements.container.classList.contains('fb-overlaid--fade-out')).toBe(false);

    api.cleanup();
  });
});
