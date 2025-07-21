import { reactive } from "vue";
import { filesApi } from "@/api";
import { state,mutations } from "@/store";

class UploadManager {
  constructor() {
    this.queue = reactive([]);
    this.activeUploads = 0;
    this.nextId = 0;
    this.overwriteAll = null; // null: ask, true: overwrite, false: skip
    this.isPausedForConflict = false;
    this.isOverallPaused = false;
    this.onConflict = () => {}; // Callback for UI
    this.hadActiveUploads = false; // Track if we've had active uploads
  }

  setOnConflict(handler) {
    this.onConflict = handler;
  }

  async add(basePath, items, overwrite = false) {

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

    const newUploads = [];

    if (dirs.size > 0) {
      // Sort paths to ensure parent directories are created before children.
      const sortedDirs = [...dirs].sort();

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
          source: state.req.source,
          overwrite: overwrite,
        };

        newUploads.push(upload);
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
        source: state.req.source,
        overwrite: overwrite,
      };
      return upload;
    });

    this.queue.push(...newUploads, ...fileUploads);

    this.processQueue();
    return newUploads;
  }

  async processQueue() {
    if (this.isPausedForConflict) {
      return;
    }

    if (this.isOverallPaused) {
      return;
    }

    while (
      this.activeUploads < state.user.fileLoading.maxConcurrentUpload &&
      this.hasPending()
    ) {
      const upload = this.queue.find((item) => item.status === "pending");
      if (upload) {
        if (this.overwriteAll) {
          upload.overwrite = true;
        }
        this.start(upload.id);
      }
    }

    // Only reload when we transition from having active uploads to having none
    const hasNoActiveOrPending = this.activeUploads === 0 && !this.hasPending();
    if (this.hadActiveUploads && hasNoActiveOrPending) {
      console.log("all uploads processed  ", this.queue);
      mutations.setReload(true);
      this.hadActiveUploads = false; // Reset the flag
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

    if (upload.type === "directory") {
      this.startDirectoryUpload(upload);
    } else {
      this.startFileUpload(upload);
    }
  }

  async startDirectoryUpload(upload) {
    this.activeUploads++;
    this.hadActiveUploads = true; // Mark that we've had active uploads
    upload.status = "uploading";

    try {
      const { promise } = filesApi.post(
        upload.source,
        upload.path,
        new Blob([]),
        upload.overwrite
      );
      await promise;

      upload.status = "completed";
      upload.progress = 100;
    } catch (err) {
      await this.handleUploadError(upload, err);
    } finally {
      this.activeUploads--;
      this.processQueue();
    }
  }

  async startFileUpload(upload) {
    this.activeUploads++;
    this.hadActiveUploads = true; // Mark that we've had active uploads
    upload.status = "uploading";

    const chunkSize = state.user.fileLoading.uploadChunkSizeMb * 1024 * 1024;
    if (chunkSize === 0) {
      const progress = (percent) => {
        upload.progress = percent;
      };

      try {
        const { xhr, promise } = filesApi.post(
          upload.source,
          upload.path,
          upload.file,
          upload.overwrite,
          progress,
          {
            "X-File-Total-Size": upload.size,
          }
        );

        upload.xhr = xhr;
        await promise;

        upload.status = "completed";
        upload.progress = 100;
      } catch (err) {
        await this.handleUploadError(upload, err);
      } finally {
        this.activeUploads--;
        upload.xhr = null;
        this.processQueue();
      }
      return;
    }

    while (upload.chunkOffset < upload.size && upload.status === "uploading") {
      const chunk = upload.file.slice(
        upload.chunkOffset,
        upload.chunkOffset + chunkSize
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
    this.isOverallPaused = true;
    this.queue.forEach((upload) => {
      if (upload.status === "uploading") {
        this.pause(upload.id);
      }
    });
  }

  resumeAll() {
    this.isOverallPaused = false;
    this.queue.forEach((upload) => {
      if (upload.status === "paused") {
        this.resume(upload.id);
      }
    });
  }

  pause(id) {
    const upload = this.findById(id);
    if (upload && upload.status === "uploading" && upload.xhr) {
      upload.xhr.abort();
      upload.status = "paused";
    }
  }

  resume(id) {
    const upload = this.findById(id);
    if (upload && upload.status === "paused") {
      this.isOverallPaused = false;
      upload.status = "pending";
      const progress =
        upload.size > 0 ? (upload.chunkOffset / upload.size) * 100 : 0;
      upload.progress = Math.round(progress * 10) / 10;
      this.processQueue();
    }
  }

  cancel(id) {
    this.pause(id); // Abort if in progress
    const index = this.queue.findIndex((item) => item.id === id);
    if (index !== -1) {
      this.queue.splice(index, 1);
    }
  }

  retry(id, overwrite = false) {
    const upload = this.findById(id);
    if (upload && ["error", "conflict"].includes(upload.status)) {
      upload.overwrite = overwrite;
      upload.status = "pending";
      if (upload.type !== 'directory') {
          upload.chunkOffset = 0; // Reset chunk offset for retries
      }
      upload.progress = 0;
      this.processQueue();
    }
  }

  clearCompleted() {
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
      upload.status = "conflict";
    } else if (err.message !== "Upload aborted") {
      upload.status = "error";
    } else {
      console.log(`upload.js: Upload aborted for id ${upload.id}`, upload);
    }
  }

  resolveConflict(overwrite) {
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
