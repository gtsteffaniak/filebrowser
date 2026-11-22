import type { ShallowRef } from "vue";
import type Plyr from "plyr";
import { getters, state } from "@/store/index.ts";

interface MediaKeyboardOptions {
  player: Readonly<ShallowRef<Plyr | null>>;
  getPreviewType: () => "audio" | "video";
  cyclePlaybackModes: () => void;
  toggleSingleLoop: () => void;
  showQueuePrompt: () => void;
  flashSkipFeedback: (rewind: boolean) => void;
  /** Opens/closes the desktop side panel (audio only). */
  toggleDesktopPanel: () => void;
}

/** Player shortcuts: P (playback mode), L (loop), Q (queue/panel) and arrow-key seek feedback */
export function useMediaKeyboard(options: MediaKeyboardOptions) {
  function handleKeydown(event: KeyboardEvent): void {
    if (event.repeat) return;
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    const key = event.key.toLowerCase();
    const target = event.target as HTMLElement | null;
    if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.tagName === "SELECT")) {
      return;
    }
    // 'P' and 'L' keys for playback mode and loop
    if (key === "p" || key === "l") {
      event.stopPropagation();
      event.preventDefault();
      if (key === "p") options.cyclePlaybackModes();
      if (key === "l") options.toggleSingleLoop();
      return;
    }
    // left/right arrows for seek feedback
    if (key === "arrowleft" || key === "arrowright") {
      if (!options.player.value) return;
      event.preventDefault();
      options.flashSkipFeedback(key === "arrowleft");
      return;
    }
    // "Q" open/close panel on desktop audio, queue prompt on video
    if (key === "q") {
      if (options.getPreviewType() === "audio" && !getters.isMobile()) {
        event.stopPropagation();
        event.preventDefault();
        options.toggleDesktopPanel();
      } else if (state.prompts.length === 0) {
        event.stopPropagation();
        event.preventDefault();
        options.showQueuePrompt();
      }
    }
  }

  function attachKeyboard(): void {
    document.addEventListener("keydown", handleKeydown);
  }

  function detachKeyboard(): void {
    document.removeEventListener("keydown", handleKeydown);
  }

  return { attachKeyboard, detachKeyboard };
}
