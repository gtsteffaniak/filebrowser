import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api", () => ({
  resourcesApi: {
    post: vi.fn(),
    postPublic: vi.fn(),
    signalUploadPause: vi.fn(),
    listDirectoryEntries: vi.fn(),
  },
}));
vi.mock("@/notify", () => ({ notify: { showSuccessToast: vi.fn() } }));
vi.mock("@/i18n", () => ({ default: { global: { t: (key, params) => `${key}:${params?.count}` } } }));
vi.mock("@/store", () => ({
  state: {
    req: { source: "test", path: "/base/", items: [] },
    user: { fileLoading: { maxConcurrentUpload: 10, uploadChunkSizeMb: 5 } },
    shareInfo: { hash: "share" },
  },
  mutations: { setIsUploading: vi.fn(), setReload: vi.fn() },
}));
vi.mock("@/store/getters", () => ({ getters: { isShare: vi.fn(() => false) } }));
vi.mock("@/utils/appNotifications", () => ({
  notifyUploadComplete: vi.fn(),
  notifyUploadError: vi.fn(),
}));

import { resourcesApi } from "@/api";
import { notify } from "@/notify";
import { state } from "@/store";
import { getters } from "@/store/getters";
import { isSameSize, numberedName, uploadManager } from "./upload";

function addUpload(status = "uploading", file = new Blob(["a"])) {
  uploadManager.queue.push({
    id: uploadManager.nextId++,
    type: "file",
    source: "test",
    path: "/file",
    file,
    size: file.size,
    status,
    progress: 0,
    chunkOffset: 0,
    lastProgressTime: Date.now(),
    connectionIssue: false,
  });
  return uploadManager.queue.at(-1);
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(0);
  vi.clearAllMocks();
  vi.spyOn(console, "log").mockImplementation(() => {});
  getters.isShare.mockReturnValue(false);
  uploadManager.queue.splice(0);
  uploadManager.activeUploads = 0;
  uploadManager.nextId = 0;
  uploadManager.isOverallPaused = false;
  uploadManager.hadActiveUploads = false;
  uploadManager.lastUploadActivityTime = null;
});

