import type { ShallowRef } from "vue";
import type Plyr from "plyr";
import { getSettingsElements } from "./settingsElements.ts";
import { CAPTION_SIZE_IDS, getStoredCaptionSize } from "./playerStorage.ts";

interface CaptionSizeOptions {
  player: Readonly<ShallowRef<Plyr | null>>;
  isVisible: () => boolean;
}

/** Caption size class on the player container and the settings button visibility */
export function useCaptionSize({ player, isVisible }: CaptionSizeOptions) {
  function applyCaptionSizeClass(): void {
    const el = player.value?.elements?.container;
    if (!el) {
      return;
    }
    for (const id of CAPTION_SIZE_IDS) {
      el.classList.remove(`caption-size--${id}`);
    }
    el.classList.add(`caption-size--${getStoredCaptionSize()}`);
  }

  function syncCaptionSizeSettingsVisibility(): void {
    const current = player.value;
    const btn = current ? getSettingsElements(current)?.buttons?.captionSize : undefined;
    if (!btn) {
      return;
    }
    if (isVisible()) {
      btn.removeAttribute("hidden");
    } else {
      btn.setAttribute("hidden", "");
    }
  }

  return { applyCaptionSizeClass, syncCaptionSizeSettingsVisibility };
}
