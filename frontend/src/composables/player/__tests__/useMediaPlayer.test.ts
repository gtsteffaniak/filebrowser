import type Plyr from "plyr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, shallowRef } from "vue";
import { mutations, state } from "@/store/index.ts";
import { useMediaPlayer } from "../useMediaPlayer.ts";
import { withSetup } from "./mediaTestUtils.ts";

const hoisted = vi.hoisted(() => ({
  state: {
    route: { path: "/files/src/a.mp4" },
    sources: { current: "src" },
    playbackQueue: { queue: [], currentIndex: -1, loop: "off", shouldTogglePlayPause: false },
  },
}));

vi.mock("plyr", () => ({ default: vi.fn() }));
vi.mock("@/i18n/index.ts", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/router/index.ts", () => ({ router: { currentRoute: { value: { query: {}, path: "", hash: "" } }, replace: vi.fn() } }));
vi.mock("@/store/index.ts", async () => {
  const { reactive } = await import("vue");
  return { state: reactive(hoisted.state), mutations: { setPlaybackState: vi.fn() } };
});
vi.mock("@/utils/constants.ts", () => ({ globalVars: { baseURL: "/" } }));
vi.mock("@/utils/playbackQueue.js", () => ({ getEndOfMediaAction: vi.fn(), navigatePlaybackQueue: vi.fn() }));
vi.mock("@/utils/url.js", () => ({
  buildItemUrl: (source: string, path: string) => `/files/${source}${path}`,
  removeTrailingSlash: (path: string) => path.replace(/\/$/, ""),
}));

function makeMedia(): HTMLMediaElement {
  const media = document.createElement("video");
  media.load = vi.fn(() => {
    media.playbackRate = media.defaultPlaybackRate;
  });
  media.pause = vi.fn();
  return media;
}

function setup(kind: "audio" | "video" = "video") {
  const media = makeMedia();
  media.setAttribute("src", "blob:a");
  const fake: Partial<Plyr> & { media: HTMLMediaElement } = { speed: 1, destroy: vi.fn(), media };
  const plyr = fake as Plyr & { media: HTMLMediaElement };
  const player = shallowRef<Plyr | null>(plyr);
  const hooks = { destroy: vi.fn(), beforeStopMedia: vi.fn(), onReleaseFile: vi.fn(), restoreSource: vi.fn() };
  const { result } = withSetup(() =>
    useMediaPlayer({
      kind,
      player,
      getMediaElement: () => media,
      getRequestedFile: () => ({ source: "src", path: "/a.mp4" }),
      onPlaybackState: vi.fn(),
      ...hooks,
    }),
  );
  result.setMountedFile("src", "/a.mp4");
  return { media, plyr, player, hooks, ...result };
}

async function navigateTo(path: string) {
  state.route.path = path;
  await nextTick();
}

describe("useMediaPlayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.route.path = "/files/src/a.mp4";
  });

  describe("releaseFile", () => {
    it("stops the media but keeps the instance", () => {
      const { releaseFile, media, plyr, player, hooks } = setup();
      releaseFile();
      expect(hooks.beforeStopMedia).toHaveBeenCalledOnce();
      expect(media.pause).toHaveBeenCalled();
      expect(media.hasAttribute("src")).toBe(false);
      expect(media.load).toHaveBeenCalled();
      expect(hooks.onReleaseFile).toHaveBeenCalledOnce();
      expect(player.value).toBe(plyr);
      expect(plyr.destroy).not.toHaveBeenCalled();
      expect(hooks.destroy).not.toHaveBeenCalled();
    });

    it("keeps the playback rate across the load, also when released twice", () => {
      const { releaseFile, reapplyPlaybackRate, media, plyr } = setup();
      media.playbackRate = 1.5;
      releaseFile();
      releaseFile();
      expect(media.playbackRate).toBe(1.5);
      media.playbackRate = 1;
      reapplyPlaybackRate();
      expect(plyr.speed).toBe(1.5);
    });

    it("forgets the selected caption track, so a track at the same index in the next file is enabled", () => {
      const { releaseFile, plyr } = setup();
      const captions = { currentTrack: 0, currentTrackNode: {} as TextTrack };
      Object.assign(plyr, { captions });
      releaseFile();
      expect(captions.currentTrack).toBe(-1);
      expect(captions.currentTrackNode).toBeUndefined();
    });

    it("reports that playback stopped, because load() drops the pause event", () => {
      const { releaseFile } = setup();
      releaseFile();
      expect(mutations.setPlaybackState).toHaveBeenCalledWith(false);
    });
  });

  describe("route leave", () => {
    it("releases the file instead of destroying", async () => {
      const { hooks, media, plyr } = setup();
      await navigateTo("/files/src/");
      expect(hooks.destroy).not.toHaveBeenCalled();
      expect(hooks.onReleaseFile).toHaveBeenCalledOnce();
      expect(media.hasAttribute("src")).toBe(false);
      expect(plyr.destroy).not.toHaveBeenCalled();
    });

    it("does nothing while the route still matches the file", async () => {
      const { hooks } = setup();
      await navigateTo("/files/src/a.mp4/");
      await navigateTo("/files/src/a.mp4#t=1");
      expect(hooks.onReleaseFile).not.toHaveBeenCalled();
      expect(hooks.destroy).not.toHaveBeenCalled();
    });

    it("releases only once per mounted file", async () => {
      const { hooks, setMountedFile } = setup();
      await navigateTo("/files/src/");
      await navigateTo("/files/src/b.mp4");
      expect(hooks.onReleaseFile).toHaveBeenCalledOnce();
      setMountedFile("src", "/b.mp4");
      await navigateTo("/files/src/c.mp4");
      expect(hooks.onReleaseFile).toHaveBeenCalledTimes(2);
    });

    it("still destroys when there is no file path", () => {
      const media = makeMedia();
      const destroy = vi.fn();
      const { result } = withSetup(() =>
        useMediaPlayer({
          kind: "video",
          player: shallowRef<Plyr | null>(null),
          getMediaElement: () => media,
          getRequestedFile: () => null,
          onPlaybackState: vi.fn(),
          destroy,
        }),
      );
      result.stopIfRouteLeftThisFile();
      expect(destroy).toHaveBeenCalledOnce();
    });

    it("does nothing after the final teardown", async () => {
      const { hooks, lifecycle } = setup();
      lifecycle.tornDown = true;
      await navigateTo("/files/src/");
      expect(hooks.onReleaseFile).not.toHaveBeenCalled();
    });
  });
});
