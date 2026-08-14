const TYPE_AHEAD_RESET_MS = 1000;
const LOG_PREFIX = '[listingTypeAhead]';

function log(...args) {
  console.log(LOG_PREFIX, ...args);
}

const session = {
  prefix: '',
  lastKey: '',
};

let resetTimeoutId = null;

export function normalizeTypeAheadName(name) {
  return String(name ?? '').normalize('NFC').toLocaleLowerCase();
}

export function normalizeTypeAheadKey(key) {
  if (typeof key !== 'string' || !/^[a-z0-9]$/i.test(key)) {
    return '';
  }
  return key.toLocaleLowerCase();
}

export function getTypeAheadPrefix() {
  return session.prefix;
}

export function isTypeAheadSessionActive() {
  return session.prefix !== '';
}

export function resetTypeAheadSession(reason = 'manual') {
  log('session reset', { reason, previousPrefix: session.prefix, previousLastKey: session.lastKey });
  if (resetTimeoutId !== null) {
    clearTimeout(resetTimeoutId);
    resetTimeoutId = null;
  }
  session.prefix = '';
  session.lastKey = '';
}

export function scheduleTypeAheadReset(onExpire = resetTypeAheadSession) {
  if (resetTimeoutId !== null) {
    clearTimeout(resetTimeoutId);
  }
  resetTimeoutId = setTimeout(() => {
    resetTimeoutId = null;
    onExpire('timer');
  }, TYPE_AHEAD_RESET_MS);
  log('timer scheduled', { resetMs: TYPE_AHEAD_RESET_MS, prefix: session.prefix });
}

/**
 * @param {string} key
 * @param {Array<{ name: string, index: number }>} items
 * @param {number | null} selectedIndex
 * @returns {{ prefix: string, matches: Array<{ name: string, index: number }>, nextPos: number }}
 */
export function processTypeAheadKey(key, items, selectedIndex = null) {
  const lowerKey = normalizeTypeAheadKey(key);
  if (!lowerKey) {
    log('ignored key', { key, reason: 'not alphanumeric single char' });
    return { prefix: session.prefix, matches: [], nextPos: 0, isSameKeyRepeat: false };
  }

  const previousPrefix = session.prefix;
  const previousLastKey = session.lastKey;
  const hasActiveSession = session.prefix !== '';
  const isSameKeyRepeat = lowerKey === session.lastKey && hasActiveSession;

  let prefix;
  if (isSameKeyRepeat) {
    prefix = session.prefix;
  } else if (!hasActiveSession) {
    prefix = lowerKey;
  } else {
    prefix = session.prefix + lowerKey;
  }

  session.prefix = prefix;
  session.lastKey = lowerKey;
  scheduleTypeAheadReset();

  const normalizedPrefix = normalizeTypeAheadName(prefix);
  const matches = items.filter((item) =>
    normalizeTypeAheadName(item.name).startsWith(normalizedPrefix)
  );

  let nextPos = 0;
  if (isSameKeyRepeat && selectedIndex !== null) {
    const curPos = matches.findIndex((m) => m.index === selectedIndex);
    if (curPos !== -1) {
      nextPos = (curPos + 1) % matches.length;
    }
  }

  log('key processed', {
    key,
    lowerKey,
    previousPrefix,
    previousLastKey,
    hasActiveSession,
    isSameKeyRepeat,
    newPrefix: prefix,
    normalizedPrefix,
    selectedIndex,
    matchCount: matches.length,
    matchNames: matches.slice(0, 8).map((m) => m.name),
    nextPos,
    willSelect: matches.length > 0 ? matches.at(nextPos)?.name : null,
  });

  return { prefix, matches, nextPos, isSameKeyRepeat };
}

export { TYPE_AHEAD_RESET_MS };