afterEach(() => {
  for (const id of uploadManager.progressTimeouts.keys()) {
    uploadManager.clearProgressTimeout(id);
  }
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("upload stall detection", () => {
  it("allows an upload to wait while another progresses, then detects a batch stall", async () => {
    const waiting = addUpload();
    const progressing = addUpload();
    uploadManager.startProgressTimeout(waiting);
    uploadManager.startProgressTimeout(progressing);

    for (let i = 0; i < 7; i++) {
      await vi.advanceTimersByTimeAsync(5000);
      // The API rounds progress, so successive events can have the same percent.
      uploadManager.updateProgress(progressing, 1);
      expect(waiting.status).toBe("uploading");
    }
    await vi.advanceTimersByTimeAsync(9999);
    expect(waiting.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(waiting.status).toBe("paused");
    expect(progressing.status).toBe("paused");
    expect(waiting.connectionIssue).toBe(true);
    expect(uploadManager.progressTimeouts.size).toBe(0);
  });

  it("still pauses a single upload after ten seconds without activity", async () => {
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    await vi.advanceTimersByTimeAsync(9999);
    expect(upload.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(upload.status).toBe("paused");
  });

  it.each([false, true])("keeps completion activity after a sibling finishes (share=%s)", async (share) => {
    getters.isShare.mockReturnValue(share);
    const request = deferred();
    const post = share ? resourcesApi.postPublic : resourcesApi.post;
    post.mockReturnValueOnce(request.promise);
    const waiting = addUpload();
    const finishing = addUpload("pending");
    uploadManager.activeUploads = 1;
    uploadManager.startProgressTimeout(waiting);
    const done = uploadManager.startFileUpload(finishing);

    await vi.advanceTimersByTimeAsync(9900);
    request.resolve("");
    await done;
    expect(finishing.status).toBe("completed");
    await vi.advanceTimersByTimeAsync(9999);
    expect(waiting.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(waiting.status).toBe("paused");
  });

  it.each([false, true])("counts a successful chunk even without progress events (share=%s)", async (share) => {
    getters.isShare.mockReturnValue(share);
    const first = deferred();
    const second = deferred();
    const post = share ? resourcesApi.postPublic : resourcesApi.post;
    post.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const waiting = addUpload();
    const chunked = addUpload("pending", new Blob([new Uint8Array(5 * 1024 * 1024 + 1)]));
    uploadManager.activeUploads = 1;
    uploadManager.startProgressTimeout(waiting);
    const done = uploadManager.startFileUpload(chunked);

    await vi.advanceTimersByTimeAsync(9900);
    first.resolve("");
    await vi.advanceTimersByTimeAsync(100);
    expect(chunked.chunkOffset).toBe(5 * 1024 * 1024);
    expect(waiting.status).toBe("uploading");
    second.resolve("");
    await done;
  });

  it.each(["pause", "cancel"])("ignores late progress after %s", async (action) => {
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    if (action === "pause") {
      await uploadManager.pause(upload.id);
    } else {
      uploadManager.cancel(upload.id);
    }
    uploadManager.updateProgress(upload, 50);
    expect(uploadManager.progressTimeouts.size).toBe(0);
  });

  it("gives a resumed upload a fresh inactivity window", async () => {
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    await uploadManager.pause(upload.id);
    await vi.advanceTimersByTimeAsync(20000);
    const request = deferred();
    resourcesApi.post.mockReturnValueOnce(request.promise);
    uploadManager.resume(upload.id);
    await vi.advanceTimersByTimeAsync(9999);
    expect(upload.status).toBe("uploading");
    request.resolve("");
    await vi.advanceTimersByTimeAsync(0);
    expect(upload.status).toBe("completed");
    expect(uploadManager.progressTimeouts.size).toBe(0);
  });
});


describe("numberedName", () => {
  it("puts the suffix before the extension", () => {
    expect(numberedName("DSCF8750.MOV", 1)).toBe("DSCF8750_01.MOV");
    expect(numberedName("a.b.mov", 12)).toBe("a.b_12.mov");
    expect(numberedName("README", 2)).toBe("README_02");
    expect(numberedName(".env", 1)).toBe(".env_01");
  });
});

describe("isSameSize", () => {
  it("accepts the exact size and the 4 KiB block-rounded disk usage", () => {
    expect(isSameSize(2500000, 2500000)).toBe(true);
    expect(isSameSize(2502656, 2500000)).toBe(true);
    expect(isSameSize(0, 0)).toBe(true);
  });

  it("rejects empty or otherwise different remote files", () => {
    expect(isSameSize(0, 2500000)).toBe(false);
    expect(isSameSize(4096, 2500000)).toBe(false);
    expect(isSameSize(2506752, 2500000)).toBe(false);
    expect(isSameSize(4096, 0)).toBe(false);
  });
});

describe("skip existing files", () => {
  const fileOf = (name, size) => ({ name, size });
  const item = (relativePath, size) => ({
    file: fileOf(relativePath.split("/").pop(), size),
    relativePath,
  });

  it("skips same-size files, keeps missing ones and flags size mismatches for replacement", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([
        ["done.mov", { size: 100, type: "file" }],
        ["empty.mov", { size: 0, type: "file" }],
      ])
    );
    const { items, skipped } = await uploadManager.filterExistingItems("/base/", [
      item("Cam/done.mov", 100),
      item("Cam/empty.mov", 500),
      item("Cam/new.mov", 300),
    ]);

    expect(resourcesApi.listDirectoryEntries).toHaveBeenCalledWith("test", "/base/Cam/");
    expect(skipped).toBe(1);
    expect(items.map((i) => i.relativePath)).toEqual(["Cam/empty.mov", "Cam/new.mov"]);
    expect(items[0].overwriteExisting).toBe(true);
    expect(items[1].overwriteExisting).toBeUndefined();
  });

  it("treats every file as missing when the destination folder can't be listed", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(null);
    const { items, skipped } = await uploadManager.filterExistingItems("/base/", [
      item("Cam/sub/a.mov", 1),
    ]);
    expect(skipped).toBe(0);
    expect(items).toHaveLength(1);
    expect(items[0].overwriteExisting).toBeUndefined();
  });

  it("uploads under a numbered name when the existing entry is a folder", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["a.mov", { size: 1, type: "directory" }]])
    );
    const { items, skipped } = await uploadManager.filterExistingItems("/base/", [item("a.mov", 1)]);
    expect(skipped).toBe(0);
    expect(items[0].relativePath).toBe("a_01.mov");
    expect(items[0].overwriteExisting).toBeUndefined();
  });

  it("never overwrites a non-empty file of a different size: uploads it as name_01, then name_02", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([
        ["clip.mov", { size: 100, type: "file" }],
        ["clip_01.mov", { size: 100, type: "file" }],
      ])
    );
    const { items, skipped, renamed } = await uploadManager.filterExistingItems("/base/", [
      item("Cam/clip.mov", 500),
    ]);
    // "Cam/" listing is the mocked one above, so the numbered names are taken from it
    expect(skipped).toBe(0);
    expect(renamed).toBe(1);
    expect(items[0].relativePath).toBe("Cam/clip_02.mov");
    expect(items[0].uploadName).toBe("clip_02.mov");
    expect(items[0].overwriteExisting).toBeUndefined();
  });

  it("skips a file when a numbered copy of it with the same size already exists", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([
        ["clip.mov", { size: 100, type: "file" }],
        ["clip_01.mov", { size: 500, type: "file" }],
      ])
    );
    const { items, skipped, renamed } = await uploadManager.filterExistingItems("/base/", [
      item("Cam/clip.mov", 500),
    ]);
    expect(skipped).toBe(1);
    expect(renamed).toBe(0);
    expect(items).toHaveLength(0);
  });

  it("gives two different items of a batch different numbered names", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["clip.mov", { size: 100, type: "file" }]])
    );
    const { items } = await uploadManager.filterExistingItems("/base/", [
      item("Cam/clip.mov", 500),
      item("Cam/clip.mov", 600),
    ]);
    expect(items.map((i) => i.relativePath)).toEqual(["Cam/clip_01.mov", "Cam/clip_02.mov"]);
  });

  it("add() with skipExisting queues only the missing/incomplete files with the right overwrite flags", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([
        ["done.mov", { size: 100, type: "file" }],
        ["empty.mov", { size: 0, type: "file" }],
      ])
    );
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();

    await uploadManager.add(
      "/base/",
      [item("Cam/done.mov", 100), item("Cam/empty.mov", 500), item("Cam/new.mov", 300)],
      false,
      true
    );

    const files = uploadManager.queue.filter((u) => u.type !== "directory");
    expect(files.map((u) => [u.name, u.overwrite])).toEqual([
      ["empty.mov", true],
      ["new.mov", false],
    ]);
    const dirs = uploadManager.queue.filter((u) => u.type === "directory");
    expect(dirs.map((u) => [u.path, u.overwrite])).toEqual([["/base/Cam/", true]]);
    expect(notify.showSuccessToast).toHaveBeenCalledWith("prompts.uploadSkipped:1");
  });

  it("add() queues a different-size file under a numbered destination path and name", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["big.mov", { size: 4096, type: "file" }]])
    );
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();

    await uploadManager.add("/base/", [item("Cam/big.mov", 900000)], false, true);

    const [upload] = uploadManager.queue.filter((u) => u.type !== "directory");
    expect(upload.path).toBe("/base/Cam/big_01.mov");
    expect(upload.name).toBe("big_01.mov");
    expect(upload.overwrite).toBe(false);
    expect(notify.showSuccessToast).toHaveBeenCalledWith("prompts.uploadRenamed:1");
  });

  it("add() with skipExisting queues nothing when everything is already uploaded", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["done.mov", { size: 100, type: "file" }]])
    );
    await uploadManager.add("/base/", [item("Cam/done.mov", 100)], false, true);
    expect(uploadManager.queue).toHaveLength(0);
    expect(notify.showSuccessToast).toHaveBeenCalledWith("prompts.uploadSkipped:1");
  });
});

