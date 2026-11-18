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
  getters: { isShare: vi.fn(() => false) },
}));
vi.mock("@/utils/appNotifications", async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...actual,
    notifyUploadComplete: vi.fn(),
    notifyUploadError: vi.fn(),
  };
});

import { resourcesApi } from "@/api/index.js";
import { notify } from "@/notify/index.ts";
import { mutations, state, getters } from "@/store/index.ts";
import { isSameSize, numberedName, uploadManager } from "./upload.js";

function resetConflictState() {
  uploadManager.conflictPromptOpen = false;
  uploadManager.conflictsCancelled = false;
  uploadManager.pendingConflictItems = null;
  uploadManager.pendingItems = null;
  uploadManager.conflictingFolder = null;
  uploadManager.overwriteAll = null;
  uploadManager.skipAll = false;
  uploadManager.answerSubmission = null;
  uploadManager.cancelledSubmission = null;
  uploadManager.isPausedForConflict = false;
  uploadManager.isOverallPaused = false;
  uploadManager.submissionId = 0;
}

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
    submission: uploadManager.submissionId,
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
  uploadManager.conflictPromptOpen = false;
  uploadManager.conflictsCancelled = false;
  uploadManager.pendingConflictItems = null;
  uploadManager.pendingItems = null;
  uploadManager.conflictingFolder = null;
  uploadManager.overwriteAll = null;
  uploadManager.skipAll = false;
  uploadManager.answerSubmission = null;
  uploadManager.cancelledSubmission = null;
  uploadManager.submissionId = 0;
  uploadManager.isPausedForConflict = false;
});

