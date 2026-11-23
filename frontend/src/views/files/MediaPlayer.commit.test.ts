import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  type App,
  createApp,
  defineAsyncComponent,
  defineComponent,
  h,
  onBeforeUnmount,
  type PropType,
} from "vue";
import { settle } from "@/composables/player/__tests__/mediaTestUtils.ts";
import type { FileListItem, ReqObject } from "@/store/types.ts";
import type { LyricLines } from "@/utils/lyrics.ts";
import MediaPlayer from "./MediaPlayer.vue";

const hoisted = vi.hoisted(() => ({
  audioMounts: 0,
  audioUnmounts: 0,
  videoMounts: 0,
  gate: null as null | Promise<void>,
}));

vi.mock("@/api/index.js", () => ({
  mediaApi: {
    getItemMediaMetadata: vi.fn(async () => null),
    getLyrics: vi.fn(async () => {
      await hoisted.gate;
      return { lyrics: "[00:01.00]hello", format: "lrc" };
    }),
    getSubtitleContent: vi.fn(async () => "WEBVTT\n\n00:00.000 --> 00:01.000\nhi"),
    getDirectoryMetadataMap: vi.fn(async () => ({})),
  },
  resourcesApi: { getViewURL: (_source: string, path: string) => `/raw${path}` },
}));
vi.mock("@/store/index.ts", async () => {
  const { reactive } = await import("vue");
  const storeState = reactive({
    req: {} as Partial<ReqObject>,
    shareInfo: {},
    navigation: { isTransitioning: false },
  });
  return {
    state: storeState,
    getters: {
      isLoggedIn: () => true,
      isShare: () => false,
      isDarkMode: () => false,
      fileViewingDisabled: () => false,
      previewType: () => (String(storeState.req.type).startsWith("video") ? "video" : "audio"),
      previewPerms: () => ({ autoplayMedia: false }),
      isPreviewPlaybackQueueNavMode: () => false,
    },
    mutations: {
      patchRequestFileMediaMetadata: (enriched: Partial<ReqObject>) => {
        storeState.req = { ...storeState.req, ...enriched };
      },
      setupNavigation: vi.fn(),
      patchListingMetadata: vi.fn(),
    },
  };
});
vi.mock("@/composables/preview/usePreviewNavigation.ts", async () => {
  const { ref } = await import("vue");
  return {
    usePreviewNavigation: () => ({
      listing: ref(null),
      loadListingContext: async () => {},
      updatePreview: async () => {},
      selectCurrentItem: vi.fn(),
      navigatePrevious: vi.fn(),
      navigateNext: vi.fn(),
      exitPreviewFromImageGesture: vi.fn(),
    }),
  };
});
vi.mock("@/utils/asyncComponent.js", () => ({ createAsyncComponent: (loader: () => Promise<{ default: never }>) =>
    defineAsyncComponent(() => loader().then((module) => module.default)) }));
vi.mock("@/utils/previewAutoplay.js", () => ({ shouldAutoPlayPreview: () => false }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => h("div", { class: "spinner" }) } }));
vi.mock("@/components/player/AudioPlayer.vue", () => ({
  default: defineComponent({
    props: {
      fileKey: { type: String as PropType<string>, default: "" },
      req: { type: Object as PropType<ReqObject>, default: () => ({}) },
      raw: { type: String as PropType<string>, default: "" },
      lyrics: { type: Array as PropType<LyricLines>, default: () => [] },
      listing: { type: Array as PropType<FileListItem[]>, default: () => [] },
      autoPlayEnabled: Boolean,
    },
    setup(props) {
      hoisted.audioMounts += 1;
      onBeforeUnmount(() => {
        hoisted.audioUnmounts += 1;
      });
      return () =>
        h("div", { class: "audio-stub", "data-key": props.fileKey, "data-raw": props.raw, "data-lyrics": props.lyrics.length });
    },
  }),
}));
vi.mock("@/components/player/VideoPlayer.vue", () => ({
  default: defineComponent({
    props: {
      req: { type: Object as PropType<ReqObject>, default: () => ({}) },
      raw: { type: String as PropType<string>, default: "" },
      subtitlesList: { type: Array as PropType<{ src: string }[]>, default: () => [] },
      listing: { type: Array as PropType<FileListItem[]>, default: () => [] },
      autoPlayEnabled: Boolean,
    },
    setup(props) {
      hoisted.videoMounts += 1;
      return () => h("div", { class: "video-stub", "data-raw": props.raw, "data-subs": props.subtitlesList.map((sub) => sub.src).join(",") });
    },
  }),
}));

import { mediaApi } from "@/api/index.js";
import { state } from "@/store/index.ts";

const audio = (name: string, extra: Partial<ReqObject> = {}) =>
  ({
    name,
    type: "audio/mpeg",
    source: "src",
    path: `/${name}`,
    metadata: { hasLyrics: true },
    ...extra,
  }) as ReqObject;
