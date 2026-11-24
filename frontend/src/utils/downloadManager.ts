import { reactive } from "vue";

export type DownloadStatus = "pending" | "downloading" | "completed" | "error" | "cancelled";

export interface DownloadFile {
  name?: string;
  path?: string;
  size?: number;
  source?: string;
  isDir?: boolean;
}

export interface DownloadItem {
  id: number;
  name: string;
  size: number;
  /** Percentage, 0-100 */
  progress: number;
  /** Bytes received so far */
  loaded: number;
  status: DownloadStatus;
  file: DownloadFile;
  shareHash: string;
  chunks: unknown[];
  abortController: AbortController | null;
  archiveFormat?: string | null;
  archiveFiles?: DownloadFile[];
  errorDetails?: string;
}

class DownloadManager {
  queue: DownloadItem[] = reactive<DownloadItem[]>([]);
  nextId = 0;

  add(file: DownloadFile, shareHash = ""): number {
    const download: DownloadItem = {
      id: this.nextId++,
      name: file.name || (file.path ? (file.path.split("/").pop() as string) : "download"),
      size: file.size || 0,
      progress: 0,
      status: "pending",
      file,
      shareHash,
      chunks: [],
      loaded: 0,
      abortController: null,
    };
    this.queue.push(download);
    return download.id;
  }

  findById(id: number): DownloadItem | undefined {
    return this.queue.find((item) => item.id === id);
  }

  updateProgress(id: number, loaded: number, total: number): void {
    const download = this.findById(id);
    if (download) {
      download.loaded = loaded;
      download.progress = total > 0 ? (loaded / total) * 100 : 0;
    }
  }

  setStatus(id: number, status: DownloadStatus): void {
    const download = this.findById(id);
    if (download) {
      download.status = status;
    }
  }

  setError(id: number, errorMessage: string): void {
    const download = this.findById(id);
    if (download) {
      download.status = "error";
      download.errorDetails = errorMessage;
    }
  }

  cancel(id: number): void {
    const download = this.findById(id);
    if (download) {
      if (download.abortController) {
        download.abortController.abort();
      }
      download.status = "cancelled";
      this.remove(id);
    }
  }

  remove(id: number): void {
    const index = this.queue.findIndex((item) => item.id === id);
    if (index !== -1) {
      this.queue.splice(index, 1);
    }
  }

  clearCompleted(): void {
    for (let i = this.queue.length - 1; i >= 0; i--) {
      const download = this.queue.at(i);
      if (download && (download.status === "completed" || download.status === "cancelled")) {
        this.queue.splice(i, 1);
      }
    }
  }

  /** Abort in-flight downloads, clear the queue, reset ids (fresh Download prompt instance). */
  reset(): void {
    for (const item of this.queue) {
      if (item.abortController) {
        try {
          item.abortController.abort();
        } catch (_) {
          // ignore
        }
      }
    }
    this.queue.length = 0;
    this.nextId = 0;
  }

  hasActive(): boolean {
    return this.queue.some((item) => item.status === "downloading" || item.status === "pending");
  }
}

export const downloadManager = new DownloadManager();