afterEach(() => {
  for (const id of uploadManager.progressTimeouts.keys()) {
    uploadManager.clearProgressTimeout(id);
  }
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("directory upload ordering", () => {
  it("waits for each directory before starting its children", async () => {
    // A populated listing avoids the separate top-level directory probe.
    state.req.items = [{ name: "unrelated.txt" }];
    const parent = deferred();
    resourcesApi.post.mockReturnValueOnce(parent.promise).mockResolvedValue("");

    await uploadManager.add("/base/", [
      { file: new File(["a"], "a.txt"), relativePath: "folder/child/a.txt" },
    ]);
    expect(resourcesApi.post.mock.calls.map((call) => call[1])).toEqual(["/base/folder/"]);

    parent.resolve("");
    await vi.advanceTimersByTimeAsync(0);
    expect(resourcesApi.post.mock.calls.map((call) => call[1])).toEqual([
      "/base/folder/", "/base/folder/child/", "/base/folder/child/a.txt",
    ]);
    state.req.items = [];
  });
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

  it("offers rename when a single loose file already exists in the current folder", async () => {
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 100 }];
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);

    await uploadManager.add("/base/", [looseItem("done.mov", 100), looseItem("new.mov", 5)]);

    expect(onConflict).toHaveBeenCalledTimes(1);
    // One file in the way: the user may give the upload another name, as for a folder.
    expect(onConflict.mock.calls[0][1]).toEqual({ allowRename: true });
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

describe("hasOngoingUploads", () => {
  it("treats paused uploads as in progress", () => {
    addUpload("paused");
    uploadManager.activeUploads = 0;
    expect(uploadManager.hasOngoingUploads()).toBe(true);
  });

  it("is false when the queue has only finished uploads", () => {
    addUpload("completed");
    uploadManager.activeUploads = 0;
    expect(uploadManager.hasOngoingUploads()).toBe(false);
  });

  it("keeps isUploading true while uploads are paused", async () => {
    addUpload("paused");
    uploadManager.activeUploads = 0;
    await uploadManager.processQueue();
    expect(mutations.setIsUploading).toHaveBeenCalledWith(true);
  });

  it("clears isUploading when the last paused upload is cancelled", () => {
    const item = addUpload("paused");
    uploadManager.activeUploads = 0;
    mutations.setIsUploading.mockClear();
    uploadManager.cancel(item.id);
    expect(mutations.setIsUploading).toHaveBeenCalledWith(false);
  });

  it("clears isUploading when clearAll removes the last paused upload", () => {
    state.user.fileLoading.clearAll = true;
    addUpload("paused");
    uploadManager.activeUploads = 0;
    mutations.setIsUploading.mockClear();
    uploadManager.clearCompleted();
    expect(mutations.setIsUploading).toHaveBeenCalledWith(false);
    state.user.fileLoading.clearAll = false;
  });
});

describe("upload waiting for the server to save the data", () => {
  it("does not pause while the server answers after the whole body was sent", async () => {
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    // the body is sent in full: no more progress events until the server answers
    await vi.advanceTimersByTimeAsync(2000);
    uploadManager.markSentToServer(upload);

    await vi.advanceTimersByTimeAsync(uploadManager.PROGRESS_TIMEOUT_MS * 3);
    expect(upload.status).toBe("uploading");
    expect(upload.awaitingServer).toBe(true);
  });

  it("still pauses if the server never answers, after the longer limit", async () => {
    const upload = addUpload();
    uploadManager.markSentToServer(upload);
    await vi.advanceTimersByTimeAsync(uploadManager.SERVER_RESPONSE_TIMEOUT_MS - 1);
    expect(upload.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(upload.status).toBe("paused");
    expect(upload.awaitingServer).toBe(false);
    expect(upload.connectionIssue).toBe(true);
  });

  it("goes back to the normal limit once the server has answered", async () => {
    const upload = addUpload();
    uploadManager.markSentToServer(upload);
    await vi.advanceTimersByTimeAsync(30000);
    // the server answered: the chunk loop clears the flag and restarts the normal timer
    upload.awaitingServer = false;
    uploadManager.recordUploadActivity(upload);
    uploadManager.startProgressTimeout(upload);
    await vi.advanceTimersByTimeAsync(uploadManager.PROGRESS_TIMEOUT_MS - 1);
    expect(upload.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(upload.status).toBe("paused");
  });

  it("ignores a late notification for an upload that is not uploading", () => {
    const upload = addUpload("paused");
    uploadManager.markSentToServer(upload);
    expect(upload.awaitingServer).toBeUndefined();
    expect(uploadManager.progressTimeouts.size).toBe(0);
  });

  it("sends the notification when the request body has been sent", async () => {
    const instances = [];
    const OriginalXhr = globalThis.XMLHttpRequest;
    globalThis.XMLHttpRequest = class {
      constructor() {
        this.upload = {};
        instances.push(this);
      }
      open() {}
      setRequestHeader() {}
      send() {}
    };
    try {
      const { post } = await import("@/api/resources.js");
      const onsent = vi.fn();
      void post("test", "/f", new Blob(["a"]), false, undefined, {}, false, onsent);
      expect(instances).toHaveLength(1);
      expect(onsent).not.toHaveBeenCalled();
      instances[0].upload.onload();
      expect(onsent).toHaveBeenCalledTimes(1);
    } finally {
      globalThis.XMLHttpRequest = OriginalXhr;
    }
  });
});

describe("upload stall timeout setting", () => {
  afterEach(() => {
    delete state.user.fileLoading.uploadStallTimeoutSeconds;
  });

  it.each([undefined, 0, -5])("uses the default limit when the setting is %s", (value) => {
    state.user.fileLoading.uploadStallTimeoutSeconds = value;
    expect(uploadManager.progressTimeoutMs()).toBe(uploadManager.PROGRESS_TIMEOUT_MS);
  });

  it("pauses after the number of seconds the user chose", async () => {
    state.user.fileLoading.uploadStallTimeoutSeconds = 45;
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    await vi.advanceTimersByTimeAsync(44999);
    expect(upload.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(upload.status).toBe("paused");
  });

  it("applies a shorter limit as well", async () => {
    state.user.fileLoading.uploadStallTimeoutSeconds = 3;
    const upload = addUpload();
    uploadManager.startProgressTimeout(upload);
    await vi.advanceTimersByTimeAsync(2999);
    expect(upload.status).toBe("uploading");
    await vi.advanceTimersByTimeAsync(1);
    expect(upload.status).toBe("paused");
  });

  it("keeps the longer limit while the server saves, and honors a setting above it", () => {
    const upload = addUpload();
    upload.awaitingServer = true;
    state.user.fileLoading.uploadStallTimeoutSeconds = 30;
    expect(uploadManager.stallLimitMs(upload)).toBe(uploadManager.SERVER_RESPONSE_TIMEOUT_MS);
    state.user.fileLoading.uploadStallTimeoutSeconds = 1800;
    expect(uploadManager.stallLimitMs(upload)).toBe(1800000);
  });
});

describe("conflicts found while transferring", () => {
  beforeEach(() => {
    uploadManager.conflictPromptOpen = false;
    uploadManager.conflictsCancelled = false;
    uploadManager.pendingConflictItems = null;
    uploadManager.overwriteAll = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    uploadManager.isPausedForConflict = false;
    uploadManager.isOverallPaused = false;
    uploadManager.conflictPromptOpen = false;
    uploadManager.submissionId = 0;
    uploadManager.pendingItems = null;
    uploadManager.conflictingFolder = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    uploadManager.overwriteAll = null;
  });

  it("asks once for every conflicting file instead of once per file", () => {
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    addUpload("conflict");
    addUpload("conflict");

    uploadManager.askConflictForBatch();

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(onConflict.mock.calls[0][1]).toEqual({ allowRename: false });
    expect(uploadManager.isPausedForConflict).toBe(true);
  });

  it("replaces every conflicting file after choosing replace all", () => {
    uploadManager.setOnConflict((resolve) => resolve(true));
    const first = addUpload("conflict");
    const second = addUpload("conflict");

    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(true);

    expect(first.status).toBe("pending");
    expect(second.status).toBe("pending");
    expect(first.overwrite).toBe(true);
    expect(second.overwrite).toBe(true);
    expect(uploadManager.isPausedForConflict).toBe(false);
  });

  it("drops only the conflicting files after choosing skip all", () => {
    addUpload("conflict");
    const done = addUpload("completed");

    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch({ skip: true });

    expect(uploadManager.queue).toEqual([done]);
    expect(notify.showSuccessToast).toHaveBeenCalledWith("prompts.uploadSkipped:1");
  });

  it("stops the queue when the conflict question is cancelled", () => {
    const upload = addUpload("conflict");

    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(false);

    // The batch is left as it is; a new submission may start again.
    expect(uploadManager.isOverallPaused).toBe(false);
    expect(uploadManager.cancelledSubmission).toBe(upload.submission);
    expect(uploadManager.queue).toEqual([upload]);
  });

  it("treats a dropped connection as a conflict when the file is already on the server", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["file", { size: 1, type: "file" }]])
    );
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    const upload = addUpload("uploading");

    await uploadManager.handleUploadError(upload, new Error("Network error"));

    expect(upload.status).toBe("conflict");
    expect(onConflict).toHaveBeenCalledTimes(1);
  });

  it("still offers to resume a stalled transfer when the file is not on the server", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(new Map());
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    const upload = addUpload("uploading");
    upload.size = 10 * 1024 * 1024;
    upload.chunkOffset = 5 * 1024 * 1024;

    await uploadManager.handleUploadError(upload, new Error("Network error"));

    expect(upload.status).toBe("paused");
    expect(onConflict).not.toHaveBeenCalled();
  });
});

describe("conflict answers cover the rest of the batch", () => {
  beforeEach(() => {
    uploadManager.conflictPromptOpen = false;
    uploadManager.conflictsCancelled = false;
    uploadManager.pendingConflictItems = null;
    uploadManager.overwriteAll = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    uploadManager.isOverallPaused = false;
    uploadManager.conflictPromptOpen = false;
    uploadManager.submissionId = 0;
    uploadManager.pendingItems = null;
    uploadManager.conflictingFolder = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    uploadManager.overwriteAll = null;
  });

  it("does not ask again when replace all was chosen and another file conflicts later", () => {
    resetConflictState();
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(true);

    const later = addUpload("conflict");
    uploadManager.askConflictForBatch();

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(later.status).toBe("pending");
    expect(later.overwrite).toBe(true);
  });

  it("drops later conflicts without asking when skip all was chosen", () => {
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch({ skip: true });

    addUpload("conflict");
    uploadManager.askConflictForBatch();

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(uploadManager.queue).toHaveLength(0);
  });

  it("stops asking after the batch is cancelled", () => {
    resetConflictState();
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    const first = addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(false);

    addUpload("conflict");
    uploadManager.askConflictForBatch();

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(uploadManager.cancelledSubmission).toBe(first.submission);
  });
});

describe("files that conflict while the question is open", () => {
  beforeEach(() => {
    uploadManager.conflictPromptOpen = false;
    uploadManager.conflictsCancelled = false;
    uploadManager.pendingConflictItems = null;
    uploadManager.overwriteAll = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    uploadManager.isPausedForConflict = false;
    uploadManager.isOverallPaused = false;
    uploadManager.conflictPromptOpen = false;
    uploadManager.submissionId = 0;
    uploadManager.pendingItems = null;
    uploadManager.conflictingFolder = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    uploadManager.overwriteAll = null;
  });

  it("retries them with the answer already given instead of leaving them stuck", () => {
    resetConflictState();
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    const late = addUpload("conflict");
    uploadManager.resolveConflictForBatch(true);
    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(late.status).toBe("pending");
    expect(late.overwrite).toBe(true);
  });

  it("drops them when skip all was chosen", () => {
    uploadManager.setOnConflict(() => {});
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    addUpload("conflict");
    uploadManager.resolveConflictForBatch({ skip: true });
    expect(uploadManager.queue).toHaveLength(0);
  });
});

describe("the conflict question names what conflicts", () => {
  beforeEach(() => {
    uploadManager.conflictingFolder = null;
    uploadManager.pendingConflictItems = null;
  });

  it("names the folder when a folder of the same name is in the way", () => {
    uploadManager.conflictingFolder = "upload-test-10";
    expect(uploadManager.getConflictDetails()).toEqual({
      kind: "folder",
      count: 1,
      names: ["upload-test-10"],
    });
  });

  it("lists the files that conflict, with the count", () => {
    uploadManager.pendingConflictItems = [addUpload("conflict"), addUpload("conflict")];
    const details = uploadManager.getConflictDetails();
    expect(details.kind).toBe("files");
    expect(details.count).toBe(2);
    expect(details.names).toHaveLength(2);
  });
});

describe("naming the conflicting files before the upload starts", () => {
  afterEach(() => {
    state.req.items = [];
    uploadManager.setOnConflict(() => {});
    uploadManager.pendingConflictItems = null;
  });

  it("lists the loose files that are already in the folder", async () => {
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 100 }];
    uploadManager.setOnConflict(() => {});
    await uploadManager.add("/base/", [
      { file: { name: "done.mov", size: 100 }, relativePath: "done.mov" },
      { file: { name: "new.mov", size: 5 }, relativePath: "new.mov" },
    ]);
    expect(uploadManager.getConflictDetails().names).toEqual(["done.mov"]);
  });
});

