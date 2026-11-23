import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type App, createApp, h, reactive } from "vue";
import {
  type FakeMediaSession,
  fakePlayers,
  installMediaSession,
  removeMediaSession,
  settle,
  type Writable,
} from "@/composables/player/__tests__/mediaTestUtils.ts";
import AudioPlayer from "../AudioPlayer.vue";

vi.mock("@/composables/player/useMediaPlayer.ts", () => import("@/composables/player/__tests__/mediaTestUtils.ts"));
vi.mock("@/i18n/index.ts", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/router/index.ts", () => ({
  router: { currentRoute: { value: { query: {}, path: "/files/src/a.mp3", hash: "" } }, replace: vi.fn() },
}));
vi.mock("@/store/index.ts", async () => {
  const { reactive: reactiveState } = await import("vue");
  return {
    state: reactiveState({
      route: { path: "/files/src/a.mp3" },
      sources: { current: "src" },
      navigation: { isTransitioning: false },
      playbackQueue: { queue: [], currentIndex: -1, loop: "off", shouldTogglePlayPause: false },
    }),
    getters: { isMobile: () => false },
    mutations: { setPlaybackState: vi.fn() },
  };
});
vi.mock("@/utils/constants.ts", () => ({ globalVars: { baseURL: "/", name: "FB", loginIcon: "/icon.png" } }));
vi.mock("@/utils/url.js", () => ({
  buildItemUrl: (source: string, path: string) => `/files/${source}${path}`,
  removeTrailingSlash: (path: string) => path.replace(/\/$/, ""),
}));
vi.mock("@/utils/playbackQueue.js", () => ({
  formatArtist: (artist: string) => artist,
  getEndOfMediaAction: vi.fn(),
  getNextItem: vi.fn(),
  navigatePlaybackQueue: vi.fn(),
}));
vi.mock("@/player/seekOnRelease.js", () => ({ blockSeekOnInput: vi.fn(), enableSeekOnRelease: () => () => {} }));
vi.mock("@/components/files/AudioPanel.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/LyricsList.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/FloatingActionButton.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/player/PlayerOverlays.vue", () => ({ default: { render: () => null } }));
vi.mock("@/composables/player/useAudioVisualizer.ts", () => ({
  useAudioVisualizer: () => ({
    audioContext: { value: null },
    audioSource: { value: null },
    setupAudioVisualizer: vi.fn(),
    resumeAudioGraph: vi.fn(),
    cleanupAudioVisualizer: vi.fn(),
  }),
}));
vi.mock("@/composables/player/useDoubleTapSeek.ts", () => ({
  useDoubleTapSeek: () => ({
    skipFeedbackVisible: { value: false },
    skipFeedbackSide: { value: "left" },
    skipFeedbackIcon: { value: "" },
    skipFeedbackKey: { value: 0 },
    flashSkipFeedback: vi.fn(),
    setupDoubleTapSeek: vi.fn(),
    teardownDoubleTapSeek: vi.fn(),
    setupRewindForwardFeedback: vi.fn(),
    clearRewindForwardFeedback: vi.fn(),
    getGestureSurface: vi.fn(),
    isControlOrMenuTarget: vi.fn(),
  }),
}));
vi.mock("@/composables/player/useSwipeNavigationGestures.ts", () => ({
  useSwipeNavigationGestures: () => ({
    speedToastVisible: { value: false },
    speedToastMessage: { value: "" },
    setupSwipeGestures: vi.fn(),
    teardownSwipeGestures: vi.fn(),
    clearDismissAnimTimers: vi.fn(),
    resetEdgeGestureImmediate: vi.fn(),
  }),
}));
vi.mock("@/composables/player/useMediaKeyboard.ts", () => ({
  useMediaKeyboard: () => ({ attachKeyboard: vi.fn(), detachKeyboard: vi.fn() }),
}));
vi.mock("@/composables/player/usePlaybackQueue.ts", async () => {
  const { ref } = await import("vue");
  return {
    usePlaybackQueue: () => ({
      queueCount: ref(0),
      playbackQueue: ref([]),
      loop: ref("off"),
      toastVisible: ref(false),
      toastType: ref("mode"),
      toastIcon: ref(""),
      playbackModeMessage: ref(""),
      showQueuePrompt: vi.fn(),
      toggleSingleLoop: vi.fn(),
      cyclePlaybackModes: vi.fn(),
      playbackMenuInitialized: ref(false),
      loopMenuInitialized: ref(false),
      applyPlaybackMenu: vi.fn(),
      applyLoopMenu: vi.fn(),
      resetPlaybackMenuState: vi.fn(),
    }),
  };
});

