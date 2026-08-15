import { afterEach, describe, expect, it } from 'vitest';
import {
  getTypeAheadPrefix,
  isTypeAheadSessionActive,
  normalizeTypeAheadName,
  processTypeAheadKey,
  resetTypeAheadSession,
} from './listingTypeAhead.js';

const items = [
  { name: 'kitchen.txt', index: 0 },
  { name: 'notes.txt', index: 1 },
  { name: 'WORK.doc', index: 2 },
  { name: 'wombat', index: 3 },
];

afterEach(() => {
  resetTypeAheadSession();
});

describe('listingTypeAhead', () => {
  it('matches names case-insensitively', () => {
    expect(normalizeTypeAheadName('WORK.doc')).toBe('work.doc');
  });

  it('accumulates a full prefix across rapid keys', () => {
    processTypeAheadKey('w', items);
    processTypeAheadKey('o', items);
    processTypeAheadKey('r', items);
    const result = processTypeAheadKey('k', items);
    expect(getTypeAheadPrefix()).toBe('work');
    expect(result.matches.map((m) => m.name)).toEqual(['WORK.doc']);
  });

  it('keeps the buffer without changing selection when no items match', () => {
    processTypeAheadKey('w', items);
    processTypeAheadKey('o', items);
    processTypeAheadKey('r', items);
    const noMatch = processTypeAheadKey('x', items);
    expect(getTypeAheadPrefix()).toBe('worx');
    expect(noMatch.matches).toHaveLength(0);
    processTypeAheadKey('k', items);
    expect(getTypeAheadPrefix()).toBe('worxk');
  });

  it('tracks whether a type-ahead session is active', () => {
    expect(isTypeAheadSessionActive()).toBe(false);
    processTypeAheadKey('w', items);
    expect(isTypeAheadSessionActive()).toBe(true);
    resetTypeAheadSession();
    expect(isTypeAheadSessionActive()).toBe(false);
  });

  it('cycles single-letter matches on repeated key presses', () => {
    const first = processTypeAheadKey('w', items, null);
    expect(first.matches.map((m) => m.name)).toEqual(['WORK.doc', 'wombat']);
    expect(first.nextPos).toBe(0);

    const second = processTypeAheadKey('w', items, items[2].index);
    expect(second.prefix).toBe('w');
    expect(second.nextPos).toBe(1);
  });
});