describe("renaming the incoming folder", () => {
  it("moves the queued upload to the new name and leaves the existing folder alone", async () => {
    uploadManager.pendingItems = [
      { file: { name: "a.txt", size: 1 }, relativePath: "upload-test-10/a.txt" },
      { file: { name: "b.txt", size: 1 }, relativePath: "upload-test-10/b.txt" },
    ];
    await uploadManager.renameFolder("upload-test-10", "upload-test-101");
    expect(uploadManager.pendingItems.map((item) => item.relativePath)).toEqual([
      "upload-test-101/a.txt",
      "upload-test-101/b.txt",
    ]);
  });
});

describe("renaming a single incoming file", () => {
  beforeEach(() => {
    uploadManager.pendingItems = null;
    uploadManager.pendingConflictItems = null;
    uploadManager.conflictingFolder = null;
  });

  it("sends the file under the new name and leaves the existing one alone", () => {
    uploadManager.pendingItems = [
      { file: { name: "testas-01.txt", size: 1 }, relativePath: "testas-01.txt" },
    ];
    uploadManager.renamePendingFile("testas-01.txt", "testas-01-kopija.txt");
    expect(uploadManager.pendingItems[0].relativePath).toBe("testas-01-kopija.txt");
  });

  it("offers rename only when a single item is in the way", () => {
    uploadManager.pendingConflictItems = [addUpload("conflict")];
    expect(uploadManager.conflictAllowsRename()).toBe(true);
    uploadManager.pendingConflictItems = [addUpload("conflict"), addUpload("conflict")];
    expect(uploadManager.conflictAllowsRename()).toBe(false);
  });
});

