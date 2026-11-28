// src/utils/playbackQueue.ts
import i18n from "@/i18n/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import type { FileListItem, PlaybackLoop, PlaybackMode } from "@/store/types.ts";
import { url } from "@/utils/index.ts";
import { sortedItems } from "@/utils/sort.ts";

const t = i18n.global.t.bind(i18n.global);

type PartialItem = Partial<FileListItem>;

function isQueueItem(item: PartialItem): item is FileListItem {
  return item.name !== undefined && item.path !== undefined;
}

/**
 * Shuffles an array -- this is used for playback queue shuffle mode
 * @param array - The array to shuffle.
 * @returns A new shuffled array.
 */
export function shuffleArray<T>(array: T[]): T[] {
  const shuffled = [...array];
  const result: T[] = [];
  while (shuffled.length > 0) {
    const randomIndex = Math.floor(Math.random() * shuffled.length);
    result.push(...shuffled.splice(randomIndex, 1));
  }
  return result;
}

/**
 * Builds the playback queue based on the current listing, current item, and playback mode.
 * @param listing - The full list of items in the current directory.
 * @param currentItem - The item currently being played.
 * @param mode - The current playback mode.
 * @param isShare - Whether if we're on a share (for the paths).
 * @returns The final queue and index of the current item.
 */
