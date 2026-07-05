// src/utils/playbackQueue.js
import { getters, mutations, state } from '@/store';
import { url } from '@/utils';

/**
 * Shuffles an array -- this is used for playback queue shuffle mode
 * @param {Array} array - The array to shuffle.
 * @returns {Array} A new shuffled array.
 */
export function shuffleArray(array) {
  const shuffled = [...array];
  const result = [];
  while (shuffled.length > 0) {
    const randomIndex = Math.floor(Math.random() * shuffled.length);
    result.push(shuffled.splice(randomIndex, 1)[0]);
  }
  return result;
}

/**
 * Builds the playback queue based on the current listing, current item, and playback mode.
 * @param {Array} listing - The full list of items in the current directory.
 * @param {Object} currentItem - The item currently being played.
 * @param {string} mode - The current playback mode.
 * @param {boolean} isShare - Whether if we're on a share (for the paths).
 * @returns {{ queue: Array, currentIndex: number }} The final queue and index of the current item.
 */
export function buildPlaybackQueue(listing, currentItem, mode = false, isShare = false) {
  // Filter only audio/video files
  const mediaFiles = listing.filter(item => {
    const type = item?.type || '';
    const isAudio = type.startsWith('audio/');
    const isVideo = type.startsWith('video/');
    return isAudio || isVideo;
  });

  if (mediaFiles.length === 0) {
    return { queue: [], currentIndex: -1 };
  }

  // Determine the value used to match the current item
  const matchValue = isShare ? currentItem.name : currentItem.path;

  // Find current index in mediaFiles
  const currentIndex = mediaFiles.findIndex(item =>
    isShare ? item.name === matchValue : item.path === matchValue
  );

  let finalQueue;
  let finalIndex;

  switch (mode) {
    case 'single': {
      const currentFile = currentIndex !== -1 ? mediaFiles.at(currentIndex) : currentItem;
      finalQueue = currentFile ? [currentFile] : [];
      finalIndex = currentFile ? 0 : -1;
      break;
    }

    case 'sequential': {
      const sortedFiles = [...mediaFiles];
      finalQueue = sortedFiles;
      if (currentIndex !== -1) {
        const currentFile = mediaFiles.at(currentIndex);
        finalIndex = sortedFiles.findIndex(item =>
          isShare ? item.name === currentFile.name : item.path === currentFile.path
        );
      } else {
        finalIndex = 0;
      }
      break;
    }

    case 'shuffle': {
      // always reshuffle, but put current item at the top
      const otherItems = mediaFiles.filter(item =>
        isShare ? item.name !== currentItem.name : item.path !== currentItem.path
      );
      const shuffledOthers = shuffleArray([...otherItems]);
      const currentFile = mediaFiles.find(item =>
        isShare ? item.name === currentItem.name : item.path === currentItem.path
      );
      finalQueue = currentFile ? [currentFile, ...shuffledOthers] : shuffledOthers;
      // at index 0
      finalIndex = currentFile ? 0 : -1;
      break;
    }

    default:
      finalQueue = [];
      finalIndex = -1;
  }

  return { queue: finalQueue, currentIndex: finalIndex };
}

/**
 * Determines the next item in the queue based on direction and loop state.
 * @param {Array} queue - The playback queue.
 * @param {number} currentIndex - The current index in the queue.
 * @param {string} loop - Loop state: "off" | "all" | "single".
 * @param {number} direction - 1 for next, -1 for previous.
 * @returns {{ index: number, item: Object } | null} The next item and its index, or null if none.
 */
export function getNextItem(queue, currentIndex, loop, direction) {
  if (!queue.length || currentIndex < 0) return null;

  let newIndex = currentIndex + direction;

  if (direction === -1) {
    if (newIndex < 0) {
      if (loop === 'all') {
        newIndex = queue.length - 1;
      } else {
        return null;
      }
    }
  } else if (newIndex >= queue.length) {
    if (loop === 'all') {
      newIndex = 0;
    } else {
      return null;
    }
  }
  const item = queue.at(newIndex);
  if (!item) return null;
  return { index: newIndex, item };
}

/**
 * Advances to the next playback mode, or jumps to targetMode if given.
 * Rebuilds the queue for the new mode and commits it to the store.
 * sequential -> shuffle -> sequential ...
 * @param {string} currentMode - The current mode.
 * @param {Object} options - { listing, currentItem, isShare, targetMode? }
 * @returns {string} The new mode.
 */
export function cyclePlaybackModes(currentMode, { listing, currentItem, isShare, targetMode } = {}) {
  const modes = ['sequential', 'shuffle'];
  const currentIndex = modes.indexOf(currentMode);
  const newMode = targetMode || modes.at((currentIndex + 1) % modes.length);

  const { queue, currentIndex: newQueueIndex } = buildPlaybackQueue(
    listing,
    currentItem,
    newMode,
    isShare
  );
  mutations.setPlaybackQueue({
    queue,
    currentIndex: newQueueIndex,
    mode: newMode,
    loop: state.playbackQueue.loop
  });

  return newMode;
}

