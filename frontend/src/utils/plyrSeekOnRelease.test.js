import { describe, expect, it } from 'vitest';
import {
  blockPlyrSeekOnInput,
  commitPlyrSeek,
  readPlyrSeekPercent,
} from './plyrSeekOnRelease';

describe('readPlyrSeekPercent', () => {
  it('prefers seek-value attribute set by Plyr while scrubbing', () => {
    const input = document.createElement('input');
    input.type = 'range';
    input.max = '100';
    input.value = '10';
    input.setAttribute('seek-value', '42');

    expect(readPlyrSeekPercent(input)).toBe(42);
    expect(input.hasAttribute('seek-value')).toBe(false);
    expect(input.value).toBe('10');
  });

  it('falls back to range value', () => {
    const input = document.createElement('input');
    input.type = 'range';
    input.max = '100';
    input.value = '55';

    expect(readPlyrSeekPercent(input)).toBe(55);
  });
});

describe('commitPlyrSeek', () => {
  it('sets currentTime from committed scrub position', () => {
    const seek = document.createElement('input');
    seek.type = 'range';
    seek.max = '100';
    seek.value = '25';

    const player = {
      duration: 200,
      elements: { inputs: { seek } },
      currentTime: 0,
    };

    commitPlyrSeek(player, { currentTarget: seek });

    expect(player.currentTime).toBe(50);
  });
});

describe('blockPlyrSeekOnInput', () => {
  it('returns false so Plyr skips seek-on-input', () => {
    expect(blockPlyrSeekOnInput()).toBe(false);
  });
});
