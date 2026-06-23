<template>
  <div class="share-picker-field">
    <div
      role="button"
      tabindex="0"
      :aria-label="resolvedAriaLabel"
      class="searchContext clickable button unified-share-picker"
      @click="openPicker"
      @keydown.enter.prevent="openPicker"
      @keydown.space.prevent="openPicker"
    >
      {{ buttonLabel }}
    </div>
    <button
      v-if="shareHash"
      type="button"
      class="button button--flat share-clear"
      :aria-label="allSharesLabel"
      :title="allSharesLabel"
      @click="clearSelection"
    >
      <i class="material-symbols">close</i>
    </button>
  </div>
</template>

<script>
import { shareApi } from "@/api";
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";

function normalizeContextId() {
  return `share-picker-btn-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

export default {
  name: "SharePickerButton",

  props: {
    shareHash: {
      type: String,
      default: "",
    },
    ariaLabel: {
      type: String,
      default: undefined,
    },
    placeholder: {
      type: String,
      default: undefined,
    },
  },

  emits: ["update:shareHash", "select"],

  data() {
    return {
      pendingContextId: null,
      selectedLabel: "",
    };
  },

  computed: {
    allSharesLabel() {
      return this.$t("general.allShares");
    },
    resolvedAriaLabel() {
      return this.ariaLabel || this.buttonLabel;
    },
    buttonLabel() {
      if (this.shareHash) {
        if (this.selectedLabel) {
          return this.selectedLabel;
        }
        return this.shareHash;
      }
      const placeholder = this.placeholder;
      if (placeholder !== undefined && placeholder !== null && placeholder !== "") {
        return placeholder;
      }
      return this.$t("sidebar.chooseShare");
    },
  },

  watch: {
    shareHash: {
      immediate: true,
      handler(hash) {
        if (!hash) {
          this.selectedLabel = "";
          return;
        }
        void this.syncLabelFromHash(hash);
      },
    },
  },

  mounted() {
    eventBus.on("shareSelected", this.onShareSelected);
    eventBus.on("sharePickerCancelled", this.onSharePickerCancelled);
  },

  beforeUnmount() {
    eventBus.off("shareSelected", this.onShareSelected);
    eventBus.off("sharePickerCancelled", this.onSharePickerCancelled);
  },

  methods: {
    formatShareLabel(share) {
      if (!share) {
        return "";
      }
      const path = share.path || "";
      const title = share.title || "";
      if (title) {
        return `${path} (${title})`;
      }
      return path || share.hash || "";
    },

    async syncLabelFromHash(hash) {
      if (!hash) {
        this.selectedLabel = "";
        return;
      }
      const requestedHash = hash;
      try {
        const shares = await shareApi.list();
        if (requestedHash !== this.shareHash) {
          return;
        }
        const share = shares.find((item) => item.hash === requestedHash);
        this.selectedLabel = this.formatShareLabel(share) || requestedHash;
      } catch {
        if (requestedHash !== this.shareHash) {
          return;
        }
        this.selectedLabel = requestedHash;
      }
    },

    openPicker() {
      this.pendingContextId = normalizeContextId();
      mutations.showPrompt({
        name: "sharePicker",
        props: {
          currentHash: this.shareHash,
          selectionContextId: this.pendingContextId,
          title: this.$t("sidebar.chooseShare"),
        },
      });
    },

    onShareSelected(data) {
      if (!this.pendingContextId || !data) {
        return;
      }
      if (data.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
      if (typeof data.hash === "string") {
        this.$emit("update:shareHash", data.hash);
        this.selectedLabel = this.formatShareLabel({
          hash: data.hash,
          path: data.path,
          title: data.title,
        }) || data.hash;
      }
      this.$emit("select");
    },

    onSharePickerCancelled(data) {
      if (!this.pendingContextId || !data) {
        return;
      }
      if (data.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
    },

    clearSelection() {
      this.selectedLabel = "";
      this.$emit("update:shareHash", "");
      this.$emit("select");
    },
  },
};
</script>

<style scoped>
.share-picker-field {
  display: flex;
  align-items: stretch;
  gap: 0.35rem;
  max-width: 100%;
}

.unified-share-picker {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 0;
  word-break: break-word;
  box-sizing: border-box;
}

.share-clear {
  flex-shrink: 0;
  padding: 0.35rem;
  min-width: unset;
}

.share-clear .material-symbols {
  font-size: 1.1rem;
}
</style>