describe("after renaming the incoming file", () => {
  afterEach(() => {
    state.req.items = [];
    uploadManager.setOnConflict(() => {});
    uploadManager.pendingItems = null;
    uploadManager.queue.splice(0);
  });

  it("does not treat the renamed file as a conflict again", async () => {
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 100 }];
    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
    uploadManager.pendingItems = [{ file: { name: "done.mov", size: 100 }, relativePath: "done.mov" }];

    uploadManager.renamePendingFile("done.mov", "done-kopija.mov");
    await uploadManager.add("/base/", uploadManager.pendingItems, false);

    expect(onConflict).not.toHaveBeenCalled();
    const queued = uploadManager.queue.find((item) => item.path?.includes("done-kopija.mov"));
    expect(queued?.path).toBe("/base/done-kopija.mov");
    expect(queued?.name).toBe("done-kopija.mov");
  });
});

describe("review follow-ups", () => {
  beforeEach(() => {
    uploadManager.pendingItems = null;
    uploadManager.pendingConflictItems = null;
    uploadManager.conflictingFolder = null;
    uploadManager.overwriteAll = null;
    uploadManager.skipAll = false;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    uploadManager.isOverallPaused = false;
    uploadManager.conflictPromptOpen = false;
    uploadManager.submissionId = 0;
    uploadManager.pendingItems = null;
    uploadManager.conflictingFolder = null;
    uploadManager.answerSubmission = null;
    uploadManager.cancelledSubmission = null;
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    uploadManager.overwriteAll = null;
    uploadManager.skipAll = false;
  });

  it("stops sending a file that keeps conflicting", () => {
    const item = addUpload("conflict");
    for (let round = 0; round < 3; round += 1) {
      uploadManager.retryConflicts([item]);
      item.status = "conflict";
    }
    uploadManager.retryConflicts([item]);
    expect(item.status).toBe("error");
  });

  it("renames a queued job instead of the file on the server", () => {
    const item = addUpload("conflict");
    item.path = "/base/old.txt";
    uploadManager.renameQueuedUpload(item, "new.txt");
    expect(item.path).toBe("/base/new.txt");
    expect(item.name).toBe("new.txt");
    expect(item.overwrite).toBe(false);
    expect(item.status).toBe("pending");
  });

  it("does not treat a dropped replacement as a conflict", async () => {
    resourcesApi.listDirectoryEntries.mockResolvedValue(
      new Map([["file", { size: 1, type: "file" }]])
    );
    const item = addUpload("uploading");
    item.overwrite = true;
    expect(await uploadManager.targetExists(item)).toBe(false);
  });

  it("keeps the skip answer for later conflicts in the batch", () => {
    resetConflictState();
    uploadManager.setOnConflict(() => {});
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch({ skip: true });

    addUpload("conflict");
    uploadManager.askConflictForBatch();

    expect(uploadManager.queue).toHaveLength(0);
  });
});

