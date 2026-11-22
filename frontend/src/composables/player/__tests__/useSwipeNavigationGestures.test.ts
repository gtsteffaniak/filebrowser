import { reactive, shallowRef } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { state } from "@/store/index.ts";
import { useGestureState } from "../useGestureState.ts";
import { useSwipeNavigationGestures } from "../useSwipeNavigationGestures.ts";
import { withSetup } from "./withSetup.ts";

vi.mock("@/store/index.ts", () => ({
  state: reactive({ navigation: { enabled: false, previousLink: "", nextLink: "" } }),
  getters: {
    currentPrompt: () => null,
    isPreviewPlaybackQueueNavMode: () => false,
    playbackQueueCanGoPrevious: () => false,
    playbackQueueCanGoNext: () => false,
  },
  mutations: { setNavigationGestureHint: vi.fn() },
}));

function fakePlayer(fullscreenActive = false) {
  return {
    fullscreen: { active: fullscreenActive, exit: vi.fn(), toggle: vi.fn() },
    speed: 1,
  };
}

function setup({ previewType = "video", player = fakePlayer(), lyricsActive = false } = {}) {
  const surface = document.createElement("div");
  document.body.appendChild(surface);
  const callbacks = { prev: vi.fn(), next: vi.fn(), close: vi.fn() };
  const { result: swipe } = withSetup(() => {
    const gestures = useGestureState();
    return useSwipeNavigationGestures({
      // biome-ignore lint/suspicious/noExplicitAny: minimal fake of the Plyr instance
      player: shallowRef(player as any),
      gestures,
      getPreviewType: () => previewType,
      isMobileLyricsActive: () => lyricsActive,
      getGestureSurface: () => surface,
      isControlOrMenuTarget: () => false,
      onNavigatePrevious: callbacks.prev,
      onNavigateNext: callbacks.next,
      onClosePreview: callbacks.close,
    });
  });
  swipe.setupSwipeGestures();
  return { swipe, surface, callbacks, player };
}

function drag(surface: HTMLElement, dx: number, dy: number) {
  surface.dispatchEvent(new MouseEvent("mousedown", { button: 0, clientX: 200, clientY: 200, bubbles: true }));
  document.dispatchEvent(new MouseEvent("mousemove", { clientX: 200 + dx, clientY: 200 + dy, bubbles: true }));
  document.dispatchEvent(new MouseEvent("mouseup", { clientX: 200 + dx, clientY: 200 + dy, bubbles: true }));
}

describe("useSwipeNavigationGestures", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    state.navigation.enabled = true;
    state.navigation.previousLink = "/prev";
    state.navigation.nextLink = "/next";
  });
  afterEach(() => {
    vi.useRealTimers();
    document.body.innerHTML = "";
  });

  it("navigates to the previous file after a long right swipe", () => {
    const { surface, callbacks } = setup();
    drag(surface, 140, 0);
    expect(callbacks.prev).toHaveBeenCalledTimes(1);
    expect(callbacks.next).not.toHaveBeenCalled();
  });

  it("navigates to the next file after a long left swipe", () => {
    const { surface, callbacks } = setup();
    drag(surface, -140, 0);
    expect(callbacks.next).toHaveBeenCalledTimes(1);
    expect(callbacks.prev).not.toHaveBeenCalled();
  });

  it("does not navigate on a short swipe or when there is no previous file", () => {
    const { surface, callbacks } = setup();
    drag(surface, 100, 0);
    state.navigation.previousLink = "";
    drag(surface, 200, 0);
    expect(callbacks.prev).not.toHaveBeenCalled();
  });

  it("closes the preview after a long downward swipe (delayed for the dismiss animation)", () => {
    const { surface, callbacks } = setup();
    drag(surface, 0, 120);
    expect(callbacks.close).not.toHaveBeenCalled();
    vi.advanceTimersByTime(120);
    expect(callbacks.close).toHaveBeenCalledTimes(1);
  });

  it("leaves fullscreen instead of closing when swiping down in fullscreen", () => {
    const player = fakePlayer(true);
    const { surface, callbacks } = setup({ player });
    drag(surface, 0, 120);
    vi.advanceTimersByTime(500);
    expect(player.fullscreen.exit).toHaveBeenCalledTimes(1);
    expect(callbacks.close).not.toHaveBeenCalled();
  });

  it("toggles fullscreen on a long upward swipe for video only", () => {
    const video = setup();
    drag(video.surface, 0, -120);
    expect(video.player.fullscreen.toggle).toHaveBeenCalledTimes(1);

    const audio = setup({ previewType: "audio" });
    drag(audio.surface, 0, -120);
    expect(audio.player.fullscreen.toggle).not.toHaveBeenCalled();
  });

  it("ignores gestures while the mobile lyrics overlay is open", () => {
    const { surface, callbacks } = setup({ lyricsActive: true });
    drag(surface, 140, 0);
    expect(callbacks.prev).not.toHaveBeenCalled();
  });

  it("stops reacting after teardown", () => {
    const { swipe, surface, callbacks } = setup();
    swipe.teardownSwipeGestures();
    drag(surface, 140, 0);
    expect(callbacks.prev).not.toHaveBeenCalled();
  });
});
