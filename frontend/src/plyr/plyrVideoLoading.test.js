import { describe, expect, it } from 'vitest';
import { enablePlyrVideoLoadingIndicator } from '@/plyr/plyrVideoLoading.js';

function fakePlayer() {
  const handlers = {};
  const media = {
    currentSrc: 'video.mp4',
    readyState: HTMLMediaElement.HAVE_NOTHING,
    networkState: HTMLMediaElement.NETWORK_IDLE,
  };
  return {
    media,
    on: (evt, fn) => { handlers[evt] = fn; },
    off: (evt) => { delete handlers[evt]; },
    emit: (evt) => handlers[evt]?.(),
  };
}

function startPlayback() {
  const player = fakePlayer();
  const states = [];
  enablePlyrVideoLoadingIndicator(player, (show) => states.push(show));
  player.emit('play');
  player.media.readyState = HTMLMediaElement.HAVE_ENOUGH_DATA;
  player.media.networkState = HTMLMediaElement.NETWORK_LOADING;
  player.emit('playing');
  return { player, states };
}

describe('video loading indicator', () => {
  it('stays hidden when the fetch stalls but playback continues from the buffer', () => {
    const { player, states } = startPlayback();
    player.emit('stalled');
    expect(states.at(-1)).toBe(false);
  });

  it('shows while playback waits for data', () => {
    const { player, states } = startPlayback();
    player.media.readyState = HTMLMediaElement.HAVE_CURRENT_DATA;
    player.emit('waiting');
    expect(states.at(-1)).toBe(true);
  });
});
