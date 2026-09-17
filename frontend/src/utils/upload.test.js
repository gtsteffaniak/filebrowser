import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api", () => ({
  resourcesApi: {
    post: vi.fn(),
    postPublic: vi.fn(),
    signalUploadPause: vi.fn(),
  },
}));
vi.mock("@/store", () => ({
  state: {
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
import { getters } from "@/store/getters";
import { uploadManager } from "./upload";

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
