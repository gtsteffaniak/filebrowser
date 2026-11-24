import { beforeEach, describe, expect, it, vi } from "vitest";

const { store, mutations } = vi.hoisted(() => ({
  store: {
    share: false,
    current: "inbox",
    selected: [0] as number[],
    items: [
      {
        path: "/a.txt",
        name: "a.txt",
        type: "file",
        source: "data",
      },
    ] as Array<{ path: string; name: string; type: string; source: string }>,
    permissions: {
      data: { view: true, download: true, modify: false, create: false, delete: false },
      inbox: { view: true, download: true, modify: true, create: true, delete: true },
      share: { view: true, download: true, modify: false, create: true, delete: false },
    },
  },
  mutations: { showPrompt: vi.fn() },
}));

vi.mock("@/store", () => ({
  state: {
    get selected() {
      return store.selected;
    },
    get req() {
      return { path: "/", source: store.current, items: store.items };
    },
    get route() {
      return { path: "/other" };
    },
    get shareInfo() {
      return store.share ? { hash: "h" } : null;
    },
  },
  mutations,
  getters: {
    isShare: () => store.share,
    sourcePermissions: (source?: string) =>
      store.permissions[(source ?? store.current) as keyof typeof store.permissions],
  },
}));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/api", () => ({
  resourcesApi: {
    moveCopy: vi.fn().mockResolvedValue(undefined),
    moveCopyPublic: vi.fn().mockResolvedValue(undefined),
  },
}));
vi.mock("@/router", () => ({ router: {} }));
vi.mock("@/utils/upload.ts", () => ({ checkConflict: vi.fn(() => false) }));
vi.mock("@/utils/appNotifications.js", () => ({ notifyMoveCopyFailure: vi.fn() }));

import { useListingClipboard } from "../useListingClipboard.ts";

const { copyCut, handleInternalPaste } = useListingClipboard();

const fakeEvent = { target: { tagName: "DIV" } } as unknown as KeyboardEvent;

function selectFrom(source: string) {
  store.selected = [0];
  store.items = [{ path: "/a.txt", name: "a.txt", type: "file", source }];
}

describe("useListingClipboard internal paste permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.share = false;
    store.current = "inbox";
    store.selected = [0];
    // clipboard module state is per-import; copyCut in the previous test leaves
    // an entry behind for the next — copyCut itself never clears, so resetting
    // `state` isn't possible without the test seam. Each test overwrites it
    // immediately, so just re-drive copyCut every time.
  });

  it("pastes a copy out of a read-only source into a writable one", async () => {
    selectFrom("data");
    copyCut(fakeEvent, "c");
    await handleInternalPaste();
    expect(mutations.showPrompt).toHaveBeenCalledOnce();
  });

  it("refuses a cut out of a source that cannot modify", async () => {
    selectFrom("data");
    copyCut(fakeEvent, "x");
    await handleInternalPaste();
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("refuses any paste into a source that cannot create", async () => {
    store.current = "data";
    selectFrom("inbox");
    copyCut(fakeEvent, "c");
    await handleInternalPaste();
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("in a share, requires modify even for a copy", async () => {
    store.share = true;
    store.permissions.share.modify = false;
    store.permissions.share.create = true;
    store.current = "share";
    selectFrom("share");
    copyCut(fakeEvent, "c");
    await handleInternalPaste();
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });
});
