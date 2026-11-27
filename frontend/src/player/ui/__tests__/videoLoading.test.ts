import type Plyr from "plyr";
import { describe, expect, it } from "vitest";
import type { Writable } from "@/composables/player/__tests__/mediaTestUtils.ts";
import { enableVideoLoadingIndicator, type LoadingMedia, type LoadingPlayer } from "@/player/ui/videoLoading.ts";

type PlayerEvent = keyof Plyr.PlyrEventMap;

function fakePlayer() {
  const handlers = new Map<PlayerEvent, () => void>();
  const media: Writable<LoadingMedia> = {
    currentSrc: "video.mp4",
    src: "",
    paused: true,
    readyState: HTMLMediaElement.HAVE_NOTHING,
    networkState: HTMLMediaElement.NETWORK_IDLE,
  };
  const player: LoadingPlayer = {
    media,
    on: (evt, fn) => {
      handlers.set(evt, fn);
    },
    off: (evt) => {
      handlers.delete(evt);
    },
  };
  return {
    player,
    media,
    emit: (evt: PlayerEvent) => handlers.get(evt)?.(),
  };
}

function startPlayback() {
  const fake = fakePlayer();
  const states: boolean[] = [];
  enableVideoLoadingIndicator(fake.player, (show) => {
    states.push(show);
  });
  fake.emit("play");
  fake.media.readyState = HTMLMediaElement.HAVE_ENOUGH_DATA;
  fake.media.networkState = HTMLMediaElement.NETWORK_LOADING;
  fake.emit("playing");
  return { fake, states };
}

describe("video loading indicator", () => {
  it("stays hidden when the fetch stalls but playback continues from the buffer", () => {
    const { fake, states } = startPlayback();
    fake.emit("stalled");
    expect(states.at(-1)).toBe(false);
  });

  it("shows while playback waits for data", () => {
    const { fake, states } = startPlayback();
    fake.media.readyState = HTMLMediaElement.HAVE_CURRENT_DATA;
    fake.emit("waiting");
    expect(states.at(-1)).toBe(true);
  });

  it("stays hidden after an explicit pause while buffering", () => {
    const { fake, states } = startPlayback();
    fake.media.readyState = HTMLMediaElement.HAVE_CURRENT_DATA;
    fake.emit("waiting");
    expect(states.at(-1)).toBe(true);
    fake.emit("pause");
    expect(states.at(-1)).toBe(false);
    fake.media.readyState = HTMLMediaElement.HAVE_ENOUGH_DATA;
    fake.emit("canplay");
    expect(states.at(-1)).toBe(false);
  });
});