/**
 * Advances to the next loop state.
 * off -> single -> all -> off ...
 * @param {string} currentLoop - Loop state: 'off' | 'all' | 'single'
 * @returns {string} New loop state
 */
export function cycleLoopState(currentLoop) {
  const states = ['off', 'single', 'all'];
  const currentIndex = states.indexOf(currentLoop);
  return states.at((currentIndex + 1) % states.length);
}

/**
 * Toggles single loop on/off, is only used by the "L" shortcut
 * @param {string} currentLoop - Loop state: 'off' | 'all' | 'single'
 * @returns {string} "off", "single"
 */
export function toggleSingleLoop(currentLoop) {
  return currentLoop === 'single' ? 'off' : 'single';
}

/**
 * Gets label for the playback modes.
 * @param {string} mode - The mode.
 * @param {Function} t - i18n translate function.
 * @param {number} [queueLength] - Number of items in the queue. When provided, uses it for the playback mode label.
 * @returns {string} The label.
 */
export function getModeLabel(mode, t, queueLength) {
  if (queueLength !== undefined && mode === 'single') {
    return t('general.none');
  }
  switch (mode) {
    case 'sequential':  return t('player.PlayAllOncePlayback');
    case 'shuffle':     return t('player.ShuffleAllPlayback');
    default:            return t('general.none');
  }
}

/**
 * Gets the icon name for a playback mode - used in the toast when changing playback modes
 * @param {string} mode - Playback mode
 * @returns {string} Icon name
 */
export function getModeIcon(mode) {
  switch (mode) {
    case 'sequential': return 'playlist_play';
    case 'shuffle':    return 'shuffle';
    default:           return 'music_note';
  }
}

/**
 * Gets the label for the loop states
 * @param {string} loop - Loop state: 'off' | 'all' | 'single'
 * @param {Function} t - i18n translate function
 * @returns {string} The label
 */
export function getLoopLabel(loop, t) {
  switch (loop) {
    case 'all':    return t('player.PlayAllLoopedPlayback');
    case 'single': return t('player.LoopSingle');
    default:       return t('player.LoopDisabled');
  }
}

/**
 * Gets the icon for the loop states
 * @param {string} loop - Loop state: 'off' | 'all' | 'single'
 * @returns {string} Icon name
 */
export function getLoopIcon(loop) {
  switch (loop) {
    case 'single': return 'repeat_one';
    default:       return 'repeat';
  }
}

/**
 * Determines the action to take when the current media ends.
 * @param {Array} queue - The playback queue.
 * @param {number} currentIndex - The current index.
 * @param {string} loop - Loop state: 'off' | 'all' | 'single'
 * @returns {'next' | 'restart' | 'none'} The action to take.
 */
export function getEndOfMediaAction(queue, currentIndex, loop = state.playbackQueue.loop) {
  if (loop === 'single') return 'restart';
  if (!queue.length || currentIndex < 0) return 'none';
  if (queue.length === 1) return loop === 'all' ? 'restart' : 'none';
  if (currentIndex + 1 < queue.length) return 'next';
  return loop === 'all' ? 'next' : 'none';
}

/**
 * Navigates the playback queue in a given direction (next or previous).
 * @param {number} direction - -1 for previous, 1 for next.
 * @returns {boolean} True if navigation was triggered, false otherwise.
 */
export function navigatePlaybackQueue(direction) {
  const queue = state.playbackQueue.queue;
  const currentIndex = state.playbackQueue.currentIndex;
  const mode = state.playbackQueue.mode;
  const loop = state.playbackQueue.loop;

  if (!queue.length || currentIndex < 0) return false;

  const result = getNextItem(queue, currentIndex, loop, direction);
  if (!result) return false;

  const { index, item } = result;

  mutations.setPlaybackQueue({
    queue: queue,
    currentIndex: index,
    mode: mode,
    loop: loop
  });

  // Trigger navigation
  mutations.setNavigationTransitioning(true);
  url.goToItem(item.source || state.req.source, item.path, undefined, false, getters.isShare());

  return true;
}

/** Clears the queue, keeping only the current item. */
export function clearPlaybackQueue(currentItem = state.req) {
  mutations.setPlaybackQueue({
    queue: currentItem ? [currentItem] : [],
    currentIndex: currentItem ? 0 : -1,
    mode: 'single',
    loop: state.playbackQueue.loop
  });
}

/**
 * @param {string} artist - Artist from metadata
 * @returns {string} Formatted artist
 */
export function formatArtist(artist) {
  if (typeof artist !== 'string' || !artist.trim()) {
    return '';
  }
  // by common separators like ',', ';', or ' feat. ', ' ft. '
  const parts = artist
    .split(/[,;]|\s+(?:feat|ft)\.?\s+/i)
    .map(s => s.trim())
    .filter(Boolean);
  return parts.join(' • ');
}
