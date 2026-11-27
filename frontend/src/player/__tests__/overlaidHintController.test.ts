import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { enableOverlaidHintController, type OverlaidHintPlayer } from "@/player/overlaidHintController.ts";

type ControlsEvent = "controlsshown" | "controlshidden";

function createMockPlayer({ playing = true } = {}) {
  const container = document.createElement("div");
  container.className = "plyr plyr--video plyr--playing plyr--hide-controls";

  const overlaid = document.createElement("button");
  overlaid.className = "plyr__control plyr__control--overlaid";
  overlaid.innerHTML = '<svg><use href=""></use></svg>';
  container.appendChild(overlaid);

  const listeners = new Map<ControlsEvent, Array<() => void>>();

  const player: OverlaidHintPlayer = {
    elements: {
      container,
      buttons: { play: [overlaid] },
    },
    on(event, fn) {
      listeners.set(event, [...(listeners.get(event) ?? []), fn]);
    },
    off(event, fn) {
      listeners.set(
        event,
        (listeners.get(event) ?? []).filter((listener) => listener !== fn),
      );
    },
  };
  return {
    player,
    playing,
    container,
    emit(event: ControlsEvent) {
      for (const fn of listeners.get(event) ?? []) {
        fn();
      }
    },
  };
}

describe("overlaidHintController", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows overlaid hint on controlsshown even when post-play suppress is active", () => {
    const mock = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player: mock.player,
      hasStartedPlayback: () => true,
      baseUrl: "/files/",
    });

    api.syncPlayback(true);
    vi.advanceTimersByTime(400);
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(false);

    mock.emit("controlsshown");
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(true);

    api.cleanup();
  });

  it("hides overlaid hint on controlshidden while playing", () => {
    const mock = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player: mock.player,
      hasStartedPlayback: () => true,
      baseUrl: "/files/",
      isPlaying: mock.playing,
    });

    mock.emit("controlsshown");
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(true);

    mock.emit("controlshidden");
    vi.advanceTimersByTime(400);
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(false);

    api.cleanup();
  });

  it("clears overlaid hint classes when playback pauses", () => {
    const mock = createMockPlayer({ playing: true });
    const api = enableOverlaidHintController({
      player: mock.player,
      hasStartedPlayback: () => true,
      baseUrl: "/files/",
      isPlaying: mock.playing,
    });

    mock.emit("controlsshown");
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(true);

    api.syncPlayback(false);
    expect(mock.container.classList.contains("fb-overlaid--shown")).toBe(false);
    expect(mock.container.classList.contains("fb-overlaid--fade-in")).toBe(false);
    expect(mock.container.classList.contains("fb-overlaid--fade-out")).toBe(false);

    api.cleanup();
  });
});
