<template>
  <Teleport to="body">
    <div
      v-if="hasActiveTransfers"
      class="transfer-indicator"
      :class="{ 'dark-mode': isDarkMode }"
      @click="openTransferPrompt"
      :title="statusText"
    >
      <svg class="progress-ring" viewBox="0 0 44 44">
        <circle
          class="progress-ring-bg"
          cx="22"
          cy="22"
          r="18"
          fill="none"
          stroke-width="3"
        />
        <circle
          class="progress-ring-fill"
          cx="22"
          cy="22"
          r="18"
          fill="none"
          stroke-width="3"
          :stroke-dasharray="circumference"
          :stroke-dashoffset="progressOffset"
        />
      </svg>
      <i class="material-symbols indicator-icon">swap_horiz</i>
      <span v-if="activeCount > 1" class="badge">{{ activeCount }}</span>
    </div>
  </Teleport>
</template>

<script>
import { transferManager } from "@/utils/transferManager";
import { state, getters, mutations } from "@/store";

export default {
  name: "TransferIndicator",
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    activeTransfers() {
      return transferManager.queue.filter(
        (t) =>
          t.status === "pending" ||
          t.status === "calculating" ||
          t.status === "running"
      );
    },
    hasActiveTransfers() {
      return this.activeTransfers.length > 0;
    },
    activeCount() {
      return this.activeTransfers.length;
    },
    overallProgress() {
      const active = this.activeTransfers;
      if (active.length === 0) return 0;
      let totalBytes = 0;
      let copiedBytes = 0;
      for (const t of active) {
        totalBytes += t.totalBytes || 0;
        copiedBytes += t.copiedBytes || 0;
      }
      if (totalBytes === 0) return 0;
      return (copiedBytes / totalBytes) * 100;
    },
    circumference() {
      return 2 * Math.PI * 18;
    },
    progressOffset() {
      return this.circumference - (this.overallProgress / 100) * this.circumference;
    },
    statusText() {
      if (this.activeCount === 1) {
        return `${Math.round(this.overallProgress)}%`;
      }
      return `${this.activeCount} transfers - ${Math.round(this.overallProgress)}%`;
    },
  },
  methods: {
    openTransferPrompt() {
      const hasPrompt = state.prompts.some((p) => p.name === "transfer");
      if (!hasPrompt) {
        mutations.showPrompt({ name: "transfer", pinned: true });
      }
    },
  },
};
</script>

<style scoped>
.transfer-indicator {
  position: fixed;
  bottom: 4rem;
  right: 1.5rem;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--primaryColor, #2196f3);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 9999;
  box-shadow: 0 3px 8px rgba(0, 0, 0, 0.3);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.transfer-indicator:hover {
  transform: scale(1.1);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.transfer-indicator:active {
  transform: scale(0.95);
}

.progress-ring {
  position: absolute;
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.progress-ring-bg {
  stroke: rgba(255, 255, 255, 0.25);
}

.progress-ring-fill {
  stroke: white;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.3s ease;
}

.indicator-icon {
  font-size: 22px;
  z-index: 1;
}

.badge {
  position: absolute;
  top: -4px;
  right: -4px;
  background: #f44336;
  color: white;
  font-size: 11px;
  font-weight: 700;
  min-width: 18px;
  height: 18px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  z-index: 2;
}

.dark-mode.transfer-indicator {
  box-shadow: 0 3px 8px rgba(0, 0, 0, 0.5);
}
</style>
