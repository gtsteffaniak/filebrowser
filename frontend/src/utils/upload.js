import { reactive } from "vue";
import { filesApi } from "@/api";
import { state as storeState } from "@/store";

const MAX_CONCURRENT_UPLOADS = 10;

class UploadManager {
  constructor() {
    this.queue = reactive([]);
    this.activeUploads = 0;
    this.nextId = 0;
    this.overwriteAll = null; // null: ask, true: overwrite, false: skip
    this.isPausedForConflict = false;
    this.onConflict = () => {}; // Callback for UI
  }

  setOnConflict(handler) {
    this.onConflict = handler;
  }

  async add(basePath, items, overwrite = false) {
    console.log(`upload.js: uploadManager.add called.`);
    console.log(` - basePath: ${basePath}`);
    console.log(` - items:`, items);
    console.log(` - overwrite: ${overwrite}`);
    if (basePath.slice(-1) !== "/") {
      basePath += "/";
    }
    const dirs = new Set();
    for (const item of items) {
      if (item.relativePath) {
        const pathParts = item.relativePath.split("/");
        pathParts.pop(); // Grab the directory path by removing the filename.

        let currentPath = "";
        for (const part of pathParts) {
          currentPath += part + "/";
          dirs.add(currentPath);
        }
      }
    }
    console.log("upload.js: Directories to create:", dirs);

    const newUploads = [];

    if (dirs.size > 0) {
      // Sort paths to ensure parent directories are created before children.
      const sortedDirs = [...dirs].sort();
      console.log("upload.js: Sorted directories:", sortedDirs);

      for (const dir of sortedDirs) {
        const pathParts = dir.slice(0, -1).split("/");
        const dirName = pathParts[pathParts.length - 1];

        const upload = {
          id: this.nextId++,
          name: dirName,
          size: 0,
          progress: 0,
          status: "pending",
          type: "directory",
          path: `${basePath}${dir}`,
          source: storeState.req.source,
          overwrite: false, // Initially false, will be updated by conflict handler
        };

        newUploads.push(upload);
        console.log("upload.js: Created directory upload object:", upload);
      }
    }

    const fileUploads = Array.from(items).map((item) => {
      const id = this.nextId++;
      const file = item.file;
      const relativePath = item.relativePath || file.name;
      let destinationPath = `${basePath}${relativePath}`;
      const upload = {
        id,
        file,
        name: file.name,
        size: file.size,
        progress: 0,
        chunkOffset: 0,
        status: "pending", // pending, uploading, paused, completed, error
        xhr: null,
        path: destinationPath, // Full destination path
        source: storeState.req.source,
        overwrite: false, // Initially false, will be updated by conflict handler
      };
      console.log("upload.js: Created upload object:", upload);
      return upload;
    });

    this.queue.push(...newUploads, ...fileUploads);

    this.processQueue();
    return newUploads;
  }

  async processQueue() {
    console.log("upload.js: processQueue called.");
    console.log(` - activeUploads: ${this.activeUploads}`);
    console.log(` - hasPending: ${this.hasPending()}`);
    console.log(` - isPausedForConflict: ${this.isPausedForConflict}`);

    if (this.isPausedForConflict) {
      console.log("upload.js: Queue is paused, waiting for conflict resolution.");
      return;
    }

    while (
      this.activeUploads < MAX_CONCURRENT_UPLOADS &&
      this.hasPending()
    ) {
      const upload = this.queue.find((item) => item.status === "pending");
      if (upload) {
        console.log("upload.js: Found pending upload to start:", upload);
        if (this.overwriteAll) {
          upload.overwrite = true;
        }
        this.start(upload.id);
      }
    }
  }

  start(id) {
    const upload = this.findById(id);
    if (!upload || upload.status !== "pending") {
      console.log(
        `upload.js: Cannot start upload for id ${id}. Status is not 'pending' or upload not found.`,
        upload
      );
      return;
    }
    console.log(`upload.js: Starting upload for id ${id}`, upload);

    if (upload.type === "directory") {
      this.startDirectoryUpload(upload);
    } else {
      this.startFileUpload(upload);
    }
  }

  async startDirectoryUpload(upload) {
    this.activeUploads++;
    upload.status = "uploading";

    try {
      const { promise } = filesApi.post(
        upload.source,
        upload.path,
        new Blob([]),
        upload.overwrite
      );
      await promise;

      console.log(`upload.js: Directory upload completed for id ${upload.id}`, upload);
      upload.status = "completed";
      upload.progress = 100;
    } catch (err) {
      await this.handleUploadError(upload, err);
    } finally {
      console.log(`upload.js: Directory upload finished (finally block) for id ${upload.id}`, upload);
      this.activeUploads--;
      this.processQueue();
    }
  }