const video = (name: string) =>
  ({
    name,
    type: "video/mp4",
    source: "src",
    path: `/${name}`,
    subtitles: [{ name: "en.vtt", embedded: false, srclang: "en" }],
  }) as ReqObject;

describe("MediaPlayer committed source", () => {
  let app: App;
  let container: HTMLElement;
  let urls: string[];
  let revoke: ReturnType<typeof vi.fn>;
  let unmounted = false;

  async function mountView(req: ReqObject) {
    state.req = req;
    container = document.createElement("div");
    document.body.appendChild(container);
    app = createApp({ render: () => h(MediaPlayer) });
    app.mount(container);
    await settle();
  }

  const audioStub = () => container.querySelector(".audio-stub") as HTMLElement | null;

  beforeEach(() => {
    hoisted.audioMounts = 0;
    hoisted.audioUnmounts = 0;
    hoisted.videoMounts = 0;
    hoisted.gate = null;
    unmounted = false;
    urls = [];
    revoke = vi.fn();
    vi.clearAllMocks();
    vi.stubGlobal(
      "URL",
      Object.assign(URL, {
        createObjectURL: vi.fn(() => {
          const url = `blob:sub-${urls.length}`;
          urls.push(url);
          return url;
        }),
        revokeObjectURL: revoke,
      }),
    );
  });

  afterEach(() => {
    if (!unmounted) app.unmount();
    container.remove();
    vi.unstubAllGlobals();
  });

  it("commits the first file after everything is loaded", async () => {
    await mountView(audio("a.mp3"));
    expect(audioStub()?.dataset.key).toBe("|src|/a.mp3");
    expect(audioStub()?.dataset.raw).toBe("/raw/a.mp3");
    expect(audioStub()?.dataset.lyrics).toBe("1");
    expect(container.querySelector(".spinner")).toBeNull();
  });

  it("keeps the previous file mounted until the next one is ready", async () => {
    await mountView(audio("a.mp3"));
    let release = () => {};
    hoisted.gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    state.req = audio("b.mp3");
    await settle();
    expect(audioStub()?.dataset.raw).toBe("/raw/a.mp3");
    expect(container.querySelector(".spinner")).not.toBeNull();

    release();
    await settle();
    expect(audioStub()?.dataset.raw).toBe("/raw/b.mp3");
    expect(container.querySelector(".spinner")).toBeNull();
    expect(hoisted.audioMounts).toBe(1);
    expect(hoisted.audioUnmounts).toBe(0);
  });

  it("only commits the last of several quick navigations", async () => {
    await mountView(audio("a.mp3"));
    let release = () => {};
    hoisted.gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    state.req = audio("b.mp3");
    await settle();
    state.req = audio("c.mp3");
    await settle();
    release();
    await settle();
    expect(audioStub()?.dataset.raw).toBe("/raw/c.mp3");
    expect(hoisted.audioMounts).toBe(1);
  });

  it("loads once when a metadata patch for the same file arrives during the load", async () => {
    await mountView(audio("a.mp3"));
    vi.mocked(mediaApi.getLyrics).mockClear();
    let release = () => {};
    hoisted.gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    state.req = audio("b.mp3");
    await settle();
    state.req = { ...state.req, metadata: { hasLyrics: true, title: "patched" } };
    await settle();
    release();
    await settle();
    expect(mediaApi.getLyrics).toHaveBeenCalledTimes(1);
    expect(audioStub()?.dataset.raw).toBe("/raw/b.mp3");
    expect(hoisted.audioMounts).toBe(1);
  });

  it("does not fetch subtitles and lyrics again for a re-commit of the same file", async () => {
    await mountView(audio("a.mp3"));
    vi.mocked(mediaApi.getLyrics).mockClear();
    state.req = { ...state.req, metadata: { hasLyrics: true, title: "patched" } };
    await settle();
    expect(mediaApi.getLyrics).not.toHaveBeenCalled();
    expect(audioStub()?.dataset.lyrics).toBe("1");
  });

  it("revokes the subtitle blob URLs of the previous video and on unmount", async () => {
    await mountView(video("a.mp4"));
    expect(container.querySelector(".video-stub")?.getAttribute("data-subs")).toBe("blob:sub-0");
    state.req = video("b.mp4");
    await settle();
    expect(container.querySelector(".video-stub")?.getAttribute("data-subs")).toBe("blob:sub-1");
    expect(revoke).toHaveBeenCalledWith("blob:sub-0");
    app.unmount();
    unmounted = true;
    expect(revoke).toHaveBeenCalledWith("blob:sub-1");
  });

  it("swaps the component when the kind changes", async () => {
    await mountView(audio("a.mp3"));
    state.req = video("b.mp4");
    await settle();
    expect(audioStub()).toBeNull();
    expect(container.querySelector(".video-stub")).not.toBeNull();
  });
});
