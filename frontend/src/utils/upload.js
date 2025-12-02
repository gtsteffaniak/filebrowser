import { reactive } from "vue";
import { filesApi, publicApi } from "@/api";
import { state,mutations } from "@/store";
import { getters } from "@/store/getters";

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
    this.conflictingFolder = null; // Track the folder name that caused conflict
    this.pendingItems = null; // Store pending items during conflict resolution
    this.probedDirs = new Set(); // Track directories that were probed/created during conflict check
  }

  setOnConflict(handler) {
    this.onConflict = handler;
  }

  async add(basePath, items, overwrite = false) {
    // Handle undefined/null basePath
    if (!basePath) {
      basePath = "/";
    }
    if (basePath.slice(-1) !== "/") {
      basePath += "/";
    }

    // Pre-upload conflict check for top-level directories
    // Skip probing if overwrite is already true or overwriteAll is set
    if (this.overwriteAll === null && !overwrite) {
      const topLevelDirs = new Set();
      for (const item of items) {
        if (item.relativePath && item.relativePath.includes('/')) {
          topLevelDirs.add(item.relativePath.split('/')[0]);
        }
      }

      if (topLevelDirs.size > 0) {
        // First try using state.req.items if available (regular uploads)
        const existingItems = new Set(state.req?.items?.map(i => i.name) || []);
        let conflictingDirs = [...topLevelDirs].filter(dir => existingItems.has(dir));
        let probedDirs = new Set();

        // If state.req.items is not available (upload shares), probe the server
        if (existingItems.size === 0 && topLevelDirs.size > 0) {
          // Probe each top-level directory to check for conflicts
          const probeResults = await Promise.all(
            [...topLevelDirs].map(async (dirName) => {
              try {
                const testPath = `${basePath}${dirName}/`;
                if (getters.isShare()) {
                  await publicApi.post(state.shareInfo?.hash, testPath, new Blob([]), false, undefined, {}, true);
                } else {
                  await filesApi.post(state.req?.source, testPath, new Blob([]), false, undefined, {}, true);
                }
                // No conflict - directory was created successfully
                // Mark it so we can skip it later in the queue
                return { dirName, conflict: false, probed: true };
              } catch (err) {
                // 409 means conflict
                if (err?.response?.status === 409) {
                  return { dirName, conflict: true, probed: false };
                }
                // Other errors are actual failures, treat as no conflict for now
                return { dirName, conflict: false, probed: false };
              }
            })
          );
          conflictingDirs = probeResults
            .filter(result => result.conflict)
            .map(result => result.dirName);
          // Track which directories we successfully probed (created)
          probedDirs = new Set(
            probeResults
              .filter(result => result.probed)
              .map(result => result.dirName)
          );
        }

        if (conflictingDirs.length > 0) {
          // Store the conflicting folder name (take the first one for now)
          this.conflictingFolder = conflictingDirs[0];
          this.pendingItems = items;

          this.onConflict(resolution => {
            if (resolution === true) {
              // User chose overwrite - set the flag and add with overwrite=true
              this.overwriteAll = true;
              this.add(basePath, items, true);
            } else if (resolution && resolution.rename) {
              // User chose rename - continue with renamed items
              this.conflictingFolder = null;
              this.add(basePath, this.pendingItems, false);
            } else {
              // User cancelled
              this.overwriteAll = null;
              this.conflictingFolder = null;
              this.pendingItems = null;
            }
          });
          return;
        }
        // Store probed directories so we can skip them in queue processing
        this.probedDirs = probedDirs;
      }
    }

    const effectiveOverwrite = this.overwriteAll || overwrite;
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

        // Skip top-level directories that were already created during probing
        if (pathParts.length === 1 && this.probedDirs.has(dirName)) {
          continue;
        }

        const upload = {
          id: this.nextId++,
          name: dirName,
          size: 0,
          progress: 0,
          status: "pending",
          type: "directory",
          isToplevelDir: pathParts.length === 1,
          path: `${basePath}${dir}`,
          source: state.req?.source,
          overwrite: effectiveOverwrite,
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
        source: state.req?.source,
        overwrite: effectiveOverwrite,
      };
      return upload;
    });

    this.queue.push(...newUploads, ...fileUploads);

    // Clean up pending items after successful add
    this.pendingItems = null;
    this.conflictingFolder = null;
    this.probedDirs.clear();

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

    const maxConcurrent = state.user.fileLoading?.maxConcurrentUpload || 3;
    while (
      this.activeUploads < maxConcurrent &&
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

    // Update isUploading state based on whether there are active or pending uploads
    const hasActiveOrPending = this.activeUploads > 0 || this.hasPending();
    mutations.setIsUploading(hasActiveOrPending);

    // Only reload when we transition from having active uploads to having none
    const hasNoActiveOrPending = this.activeUploads === 0 && !this.hasPending();
    if (this.hadActiveUploads && hasNoActiveOrPending) {
      console.log("all uploads processed  ", this.queue);
      // Only reload if there are no errors or conflicts - keep prompt open so users can see and retry
      const hasErrorsOrConflicts = this.queue.some((item) =>
        item.status === "error" || item.status === "conflict"
      );
      if (!hasErrorsOrConflicts) {
        mutations.setReload(true);
      }
      this.hadActiveUploads = false; // Reset the flag
      this.overwriteAll = null; // Reset for next batch of uploads
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
      if (getters.isShare()) {
        await publicApi.post(state.shareInfo?.hash, upload.path, new Blob([]), upload.overwrite, undefined, {}, true);
      } else {
        await filesApi.post(upload.source, upload.path, new Blob([]), upload.overwrite, undefined, {}, true);
      }

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

    // Get chunk size in MB, default to 5 if not set or if 0
    let chunkSizeMb = state.user.fileLoading?.uploadChunkSizeMb ?? 5;
    if (chunkSizeMb === 0) {
      chunkSizeMb = 5;
    }
    const chunkSize = chunkSizeMb * 1024 * 1024;

    // Use non-chunked upload if file size is less than chunk size
    if (upload.size < chunkSize) {
      const progress = (percent) => {
        upload.progress = percent;
      };

      try {
        let promise;
        if (getters.isShare()) {
          promise = publicApi.post(state.shareInfo?.hash, upload.path, upload.file, upload.overwrite, progress, {
            "X-File-Total-Size": upload.size,
          });
        } else {
          promise = filesApi.post(upload.source, upload.path, upload.file, upload.overwrite, progress, {
            "X-File-Total-Size": upload.size,
          });
        }

        upload.xhr = promise.xhr;
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
        let promise;
        if (getters.isShare()) {
          promise = publicApi.post(
            state.shareInfo?.hash,
            upload.path,
            chunk,
            upload.overwrite,
            chunkProgress,
            {
              "X-File-Chunk-Offset": upload.chunkOffset,
              "X-File-Total-Size": upload.size,
            }
          );
        } else {
          promise = filesApi.post(
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
        }

        upload.xhr = promise.xhr;
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
    let hadCompleted = false;
    for (let i = this.queue.length - 1; i >= 0; i--) {
      const status = this.queue[i].status;
      if (status === "completed") {
        this.queue.splice(i, 1);
        hadCompleted = true;
      }
      if (state.user.fileLoading?.clearAll) {
        if (status === "error" || status === "conflict" || status === "paused") {
          this.queue.splice(i, 1);
        }
      }
    }
    // If we had completed uploads and the queue is now empty, trigger reload
    if (hadCompleted && this.queue.length === 0) {
      mutations.setReload(true);
    }
  }

  findById(id) {
    return this.queue.find((item) => item.id === id);
  }

  hasPending() {
    return this.queue.some((item) => item.status === "pending");
  }

  getConflictingFolder() {
    return this.conflictingFolder;
  }

  async renameFolder(oldName, newName) {
    if (!oldName || !newName || !this.pendingItems) {
      throw new Error("Invalid parameters for folder rename");
    }

    // Update all items in the pending items that reference the old folder name
    const updatedItems = this.pendingItems.map(item => {
      if (item.relativePath) {
        const pathParts = item.relativePath.split("/");
        if (pathParts[0] === oldName) {
          // Replace the first part of the path with the new name
          pathParts[0] = newName;
          return {
            ...item,
            relativePath: pathParts.join("/")
          };
        }
      }
      return item;
    });

    // Store the updated items for processing
    this.pendingItems = updatedItems;
    return true;
  }

  async handleUploadError(upload, err) {
    // Check if the error is a 409 Conflict
    if (err?.response?.status === 409) {
      upload.status = "conflict";
    } else if (err.message !== "Upload aborted") {
      upload.status = "error";
      // Store detailed error information for tooltip display
      upload.errorDetails = this.formatErrorMessage(err);
    } else {
      console.log(`upload.js: Upload aborted for id ${upload.id}`, upload);
    }
  }

  formatErrorMessage(err) {
    if (err?.response) {
      // API error with response
      const status = err.response.status;
      const statusText = err.response.statusText;
      const message = err.response.data?.message || err.response.data?.error || statusText;
      return `${status} ${statusText}: ${message}`;
    } else if (err?.message) {
      // Network error or other error with message
      return err.message;
    } else {
      // Fallback for unknown errors
      return "Unknown error occurred";
    }
  }
}

export const uploadManager = new UploadManager();

// Check for conflicts between items to be uploaded/moved and existing items
export function checkConflict(newItems, existingItems) {
  if (!newItems || !existingItems) {
    return false;
  }

  const existingNames = new Set(existingItems.map(item => item.name));

  return newItems.some(item => {
    const itemName = item.name || item.file?.name;
    return itemName && existingNames.has(itemName);
  });
}