describe("second review round", () => {
  beforeEach(() => {
    uploadManager.pendingItems = null;
    uploadManager.pendingConflictItems = null;
    uploadManager.overwriteAll = null;
    uploadManager.skipAll = false;
    uploadManager.continuingBatch = false;
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    state.req.items = [];
    uploadManager.setOnConflict(() => {});
    uploadManager.skipAll = false;
    uploadManager.continuingBatch = false;
  });

  it("renames a queued directory together with everything inside it", () => {
    const dir = addUpload("conflict");
    dir.type = "directory";
    dir.path = "/base/docs/";
    const child = addUpload("pending");
    child.path = "/base/docs/a.txt";
    const other = addUpload("pending");
    other.path = "/base/other.txt";

    uploadManager.renameQueuedUpload(dir, "docs-copy");

    expect(dir.path).toBe("/base/docs-copy/");
    expect(child.path).toBe("/base/docs-copy/a.txt");
    expect(other.path).toBe("/base/other.txt");
  });

  it("keeps the skip answer while the same batch continues", async () => {
    uploadManager.skipAll = true;
    uploadManager.continuingBatch = true;
    await uploadManager.add("/base/", [
      { file: { name: "new.mov", size: 5 }, relativePath: "new.mov" },
    ]);
    expect(uploadManager.skipAll).toBe(true);
  });
});

