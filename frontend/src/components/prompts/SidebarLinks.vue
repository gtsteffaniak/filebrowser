<template>
  <SidebarLinksEditor
    ref="editor"
    :mode="context"
    :share-data="shareData"
    :show-prompt-actions="true"
    @save="saveLinks"
  />
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { shareApi } from "@/api";
import { eventBus } from "@/store/eventBus";
import SidebarLinksEditor from "@/components/sidebar/SidebarLinksEditor.vue";

export default {
  name: "SidebarLinks",
  components: {
    SidebarLinksEditor,
  },
  props: {
    context: {
      type: String,
      default: "user",
    },
    shareData: {
      type: Object,
      default: null,
    },
  },
  mounted() {
    if (this.context === "user") {
      void mutations.syncSidebarLinkDefaultsPolicy();
    }
  },
  methods: {
    async saveLinks({ links, showToolsInSidebar }) {
      try {
        if (this.context === "share") {
          const payload = {
            hash: this.shareData.hash,
            sidebarLinks: links,
          };

          await shareApi.create(payload);

          eventBus.emit("shareSidebarLinksUpdated", {
            hash: this.shareData.hash,
            sidebarLinks: links,
          });
        } else {
          await mutations.updateCurrentUser({
            sidebarLinks: [...links],
            showToolsInSidebar,
          });
        }

        mutations.closeTopPrompt();
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>

<style scoped>

.settings-items {
  margin-top: 0.5em;
  margin-bottom: 0.5em;
}

.padding-top {
  margin-top: 0.5em;
}

.links-list h3,
.add-link-form h3 {
  margin-bottom: 0.5em;
  font-size: 1em;
  font-weight: 600;
}

.empty-state {
  padding: 2em 1em;
  text-align: center;
  color: var(--textSecondary);
  font-style: italic;
}

.add-link-form h3 {
  margin-top: 0;
  margin-bottom: 0.75em;
  font-size: 0.95em;
}

.links-container {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  padding-bottom: 0.5em;
}

/* Link item styles */
.link-item {
  display: flex;
  align-items: center;
  gap: 0.5em;
  background: var(--surfaceSecondary);
  transition: all 0.2s ease;
}

.link-item.dragging {
  opacity: 0.5;
  border-color: var(--primaryColor);
  background: var(--surfaceTertiary);
}

.link-drag-handle {
  color: var(--textSecondary);
  cursor: grab;
}

.link-drag-handle:active {
  cursor: grabbing;
}

.link-icon {
  color: var(--primaryColor);
}

.link-details {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
  flex-grow: 1;
  width: 100%;
}

.link-name {
  font-weight: 500;
}

.link-category {
  font-size: 0.85em;
  color: var(--textSecondary);
}

.add-link-button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5em;
}

.add-link-form {
  padding: 0;
  margin-top: 0;
}

.form-group p {
  margin: 0.5em
}

.form-group p:first-of-type,
.add-link-form>p:first-of-type {
  margin-top: 0;
}

/* Icon preview styles */
.icon-input-group {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.icon-input {
  flex: 1;
}

.icon-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3em;
  height: 3em;
  border: 1px solid var(--borderColor);
  border-radius: 1em;
  background: var(--surfaceSecondary);
  color: var(--primaryColor);
}

.icon-preview .material-symbols,
.icon-preview .material-symbols-outlined {
  font-size: 2em;
}

.icon-preview-placeholder {
  color: var(--textSecondary);
  opacity: 0.6;
}

</style>
