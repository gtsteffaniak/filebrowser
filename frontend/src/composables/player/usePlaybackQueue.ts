import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import type Plyr from "plyr";
import { useI18n } from "@/i18n/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import type { FileListItem } from "@/store/types.ts";
import {
  buildPlaybackQueue,
  clearPlaybackQueue,
  cyclePlaybackModes as cycleMode,
  getLoopIcon,
  getLoopLabel,
  getModeIcon,
  getModeLabel,
  toggleSingleLoop as nextSingleLoop,
} from "@/utils/playbackQueue.js";
import { getPlayerI18n, getSettingsElements } from "./settingsElements.ts";

type ToastType = "mode" | "loop" | "cleared";
type LoopMode = "all" | "off" | "single";

/** The fields the queue identifies items by */
type QueueItem = Pick<Partial<FileListItem>, "name" | "path" | "type">;

interface PlaybackQueueOptions {
  /** The file being played. */
  getReq: () => QueueItem;
  /** The listing of the folder the file lives in. */
  getListing: () => FileListItem[] | null | undefined;
  onQueueChange: () => void;
  applyMenus: () => void;
}

/** Queue, loop and playback modes, the mode toast and the hand-built playback/loop settings menus. */
export function usePlaybackQueue({ getReq, getListing, onQueueChange, applyMenus }: PlaybackQueueOptions) {
  const { t } = useI18n();

  const queueCount = computed(() => state.playbackQueue.queue.length || 0);
  const playbackQueue = computed(() => state.playbackQueue.queue);
  const currentQueueIndex = computed(() => state.playbackQueue.currentIndex ?? -1);
  const playbackMode = computed(() => state.playbackQueue.mode);
  const loop = computed(() => state.playbackQueue.loop || "off");

  // --- Toast ---
  const toastVisible = ref(false);
  /** 'mode' for the playback modes, 'loop' for loop, 'cleared' when the queue is cleared. */
  const toastType = ref<ToastType>("mode");
  let toastTimeout: ReturnType<typeof setTimeout> | null = null;

  function showToast(): void {
    if (toastTimeout) {
      clearTimeout(toastTimeout);
    }
    toastVisible.value = true;
    toastTimeout = setTimeout(() => {
      toastVisible.value = false;
    }, 1500);
  }

  const playbackModeMessage = computed(() => {
    if (toastType.value === "loop") {
      return getLoopLabel(loop.value);
    }
    if (toastType.value === "cleared") {
      return t("player.queueCleared");
    }
    return getModeLabel(playbackMode.value);
  });

  const toastIcon = computed(() => {
    if (toastType.value === "loop") {
      return getLoopIcon(loop.value);
    }
    if (toastType.value === "cleared") {
      return "delete";
    }
    return getModeIcon(playbackMode.value);
  });

  onBeforeUnmount(() => {
    if (toastTimeout) {
      clearTimeout(toastTimeout);
    }
  });

  // --- Queue actions ---
  function showQueuePrompt(): void {
    mutations.showPrompt({ name: "PlaybackQueue" });
  }

  function toggleSingleLoop(): void {
    mutations.setPlaybackQueue({
      queue: playbackQueue.value,
      currentIndex: currentQueueIndex.value,
      mode: playbackMode.value,
      loop: nextSingleLoop(loop.value) as LoopMode,
    });
  }

  function cyclePlaybackModes(): void {
    cycleMode(playbackMode.value, {
      listing: getListing(),
      currentItem: getReq(),
      isShare: getters.isShare(),
    });
  }

  function setupPlaybackQueue(): void {
    const listing = getListing();
    const req = getReq();
    if (!listing || !req) return;
    const mode = state.playbackQueue.mode || "single";
    const { queue, currentIndex } = buildPlaybackQueue(listing, req, mode, getters.isShare());
    mutations.setPlaybackQueue({ queue, currentIndex, mode, loop: loop.value });
  }

  // Keep the queue in sync with the listing: refresh matching items in place, otherwise rebuild.
  watch(
    getListing,
    (newListing) => {
      if (!newListing?.length) return;
      const isShare = getters.isShare();
      const getId = (item: QueueItem): string | undefined => (isShare ? item.name : item.path);
      const mediaFiles = newListing.filter((item) => /^(audio|video)\//.test(item.type || ""));
      const queue = state.playbackQueue.queue as QueueItem[];
      const mode = state.playbackQueue.mode;

      if (queue.length && mediaFiles.length === queue.length) {
        const currIds = mediaFiles.map(getId);
        const queIds = queue.map(getId);
        let match: boolean;
        if (mode === "shuffle") {
          const set = new Set(queIds);
          match = currIds.every((id) => set.has(id));
        } else {
          match = currIds.every((id, i) => id === queIds.at(i));
        }
        if (match) {
          const byId = new Map(mediaFiles.map((item) => [getId(item), item]));
          const refreshed = queue.map((item) => byId.get(getId(item)) || item);
          const currentItemId = getId(getReq());
          const newIndex = refreshed.findIndex((item) => getId(item) === currentItemId);
          mutations.setPlaybackQueue({
            queue: refreshed,
            currentIndex: newIndex !== -1 ? newIndex : state.playbackQueue.currentIndex,
            mode,
            loop: loop.value,
          });
          return;
        }
      }
      setupPlaybackQueue();
    },
    { immediate: true },
  );

  watch(playbackMode, async (newMode, oldMode) => {
    if (newMode === oldMode) return;
    if (oldMode !== undefined) {
      toastType.value = newMode === "single" ? "cleared" : "mode";
      showToast();
    }
    onQueueChange();
    await nextTick();
    applyMenus();
  });
  watch(loop, async (newVal, oldVal) => {
    if (newVal === oldVal) return;
    toastType.value = "loop";
    showToast();
    onQueueChange();
    await nextTick();
    applyMenus();
  });
  watch(currentQueueIndex, onQueueChange);

  // --- Plyr settings menus (Plyr-specific DOM -- should be rewritten with the Video.js 10 migration) ---
  const playbackMenuInitialized = ref(false);
  const loopMenuInitialized = ref(false);
  let playbackButtons: NodeListOf<HTMLButtonElement> | null = null;
  let playbackValueSpan: HTMLElement | null = null;
  let loopButtons: NodeListOf<HTMLButtonElement> | null = null;
  let loopValueSpan: HTMLElement | null = null;

  function resetPlaybackMenuState(): void {
    playbackMenuInitialized.value = false;
    loopMenuInitialized.value = false;
    playbackButtons = null;
    playbackValueSpan = null;
    loopButtons = null;
    loopValueSpan = null;
  }

  function setChecked(buttons: NodeListOf<HTMLButtonElement> | null, value: string): void {
    if (!buttons) return;
    for (const btn of buttons) {
      btn.setAttribute("aria-checked", String(btn.getAttribute("value") === value));
    }
  }

  /** Builds the playback menu once, then only updates its checked state and label. */
  function applyPlaybackMenu(plyr: Plyr): void {
    const settings = getSettingsElements(plyr);
    const playbackBtn = settings?.buttons?.playback;
    const playbackPanel = settings?.panels?.playback;
    if (!playbackBtn || !playbackPanel) return;
    playbackBtn.removeAttribute("hidden");
    const title = getPlayerI18n(plyr)?.playback || "Playback";
    const currentLabel = getModeLabel(playbackMode.value, queueCount.value);

    if (!playbackMenuInitialized.value) {
      const menu = playbackPanel.querySelector('div[role="menu"]') as HTMLElement;
      menu.innerHTML = `
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" value="single">
                <span>${getModeLabel("single")}</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" value="sequential">
                <span>${getModeLabel("sequential")}</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" value="shuffle">
                <span>${getModeLabel("shuffle")}</span>
              </button>
            `;
      const buttons = menu.querySelectorAll<HTMLButtonElement>('button[data-plyr="playback"]');
      playbackButtons = buttons;
      setChecked(buttons, playbackMode.value);
      for (const btn of buttons) {
        btn.addEventListener("click", (event) => {
          const value = (event.currentTarget as HTMLElement).getAttribute("value") ?? "";
          if (value === "single") {
            clearPlaybackQueue();
          } else {
            cycleMode(playbackMode.value, {
              listing: getListing(),
              currentItem: getReq(),
              isShare: getters.isShare(),
              targetMode: value,
            });
          }
          if (playbackValueSpan) playbackValueSpan.textContent = getModeLabel(value);
          setChecked(buttons, value);
        });
      }
      const valueSpan = playbackBtn.querySelector<HTMLElement>("span .plyr__menu__value");
      if (valueSpan) {
        valueSpan.textContent = currentLabel;
        playbackValueSpan = valueSpan;
      } else {
        const label = playbackBtn.querySelector("span") as HTMLElement;
        label.innerHTML = `${title}: <span class="plyr__menu__value">${currentLabel}</span>`;
        playbackValueSpan = playbackBtn.querySelector<HTMLElement>("span .plyr__menu__value");
      }
      playbackMenuInitialized.value = true;
    } else {
      setChecked(playbackButtons, playbackMode.value);
      if (playbackValueSpan) {
        playbackValueSpan.textContent = currentLabel;
      }
    }
  }

  /** Builds the loop menu (off, loop all, loop single) once, then only updates it. */
  function applyLoopMenu(plyr: Plyr): void {
    const settings = getSettingsElements(plyr);
    const loopBtn = settings?.buttons?.loop;
    const loopPanel = settings?.panels?.loop;
    if (!loopBtn || !loopPanel) return;
    loopBtn.removeAttribute("hidden");
    const loopTitle = t("player.loop");
    const currentLoopLabel = getLoopLabel(loop.value);

    if (!loopMenuInitialized.value) {
      const menu = loopPanel.querySelector('div[role="menu"]') as HTMLElement;
      menu.innerHTML = `
              <button data-plyr="loop" type="button" role="menuitemradio" class="plyr__control" value="off">
                <span>${getLoopLabel("off")}</span>
              </button>
              <button data-plyr="loop" type="button" role="menuitemradio" class="plyr__control" value="all">
                <span>${getLoopLabel("all")}</span>
              </button>
              <button data-plyr="loop" type="button" role="menuitemradio" class="plyr__control" value="single">
                <span>${getLoopLabel("single")}</span>
              </button>
            `;
      const buttons = menu.querySelectorAll<HTMLButtonElement>('button[data-plyr="loop"]');
      loopButtons = buttons;
      setChecked(buttons, loop.value);
      for (const btn of buttons) {
        btn.addEventListener("click", (event) => {
          const value = (event.currentTarget as HTMLElement).getAttribute("value") ?? "";
          if (value !== loop.value) {
            mutations.setPlaybackQueue({
              queue: playbackQueue.value,
              currentIndex: currentQueueIndex.value,
              mode: playbackMode.value,
              loop: value as LoopMode,
            });
          }
          if (loopValueSpan) loopValueSpan.textContent = getLoopLabel(value);
          setChecked(buttons, value);
        });
      }
      const valueSpan = loopBtn.querySelector<HTMLElement>("span .plyr__menu__value");
      if (valueSpan) {
        valueSpan.textContent = currentLoopLabel;
        loopValueSpan = valueSpan;
      } else {
        const label = loopBtn.querySelector("span") as HTMLElement;
        label.innerHTML = `${loopTitle}: <span class="plyr__menu__value">${currentLoopLabel}</span>`;
        loopValueSpan = loopBtn.querySelector<HTMLElement>("span .plyr__menu__value");
      }
      loopMenuInitialized.value = true;
    } else {
      setChecked(loopButtons, loop.value);
      if (loopValueSpan) {
        loopValueSpan.textContent = currentLoopLabel;
      }
    }
  }

  return {
    queueCount,
    playbackQueue,
    currentQueueIndex,
    playbackMode,
    loop,
    toastVisible,
    toastType,
    toastIcon,
    playbackModeMessage,
    showToast,
    showQueuePrompt,
    toggleSingleLoop,
    cyclePlaybackModes,
    setupPlaybackQueue,
    playbackMenuInitialized,
    loopMenuInitialized,
    applyPlaybackMenu,
    applyLoopMenu,
    resetPlaybackMenuState,
  };
}