  async startFileUpload(upload) {
    this.activeUploads++;
    upload.status = "uploading";

    const CHUNK_SIZE = 10 * 1024 * 1024; // 10MB

    while (upload.chunkOffset < upload.size && upload.status === "uploading") {
      const chunk = upload.file.slice(
        upload.chunkOffset,
        upload.chunkOffset + CHUNK_SIZE
      );

      const chunkProgress = (percent) => {
        const chunkLoaded = (percent / 100) * chunk.size;
        const totalLoaded = upload.chunkOffset + chunkLoaded;
        const progress = (totalLoaded / upload.size) * 100;
        upload.progress = Math.round(progress * 10) / 10;
      };

      try {
        const { xhr, promise } = filesApi.post(
          upload.source,
          upload.path,
          chunk,
          upload.overwrite,
          chunkProgress,
          {
            "X-File-Chunk-Offset": upload.chunkOffset,
            "X-File-Total-Size": upload.size,
          }
        );

        upload.xhr = xhr;
        await promise;

        upload.chunkOffset += chunk.size;
      } catch (err) {
        await this.handleUploadError(upload, err);
        break; // Exit loop on error or pause
      }
    }

    if (upload.status === "uploading") {
      // If the loop finished without being paused/errored
      upload.status = "completed";
      upload.progress = 100;
    }

    this.activeUploads--;
    upload.xhr = null;
    this.processQueue();
  }

  pauseAll() {
    console.log(`upload.js: pauseAll called`);
    this.queue.forEach((upload) => {
      if (upload.status === "uploading") {
        this.pause(upload.id);
      }
    });
  }

  resumeAll() {
    console.log(`upload.js: resumeAll called`);
    this.queue.forEach((upload) => {
      if (upload.status === "paused") {
        this.resume(upload.id);
      }
    });
  }

  pause(id) {
    console.log(`upload.js: pause called for id ${id}`);
    const upload = this.findById(id);
    if (upload && upload.status === "uploading" && upload.xhr) {
      upload.xhr.abort();
      upload.status = "paused";
    }
  }

  resume(id) {
    console.log(`upload.js: resume called for id ${id}`);
    const upload = this.findById(id);
    if (upload && upload.status === "paused") {
      upload.status = "pending";
      const progress =
        upload.size > 0 ? (upload.chunkOffset / upload.size) * 100 : 0;
      upload.progress = Math.round(progress * 10) / 10;
      this.processQueue();
    }
  }

  cancel(id) {
    console.log(`upload.js: cancel called for id ${id}`);
    this.pause(id); // Abort if in progress
    const index = this.queue.findIndex((item) => item.id === id);
    if (index !== -1) {
      this.queue.splice(index, 1);
    }
  }

  retry(id) {
    console.log(`upload.js: retry called for id ${id}`);
    const upload = this.findById(id);
    if (upload && ["error", "conflict"].includes(upload.status)) {
      upload.status = "pending";
      upload.chunkOffset = 0; // Reset chunk offset for retries
      upload.progress = 0;
      this.processQueue();
    }
  }

  clearCompleted() {
    console.log("upload.js: clearCompleted called");
    for (let i = this.queue.length - 1; i >= 0; i--) {
      if (this.queue[i].status === "completed") {
        this.queue.splice(i, 1);
      }
    }
  }

  findById(id) {
    return this.queue.find((item) => item.id === id);
  }

  hasPending() {
    return this.queue.some((item) => item.status === "pending");
  }

  async handleUploadError(upload, err) {
    // Check if the error is a 409 Conflict
    if (err?.response?.status === 409) {
      console.log(`upload.js: Conflict detected by backend for id ${upload.id}`);
      upload.status = "conflict";

      // Pause the queue if this is the first conflict of the batch
      if (this.overwriteAll === null) {
        this.isPausedForConflict = true;
        this.overwriteAll = "pending"; // Mark that we are waiting for user input
        this.onConflict((resolution) => {
          this.resolveConflict(resolution);
        });
      }
    } else if (err.message !== "Upload aborted") {
      upload.status = "error";
      console.error(`upload.js: Upload error for id ${upload.id}:`, err, upload);
    } else {
      console.log(`upload.js: Upload aborted for id ${upload.id}`, upload);
    }
  }

  resolveConflict(overwrite) {
    console.log(`upload.js: Resolving conflict with strategy: ${overwrite ? 'OVERWRITE' : 'CANCEL'}`);
    this.overwriteAll = overwrite;
    this.isPausedForConflict = false;

    if (overwrite) {
      // Find all items that hit a conflict and requeue them.
      for (const item of this.queue) {
        if (item.status === "conflict") {
          item.status = "pending";
          item.overwrite = true;
          if (item.type !== 'directory') {
            item.chunkOffset = 0; // Reset progress for resume
          }
        }
      }
    } else {
      // Cancel all uploads in the queue.
      for (let i = this.queue.length - 1; i >= 0; i--) {
        this.cancel(this.queue[i].id)
      }
    }

    this.processQueue();
  }
}

export const uploadManager = new UploadManager();

export function checkConflict(files, items) {
  if (typeof items === 'undefined' || items === null) {
    items = [];
  }

  let folder_upload = files[0].path !== undefined;

  let conflict = false;
  for (let i = 0; i < files.length; i++) {
    let file = files[i];
    let name = file.name;

    if (folder_upload) {
      let dirs = file.path.split('/');
      if (dirs.length > 1) {
        name = dirs[0];
      }
    }

    let res = items.findIndex(function hasConflict(element) {
      return element.name === this;
    }, name);

    if (res >= 0) {
      conflict = true;
      break;
    }
  }

  return conflict;
}