export function buildPlaybackQueue(
  listing: PartialItem[],
  currentItem: PartialItem,
  mode: PlaybackMode | false = false,
  isShare = false,
): { queue: FileListItem[]; currentIndex: number } {
  const searchItems = getters.searchNavigationItems(currentItem);
  if (searchItems) {
    const sorting = state.searchResults?.sorting;
    listing = sorting ? sortedItems([...searchItems], sorting.by, sorting.asc) : searchItems;
  }

  // Filter only audio/video files
  const mediaFiles = listing.filter((item): item is FileListItem => {
    const type = item.type || '';
    const isAudio = type.startsWith('audio/');
    const isVideo = type.startsWith('video/');
    return isQueueItem(item) && (isAudio || isVideo);
  });

  if (mediaFiles.length === 0) {
    return { queue: [], currentIndex: -1 };
  }

  const isCurrent = (item: PartialItem) => {
    if (isShare) {
      return item.name === currentItem.name;
    }
    const sameSource = !item.source || !currentItem.source || item.source === currentItem.source;
    return sameSource && item.path === currentItem.path;
  };

  const currentIndex = mediaFiles.findIndex(isCurrent);

  let finalQueue: FileListItem[];
  let finalIndex: number;

  switch (mode) {
    case 'single': {
      const currentFile = currentIndex !== -1 ? mediaFiles.at(currentIndex) : currentItem;
      const queued = currentFile && isQueueItem(currentFile) ? currentFile : undefined;
      finalQueue = queued ? [queued] : [];
      finalIndex = queued ? 0 : -1;
      break;
    }

    case 'sequential': {
      const sortedFiles = [...mediaFiles];
      finalQueue = sortedFiles;
      const currentFile = currentIndex !== -1 ? mediaFiles.at(currentIndex) : undefined;
      finalIndex = currentFile
        ? sortedFiles.indexOf(currentFile)
        : 0;
      break;
    }

    case 'shuffle': {
      // always reshuffle, but put current item at the top
      const currentFile = currentIndex !== -1 ? mediaFiles[currentIndex] : undefined;
      const otherItems = mediaFiles.filter(item => item !== currentFile);
      const shuffledOthers = shuffleArray(otherItems);
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
 * @param queue - The playback queue.
 * @param currentIndex - The current index in the queue.
 * @param loop - Loop state: "off" | "all" | "single".
 * @param direction - 1 for next, -1 for previous.
 * @returns The next item and its index, or null if none.
 */
export function getNextItem(
  queue: FileListItem[],
  currentIndex: number,
  loop: PlaybackLoop,
  direction: number,
): { index: number; item: FileListItem } | null {
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
 * @param currentMode - The current mode.
 * @param options - { listing, currentItem, isShare, targetMode? }
 * @returns The new mode.
 */
export function cyclePlaybackModes(
  currentMode: PlaybackMode,
  { listing, currentItem, isShare, targetMode }: {
    listing: PartialItem[];
    currentItem: PartialItem;
    isShare?: boolean;
    targetMode?: PlaybackMode;
  },
): PlaybackMode {
  const modes: PlaybackMode[] = ['sequential', 'shuffle'];
  const currentIndex = modes.indexOf(currentMode);
  const newMode = targetMode || modes.at((currentIndex + 1) % modes.length) || 'sequential';

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
 * @param currentLoop - Loop state: 'off' | 'all' | 'single'
 * @returns New loop state
 */
export function cycleLoopState(currentLoop: PlaybackLoop): PlaybackLoop {
  const states: PlaybackLoop[] = ['off', 'single', 'all'];
  const currentIndex = states.indexOf(currentLoop);
  return states.at((currentIndex + 1) % states.length) ?? 'off';
}

/**
 * Toggles single loop on/off, is only used by the "L" shortcut
 * @param currentLoop - Loop state: 'off' | 'all' | 'single'
 * @returns "off", "single"
 */
export function toggleSingleLoop(currentLoop: PlaybackLoop): PlaybackLoop {
  return currentLoop === 'single' ? 'off' : 'single';
}

/**
 * Gets label for the playback modes.
 * @param mode - The mode.
 * @param queueLength - Number of items in the queue. When provided, uses it for the playback mode label.
 * @returns The label.
 */
export function getModeLabel(mode: PlaybackMode, queueLength?: number): string {
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
 * @param mode - Playback mode
 * @returns Icon name
 */
export function getModeIcon(mode: PlaybackMode): string {
  switch (mode) {
    case 'sequential': return 'playlist_play';
    case 'shuffle':    return 'shuffle';
    default:           return 'music_note';
  }
}

/**
 * Gets the label for the loop states
 * @param loop - Loop state: 'off' | 'all' | 'single'
 * @returns The label
 */
export function getLoopLabel(loop: PlaybackLoop): string {
  switch (loop) {
    case 'all':    return t('player.PlayAllLoopedPlayback');
    case 'single': return t('player.LoopSingle');
    default:       return t('player.LoopDisabled');
  }
}

/**
 * Gets the icon for the loop states
 * @param loop - Loop state: 'off' | 'all' | 'single'
 * @returns Icon name
 */
export function getLoopIcon(loop: PlaybackLoop): string {
  switch (loop) {
    case 'single': return 'repeat_one';
    default:       return 'repeat';
  }
}

/**
 * Determines the action to take when the current media ends.
 * @param queue - The playback queue.
 * @param currentIndex - The current index.
 * @param loop - Loop state: 'off' | 'all' | 'single'
 * @returns The action to take.
 */
export function getEndOfMediaAction(
  queue: FileListItem[],
  currentIndex: number,
  loop: PlaybackLoop = state.playbackQueue.loop,
): 'next' | 'restart' | 'none' {
  if (loop === 'single') return 'restart';
  if (!queue.length || currentIndex < 0) return 'none';
  if (queue.length === 1) return loop === 'all' ? 'restart' : 'none';
  if (currentIndex + 1 < queue.length) return 'next';
  return loop === 'all' ? 'next' : 'none';
}

/**
 * Navigates the playback queue in a given direction (next or previous).
 * @param direction - -1 for previous, 1 for next.
 * @returns True if navigation was triggered, false otherwise.
 */
export function navigatePlaybackQueue(direction: number): boolean {
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
export function clearPlaybackQueue(currentItem: PartialItem = state.req): void {
  mutations.setPlaybackQueue({
    queue: isQueueItem(currentItem) ? [currentItem] : [],
    currentIndex: isQueueItem(currentItem) ? 0 : -1,
    mode: 'single',
    loop: state.playbackQueue.loop
  });
}

/**
 * @param artist - Artist from metadata
 * @returns Formatted artist
 */
export function formatArtist(artist: string | null | undefined): string {
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
