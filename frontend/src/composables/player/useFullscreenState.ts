import { onBeforeUnmount, ref } from "vue";
import { state } from "@/store/index.ts";

interface FullscreenStateOptions {
  onEnter?: () => void;
}

/** Keyboard Lock API (this only works in chromium, not in firefox sadly) */
type KeyboardWithLock = { lock?: (keyCodes?: string[]) => Promise<void>; unlock?: () => void };

function getKeyboard(): KeyboardWithLock | undefined {
  return (navigator as Navigator & { keyboard?: KeyboardWithLock }).keyboard;
}

function onEscapeKey(event: KeyboardEvent): void {
  if (event.key !== "Escape" || state.prompts.length > 0 || state.isSearchActive) return;
  if (!document.fullscreenElement) return;
  event.preventDefault();
  event.stopImmediatePropagation();
  void document.exitFullscreen();
}

/** Tracks the player's fullscreen state and frees the screen orientation while in fullscreen. */
export function useFullscreenState({ onEnter }: FullscreenStateOptions = {}) {
  const isFullscreen = ref(false);

  function lockEscape(): void {
    getKeyboard()
      ?.lock?.(["Escape"])
      .catch(() => {});
    window.addEventListener("keydown", onEscapeKey, true);
  }

  function unlockEscape(): void {
    window.removeEventListener("keydown", onEscapeKey, true);
    getKeyboard()?.unlock?.();
  }

  async function onFullscreenEnter(): Promise<void> {
    isFullscreen.value = true;
    onEnter?.();
    lockEscape();
    // Allow free rotation when video enters full screen mode. This works even if the device's orientation is currently locked.
    try {
      await screen.orientation.lock("any");
    } catch (error) {
      if ((error as DOMException).name !== "NotSupportedError") throw error;
    }
  }

  function onFullscreenExit(): void {
    isFullscreen.value = false;
    unlockEscape();
    if (!screen.orientation?.unlock) return;
    try {
      screen.orientation.unlock();
    } catch (error) {
      if ((error as DOMException).name !== "NotSupportedError") throw error;
    }
  }
  onBeforeUnmount(unlockEscape);

  return { isFullscreen, onFullscreenEnter, onFullscreenExit };
}