describe("conflicts on loose files", () => {
  const looseItem = (name, size) => ({ file: { name, size }, relativePath: name });

  afterEach(() => {
    state.req.items = [];
    uploadManager.setOnConflict(() => {});
    uploadManager.overwriteAll = null;
  });

  it("asks what to do when a loose file already exists in the current folder, without offering rename", async () => {
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 100 }];
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);

    await uploadManager.add("/base/", [looseItem("done.mov", 100), looseItem("new.mov", 5)]);

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(onConflict.mock.calls[0][1]).toEqual({ allowRename: false });
    expect(uploadManager.queue).toHaveLength(0);
  });

  it("uploads only the missing files after choosing to skip existing ones", async () => {
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 4096 }];
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["done.mov", { size: 4096, type: "video/quicktime" }]])
    );
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
    uploadManager.setOnConflict((resolve) => resolve({ skip: true }));

    await uploadManager.add("/base/", [looseItem("done.mov", 100), looseItem("new.mov", 5)]);
    await vi.waitFor(() => expect(uploadManager.queue).toHaveLength(1));

    expect(uploadManager.queue[0].name).toBe("new.mov");
    expect(uploadManager.queue[0].overwrite).toBe(false);
    expect(notify.showSuccessToast).toHaveBeenCalledWith("prompts.uploadSkipped:1");
  });

  it("does not ask when no loose file exists or the destination is not the current folder", async () => {
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);

    state.req.items = [{ name: "other.mov", type: "video/quicktime", size: 1 }];
    await uploadManager.add("/base/", [looseItem("new.mov", 5)]);
    state.req.items = [{ name: "new.mov", type: "video/quicktime", size: 1 }];
    await uploadManager.add("/elsewhere/", [looseItem("new.mov", 5)]);

    expect(onConflict).not.toHaveBeenCalled();
    expect(uploadManager.queue).toHaveLength(2);
  });
});
