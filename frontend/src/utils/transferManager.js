import { reactive } from "vue";
import { cancelTransfer, listTransfers } from "@/api/transfers";
import { showSuccess } from "@/notify/message.js";
import i18n from "@/i18n";
import { mutations } from "@/store";
import { goToItemNotificationButton } from "@/utils/notificationActions";

class TransferManager {
  constructor() {
    this.queue = reactive([]);
    this._pollTimer = null;
  }

  addJob(jobId, action, items, { destPath, destSource } = {}) {
    const transfer = {
      id: jobId,
      action,
      items,
      destPath,
      destSource,
      status: "pending",
      totalBytes: 0,
      copiedBytes: 0,
      currentFile: "",
      itemsTotal: items.length,
      itemsCompleted: 0,
      progress: 0,
      error: "",
      startTime: Date.now(),
      speed: 0,
      _lastSpeedTime: Date.now(),
    };
    this.queue.push(transfer);
    this._startPolling();
    return transfer;
  }

  updateFromEvent(event) {
    const transfer = this.queue.find((t) => t.id === event.jobId);
    if (!transfer) return;
    this._applyUpdate(transfer, event);
  }

  _applyUpdate(transfer, data) {
    const now = Date.now();
    const prevBytes = transfer.copiedBytes || 0;
    const prevTime = transfer._lastSpeedTime || transfer.startTime || now;
    const timeDelta = (now - prevTime) / 1000;
    const bytesDelta = (data.copiedBytes || 0) - prevBytes;

    if (timeDelta > 0.5 && bytesDelta > 0 && data.status === "running") {
      const instantSpeed = bytesDelta / timeDelta;
      transfer.speed =
        transfer.speed > 0
          ? transfer.speed * 0.7 + instantSpeed * 0.3
          : instantSpeed;
      transfer._lastSpeedTime = now;
    } else if (data.status !== "running") {
      transfer.speed = 0;
      transfer._lastSpeedTime = now;
    }

    const prevStatus = transfer.status;
    transfer.status = data.status;
    transfer.totalBytes = data.totalBytes;
    transfer.copiedBytes = data.copiedBytes;
    transfer.currentFile = data.currentFile;
    transfer.itemsTotal = data.itemsTotal;
    transfer.itemsCompleted = data.itemsCompleted;
    transfer.error = data.error || "";
    transfer.progress =
      data.totalBytes > 0 ? (data.copiedBytes / data.totalBytes) * 100 : 0;

    if (prevStatus !== "completed" && data.status === "completed") {
      this._onTransferComplete(transfer);
    }
  }

  _onTransferComplete(transfer) {
    const t = i18n.global.t;
    const message =
      transfer.action === "move"
        ? t("prompts.moveSuccess")
        : t("prompts.copySuccess");

    const options = {};
    if (transfer.destPath) {
      options.icon = "folder";
      options.buttons = [
        goToItemNotificationButton(
          t("buttons.goToItem"),
          transfer.destSource || null,
          transfer.destPath,
          false
        ),
      ];
    }

    showSuccess(message, options);
    mutations.setReload(true);
  }

  _startPolling() {
    if (this._pollTimer) return;
    this._pollTimer = setInterval(() => void this._poll(), 1000);
  }

  _stopPolling() {
    if (this._pollTimer) {
      clearInterval(this._pollTimer);
      this._pollTimer = null;
    }
  }

  async _poll() {
    if (!this.hasActive()) {
      this._stopPolling();
      return;
    }
    try {
      const jobs = await listTransfers();
      if (!Array.isArray(jobs)) return;
      for (const job of jobs) {
        const transfer = this.queue.find((t) => t.id === job.id);
        if (transfer) {
          this._applyUpdate(transfer, job);
        }
      }
    } catch (_err) {
      // Polling failure is non-fatal
    }
  }

  findById(id) {
    return this.queue.find((item) => item.id === id);
  }

  async cancel(id) {
    const transfer = this.findById(id);
    if (transfer) {
      try {
        await cancelTransfer(id);
        transfer.status = "cancelled";
      } catch (_err) {
        // SSE/poll will update status
      }
    }
  }

  remove(id) {
    const index = this.queue.findIndex((item) => item.id === id);
    if (index !== -1) {
      this.queue.splice(index, 1);
    }
    if (!this.hasActive()) {
      this._stopPolling();
    }
  }

  clearCompleted() {
    for (let i = this.queue.length - 1; i >= 0; i--) {
      const transfer = this.queue.at(i);
      if (
        transfer.status === "completed" ||
        transfer.status === "cancelled" ||
        transfer.status === "failed"
      ) {
        this.queue.splice(i, 1);
      }
    }
    if (!this.hasActive()) {
      this._stopPolling();
    }
  }

  hasActive() {
    return this.queue.some(
      (item) =>
        item.status === "pending" ||
        item.status === "calculating" ||
        item.status === "running"
    );
  }
}

export const transferManager = new TransferManager();