describe("directory renames stay inside their submission", () => {
  it("moves only the jobs of the renamed submission", () => {
    const dir = addUpload("conflict");
    Object.assign(dir, { type: "directory", path: "/base/docs/", submission: 1, source: "shared" });
    const own = addUpload("pending");
    Object.assign(own, { path: "/base/docs/a.txt", submission: 1, source: "shared" });
    const foreign = addUpload("pending");
    Object.assign(foreign, { path: "/base/docs/b.txt", submission: 2, source: "shared" });
    const otherSource = addUpload("pending");
    Object.assign(otherSource, { path: "/base/docs/c.txt", submission: 1, source: "other" });

    uploadManager.renameQueuedUpload(dir, "docs-copy");

    expect(own.path).toBe("/base/docs-copy/a.txt");
    expect(foreign.path).toBe("/base/docs/b.txt");
    expect(otherSource.path).toBe("/base/docs/c.txt");
  });

  it("restarts a paused job under the new name instead of resuming the old one", () => {
    const dir = addUpload("conflict");
    Object.assign(dir, { type: "directory", path: "/base/docs/", submission: 1, source: "shared" });
    const paused = addUpload("paused");
    Object.assign(paused, {
      path: "/base/docs/big.bin",
      submission: 1,
      source: "shared",
      chunkOffset: 5000000,
      progress: 40,
      sessionId: "old-session",
    });

    uploadManager.renameQueuedUpload(dir, "docs-copy");

    expect(paused.path).toBe("/base/docs-copy/big.bin");
    expect(paused.chunkOffset).toBe(0);
    expect(paused.progress).toBe(0);
    expect(paused.sessionId).not.toBe("old-session");
    expect(paused.status).toBe("pending");
  });
});

describe("submission identity", () => {
  beforeEach(() => {
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    state.req.items = [];
  });

  it("gives a second submission its own id even while the first is still queued", async () => {
    await uploadManager.add("/base/", [{ file: { name: "a.txt", size: 1 }, relativePath: "a.txt" }]);
    const first = uploadManager.queue.at(-1).submission;
    await uploadManager.add("/base/", [{ file: { name: "b.txt", size: 1 }, relativePath: "b.txt" }]);
    const second = uploadManager.queue.at(-1).submission;
    expect(first).toBeDefined();
    expect(second).toBeGreaterThan(first);
  });

  it("keeps the id when the same batch continues after an answer", async () => {
    await uploadManager.add("/base/", [{ file: { name: "a.txt", size: 1 }, relativePath: "a.txt" }]);
    const first = uploadManager.queue.at(-1).submission;
    uploadManager.continuingBatch = true;
    await uploadManager.add("/base/", [{ file: { name: "a.txt", size: 1 }, relativePath: "a.txt" }]);
    expect(uploadManager.queue.at(-1).submission).toBe(first);
  });
});

describe("answers never cross submissions", () => {
  beforeEach(() => {
    resetConflictState();
    vi.spyOn(uploadManager, "processQueue").mockResolvedValue();
  });

  afterEach(() => {
    uploadManager.setOnConflict(() => {});
    state.req.items = [];
  });

  it("asks a second submission its own question instead of applying the first answer", () => {
    const first = addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(true);

    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    const foreign = addUpload("conflict");
    foreign.submission = first.submission + 1;
    uploadManager.askConflictForBatch();

    expect(onConflict).toHaveBeenCalledTimes(1);
    expect(foreign.status).toBe("conflict");
    expect(foreign.overwrite).not.toBe(true);
  });

  it("does not let an earlier replace answer overwrite a new submission's file", async () => {
    addUpload("conflict");
    uploadManager.askConflictForBatch();
    uploadManager.resolveConflictForBatch(true);

    const onConflict = vi.fn();
    uploadManager.setOnConflict(onConflict);
    state.req.items = [{ name: "done.mov", type: "video/quicktime", size: 100 }];
    await uploadManager.add("/base/", [
      { file: { name: "done.mov", size: 100 }, relativePath: "done.mov" },
    ]);

    // The new submission is asked, and none of its jobs may overwrite by default.
    expect(onConflict).toHaveBeenCalledTimes(1);
    const ownJobs = uploadManager.queue.filter(
      (item) => item.submission === uploadManager.submissionId
    );
    expect(ownJobs.every((item) => item.overwrite !== true)).toBe(true);
  });
});
