import { reactive } from "vue";

class DownloadManager {
  constructor() {
    this.queue = reactive([]);
    this.nextId = 0;
  }

  add(file, shareHash = "") {
    if (!this.queue) {
      this.queue = reactive([]);
    }
    const download = {
      id: this.nextId++,
      name: file.name || (file.path ? file.path.split('/').pop() : 'download'),
      size: file.size || 0,
      progress: 0,
      status: "pending", // pending, downloading, completed, error, cancelled
      file: file,
      shareHash: shareHash,
      chunks: [],
      loaded: 0,
      abortController: null,
    };
    this.queue.push(download);
    return download.id;
  }

  findById(id) {
    if (!this.queue) return null;
    return this.queue.find((item) => item.id === id);
  }

  updateProgress(id, loaded, total) {
    const download = this.findById(id);
    if (download) {
      download.loaded = loaded;
      download.progress = total > 0 ? (loaded / total) * 100 : 0;
    }
  }

  setStatus(id, status) {
    const download = this.findById(id);
    if (download) {
      download.status = status;
    }
  }

  setError(id, errorMessage) {
    const download = this.findById(id);
    if (download) {
      download.status = "error";
      download.errorDetails = errorMessage;
    }
  }

  cancel(id) {
    const download = this.findById(id);
    if (download) {
      if (download.abortController) {
        download.abortController.abort();
      }
      download.status = "cancelled";
      this.remove(id);
    }
  }

  remove(id) {
    if (!this.queue) return;
    const index = this.queue.findIndex((item) => item.id === id);
    if (index !== -1) {
      this.queue.splice(index, 1);
    }
  }

  clearCompleted() {
    if (!this.queue) return;
    for (let i = this.queue.length - 1; i >= 0; i--) {
      if (this.queue[i].status === "completed" || this.queue[i].status === "cancelled") {
        this.queue.splice(i, 1);
      }
    }
  }

  hasActive() {
    if (!this.queue) return false;
    return this.queue.some((item) => item.status === "downloading" || item.status === "pending");
  }
}

export const downloadManager = new DownloadManager();
