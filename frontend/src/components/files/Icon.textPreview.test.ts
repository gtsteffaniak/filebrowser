import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, h, nextTick } from "vue";

const { mockState, mockGetters, setPopupPreview } = vi.hoisted(() => ({
  mockState: {
    sessionId: "test-session",
    user: {},
    shareInfo: null,
    req: { source: "main", path: "/", modified: "" },
    popupPreviewSourceInfo: null,
  },
  mockGetters: {
    sourcePermissions: () => ({ view: true }),
    previewPerms: () => ({ office: true, popup: true, image: true, video: true, audio: true, models: true, folder: true }),
    isShare: () => false,
    viewMode: () => "gallery",
    isDarkMode: () => false,
  },
  setPopupPreview: vi.fn(),
}));

vi.mock("@/store", () => ({
  state: mockState,
  getters: mockGetters,
  mutations: { setPopupPreview, setPreviewSource: vi.fn() },
}));

// The real TextPreview is async and pulls in highlight.js; a stub is enough to detect mounting.
vi.mock("@/utils/asyncComponent.js", () => ({
  createAsyncComponent: () => ({
    name: "TextPreviewStub",
    render: () => h("div", { class: "text-preview" }),
  }),
}));

import Icon from "./Icon.vue";

type ResizeCb = (entries: ResizeObserverEntry[]) => void;
let resizeCallback: ResizeCb | null = null;

interface IconProps {
  filename?: string;
  mimetype?: string;
  hasPreview?: boolean;
  path?: string;
  source?: string;
  textPreviewUrl?: string;
}

function mountIcon(props: IconProps = {}) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const app = createApp({
    render: () =>
      h(Icon, {
        filename: "notes.md",
        mimetype: "text/markdown",
        hasPreview: true,
        path: "/notes.md",
        source: "main",
        textPreviewUrl: "/api/resources/preview/text?path=%2Fnotes.md",
        ...props,
      }),
  });
  app.mount(el);
  return { el, app };
}

describe("Icon text preview width gate", () => {
  beforeEach(() => {
    setPopupPreview.mockClear();
    resizeCallback = null;
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(cb: ResizeCb) {
          resizeCallback = cb;
        }
        observe() {}
        disconnect() {}
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.innerHTML = "";
  });

  it("shows the type icon plus a preview-available badge on a small tile", async () => {
    const { el, app } = mountIcon();
    await nextTick();

    expect(el.querySelector(".text-preview")).toBeNull();
    const hint = el.querySelector(".overlay-icon--preview-hint");
    expect(hint).not.toBeNull();
    expect(hint?.textContent).toContain("saved_search");
    app.unmount();
  });

  it("hides the badge when popup previews are disabled", async () => {
    const original = mockGetters.previewPerms;
    mockGetters.previewPerms = () => ({
      office: true,
      popup: false,
      image: true,
      video: true,
      audio: true,
      models: true,
      folder: true,
    });
    try {
      const { el, app } = mountIcon();
      await nextTick();
      expect(el.querySelector(".overlay-icon--preview-hint")).toBeNull();
      app.unmount();
    } finally {
      mockGetters.previewPerms = original;
    }
  });

  it("keeps the hover popup preview working on a small tile", async () => {
    vi.useFakeTimers();
    const { el, app } = mountIcon();
    await nextTick();

    el.querySelector(".image-preview")?.dispatchEvent(new MouseEvent("mouseenter"));
    vi.advanceTimersByTime(300);

    expect(setPopupPreview).toHaveBeenCalledTimes(1);
    expect(setPopupPreview.mock.calls[0]?.[0]).toMatchObject({
      type: "text",
      path: "/notes.md",
      textUrl: "/api/resources/preview/text?path=%2Fnotes.md",
    });
    vi.useRealTimers();
    app.unmount();
  });

  it("mounts TextPreview once the tile is wide enough", async () => {
    const { el, app } = mountIcon();
    await nextTick();
    expect(el.querySelector(".text-preview")).toBeNull();

    // Simulate the ResizeObserver reporting a gallery-sized tile.
    resizeCallback?.([{ contentRect: { width: 400 } } as ResizeObserverEntry]);
    await nextTick();
    await nextTick();

    expect(el.querySelector(".text-preview")).not.toBeNull();
    expect(el.querySelector(".overlay-icon--preview-hint")).toBeNull();
    app.unmount();
  });
});
