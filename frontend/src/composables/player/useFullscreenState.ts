import { ref } from "vue";

interface FullscreenStateOptions {
  onEnter?: () => void;
}

/** Tracks the player's fullscreen state and frees the screen orientation while in fullscreen. */
export function useFullscreenState({ onEnter }: FullscreenStateOptions = {}) {
  const isFullscreen = ref(false);

  async function onFullscreenEnter(): Promise<void> {
    isFullscreen.value = true;
    onEnter?.();
    // Allow free rotation when video enters full screen mode. This works even if the device's orientation is currently locked.
    try {
      await screen.orientation.lock("any");
    } catch (error) {
      if ((error as DOMException).name !== "NotSupportedError") throw error;
    }
  }

  function onFullscreenExit(): void {
    isFullscreen.value = false;
    if (!screen.orientation?.unlock) return;
    try {
      screen.orientation.unlock();
    } catch (error) {
      if ((error as DOMException).name !== "NotSupportedError") throw error;
    }
  }
  return { isFullscreen, onFullscreenEnter, onFullscreenExit };
}