const FILE_A = {
  fileKey: "|src|/a.mp3",
  raw: "/raw/a.mp3",
  req: { name: "a.mp3", type: "audio/mpeg", source: "src", path: "/a.mp3", metadata: { title: "Song A", albumArt: btoa("a") } },
};
const FILE_B = {
  fileKey: "|src|/b.mp3",
  raw: "/raw/b.mp3",
  req: { name: "b.mp3", type: "audio/mpeg", source: "src", path: "/b.mp3", metadata: { title: "Song B", albumArt: btoa("b") } },
};

type Props = InstanceType<typeof AudioPlayer>["$props"];

describe("AudioPlayer file swap", () => {
  let app: App;
  let container: HTMLElement;
  let props: Writable<Props>;
  let created: string[];
  let revoke: ReturnType<typeof vi.fn>;
  let unmounted = false;
  let session: FakeMediaSession;

  async function mountPlayer(initial: Partial<Props>) {
    props = reactive({ ...FILE_A, autoPlayEnabled: false, ...initial });
    container = document.createElement("div");
    document.body.appendChild(container);
    app = createApp({ render: () => h(AudioPlayer, props) });
    app.mount(container);
    await settle();
  }

  async function swapTo(file: typeof FILE_B) {
    Object.assign(props, file);
    await settle();
  }

  beforeEach(() => {
    fakePlayers.length = 0;
    unmounted = false;
    created = [];
    revoke = vi.fn();
    session = installMediaSession();
    vi.stubGlobal(
      "URL",
      Object.assign(URL, {
        createObjectURL: vi.fn(() => {
          const url = `blob:art-${created.length}`;
          created.push(url);
          return url;
        }),
        revokeObjectURL: revoke,
      }),
    );
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(function (this: HTMLMediaElement) {
      this.playbackRate = this.defaultPlaybackRate;
    });
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  });

  afterEach(() => {
    if (!unmounted) app.unmount();
    container.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    removeMediaSession();
  });

  it("keeps the Plyr instance and the element, and swaps src, metadata, artwork and media session", async () => {
    await mountPlayer({});
    const audio = container.querySelector("audio");
    const handlers = session.setActionHandler.mock.calls.length;
    expect(fakePlayers).toHaveLength(1);
    expect(audio?.getAttribute("src")).toBe("/raw/a.mp3");
    expect(container.querySelector(".audio-title")?.textContent?.trim()).toBe("Song A");
    expect(container.querySelector("img.album-art")?.getAttribute("src")).toBe("blob:art-0");

    await swapTo(FILE_B);

    expect(fakePlayers).toHaveLength(1);
    expect(fakePlayers[0]?.destroy).not.toHaveBeenCalled();
    expect(container.querySelector("audio")).toBe(audio);
    expect(audio?.getAttribute("src")).toBe("/raw/b.mp3");
    expect(container.querySelector(".audio-title")?.textContent?.trim()).toBe("Song B");
    expect(container.querySelector("img.album-art")?.getAttribute("src")).toBe("blob:art-1");
    expect(revoke).toHaveBeenCalledWith("blob:art-0");
    expect(session.metadata).toMatchObject({ title: "Song B", artwork: [{ src: "blob:art-1" }] });
    expect(session.setActionHandler).toHaveBeenCalledTimes(handlers);
  });
});
