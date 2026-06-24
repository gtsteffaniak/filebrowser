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
 * @param {boolean} forceReshuffle - Whether to force a reshuffle for shuffle mode.
 * @param {boolean} isShare - Whether if we're on a share (for the paths).
 * @returns {{ queue: Array, currentIndex: number }} The final queue and index of the current item.
 */
export function buildPlaybackQueue(listing, currentItem, mode, forceReshuffle = false, isShare = false) {
  // Filter only audio/video files
  const mediaFiles = listing.filter(item => {
    const isAudio = item?.type.startsWith('audio/');
    const isVideo = item?.type.startsWith('video/');
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
      if (forceReshuffle || state.playbackQueue.queue.length === 0) {
        const shuffledFiles = shuffleArray([...mediaFiles]);
        finalQueue = shuffledFiles;
      } else {
        // Preserve existing queue when not forcing reshuffle (basically never since we always reshuffle)
        finalQueue = state.playbackQueue.queue;
      }
      if (currentIndex !== -1) {
        const currentFile = mediaFiles.at(currentIndex);
        finalIndex = finalQueue.findIndex(item =>
          isShare ? item.name === currentFile.name : item.path === currentFile.path
        );
      } else {
        finalIndex = 0;
      }
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
 * Cycles through the available playback modes (excluding single/loop-single).
 * Order: loop-all -> shuffle -> sequential -> loop-all ...
 * @param {string} currentMode - The current mode.
 * @returns {string} The new mode.
 */
export function cyclePlaybackModes(currentMode) {
  const modes = ['loop-all', 'shuffle', 'sequential'];
  const currentIndex = modes.indexOf(currentMode);
  const nextIndex = (currentIndex + 1) % modes.length;
  return modes.at(nextIndex);
}

/**
 * Toggles between 'single' and 'loop-single'.
 * @param {string} currentMode - The current mode.
 * @returns {string} The new mode.
 */
export function toggleLoop(currentMode) {
  return currentMode === 'loop-single' ? 'single' : 'loop-single';
}

/**
 * Gets a human-readable label for a playback mode.
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
    case 'single':       return 'music_note';
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
  if (!queue.length || currentIndex < 0) return 'none';

  switch (mode) {
    case 'single':      return 'none';
    case 'loop-single': return 'restart';
    case 'sequential':  return currentIndex + 1 < queue.length ? 'next' : 'none';
    case 'shuffle':     return 'next';
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
    mode: mode
  });

  // Trigger navigation
  mutations.setNavigationTransitioning(true);
  url.goToItem(item.source || state.req.source, item.path, undefined, false, getters.isShare());

  return true;
}
