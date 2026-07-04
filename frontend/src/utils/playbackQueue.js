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
    case 'single':
      finalQueue = [];
      finalIndex = -1;
      break;

    case 'loop-single':
      finalQueue = currentIndex !== -1 ? [mediaFiles.at(currentIndex)] : [];
      finalIndex = 0;
      break;

    case 'sequential':
    case 'loop-all': {
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
 * Determines the next item in the queue based on direction and mode.
 * @param {Array} queue - The playback queue.
 * @param {number} currentIndex - The current index in the queue.
 * @param {string} mode - The playback mode.
 * @param {number} direction - 1 for next, -1 for previous.
 * @returns {{ index: number, item: Object } | null} The next item and its index, or null if none.
 */
export function getNextItem(queue, currentIndex, mode, direction) {
  if (!queue.length || currentIndex < 0) return null;

  let newIndex = currentIndex + direction;

  if (direction === -1) {
    if (newIndex < 0) {
      if (mode === 'loop-all' || mode === 'shuffle') {
        newIndex = queue.length - 1;
      } else {
        return null;
      }
    }
  } else if (newIndex >= queue.length) {
    if (mode === 'loop-all' || mode === 'shuffle') {
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
 * Order: sequential -> shuffle -> loop-all -> sequential ...
 * @param {string} currentMode - The current mode.
 * @param {Object} options - { listing, currentItem, isShare, targetMode? }
 * @returns {string} The new mode.
 */
export function cyclePlaybackModes(currentMode, { listing, currentItem, isShare, targetMode } = {}) {
  const modes = ['sequential', 'shuffle', 'loop-all'];
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
 * Toggles the loop flag (single - loop-single)
 * @param {boolean} currentLoop - current state
 * @returns {boolean} the new state
 */
export function toggleLoop(currentLoop) {
  return !currentLoop;
}

/**
 * Gets label for the playback modes.
 * @param {string} mode - The mode.
 * @param {Function} t - i18n translate function.
 * @returns {string} The label.
 */
export function getModeLabel(mode, t) {
  switch (mode) {
    case 'single':       return t('player.LoopDisabled');
    case 'sequential':   return t('player.PlayAllOncePlayback');
    case 'shuffle':      return t('player.ShuffleAllPlayback');
    case 'loop-single':  return t('player.LoopEnabled');
    case 'loop-all':     return t('player.PlayAllLoopedPlayback');
    default:             return t('player.LoopDisabled');
  }
}

/**
 * Gets the icon name for a playback mode - used in the toast when changing playback modes
 * @param {string} mode - Playback mode
 * @returns {string} Icon name
 */
export function getModeIcon(mode) {
  switch (mode) {
    case 'single':       return 'repeat';
    case 'sequential':   return 'playlist_play';
    case 'shuffle':      return 'shuffle';
    case 'loop-single':  return 'repeat_one';
    case 'loop-all':     return 'repeat';
    default:             return 'music_note';
  }
}

/**
 * Determines the action to take when the current media ends.
 * @param {Array} queue - The playback queue.
 * @param {number} currentIndex - The current index.
 * @param {string} mode - The playback mode.
 * @returns {'next' | 'restart' | 'none'} The action to take.
 */
export function getEndOfMediaAction(queue, currentIndex, mode) {
  if (state.playbackQueue.loop) return 'restart';
  if (!queue.length || currentIndex < 0) return 'none';

  switch (mode) {
    case 'single':      return 'none';
    case 'loop-single': return 'restart';
    case 'sequential':  return currentIndex + 1 < queue.length ? 'next' : 'none';
    case 'shuffle':     return currentIndex + 1 < queue.length ? 'next' : 'none';
    case 'loop-all':    return 'next';
    default:            return 'none';
  }
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

  if (!queue.length || currentIndex < 0) return false;

  const result = getNextItem(queue, currentIndex, mode, direction);
  if (!result) return false;

  const { index, item } = result;

  mutations.setPlaybackQueue({
    queue: queue,
    currentIndex: index,
    mode: mode,
    loop: state.playbackQueue.loop
  });

  // Trigger navigation
  mutations.setNavigationTransitioning(true);
  url.goToItem(item.source || state.req.source, item.path, undefined, false, getters.isShare());

  return true;
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
