import { shallowRef } from "vue";
import { beforeEach, describe, expect, it } from "vitest";
import type Plyr from "plyr";
import { setStoredCaptionSize } from "../playerStorage.ts";
import { useCaptionSize } from "../useCaptionSize.ts";

function fakePlayer() {
  const container = document.createElement("div");
  const captionSize = document.createElement("button");
  const player = {
    elements: { container, settings: { buttons: { captionSize } } },
  } as unknown as Plyr;
  return { player, container, captionSize };
}

describe("useCaptionSize", () => {
  beforeEach(() => localStorage.clear());

  it("applies exactly one caption-size class from storage", () => {
    const { player, container } = fakePlayer();
    container.classList.add("caption-size--small");
    setStoredCaptionSize("large");
    const { applyCaptionSizeClass } = useCaptionSize({ player: shallowRef(player), isVisible: () => true });
    applyCaptionSizeClass();
    expect([...container.classList].filter((c) => c.startsWith("caption-size--"))).toEqual([
      "caption-size--large",
    ]);
  });

  it("shows or hides the settings button", () => {
    const { player, captionSize } = fakePlayer();
    let visible = false;
    const { syncCaptionSizeSettingsVisibility } = useCaptionSize({
      player: shallowRef(player),
      isVisible: () => visible,
    });
    syncCaptionSizeSettingsVisibility();
    expect(captionSize.hasAttribute("hidden")).toBe(true);
    visible = true;
    syncCaptionSizeSettingsVisibility();
    expect(captionSize.hasAttribute("hidden")).toBe(false);
  });

  it("does nothing without a player", () => {
    const { applyCaptionSizeClass, syncCaptionSizeSettingsVisibility } = useCaptionSize({
      player: shallowRef<Plyr | null>(null),
      isVisible: () => true,
    });
    expect(() => {
      applyCaptionSizeClass();
      syncCaptionSizeSettingsVisibility();
    }).not.toThrow();
  });
});
